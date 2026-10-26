package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	agentcrypto "github.com/open-agentpay/agentpay-facilitator-go/internal/crypto"
	agentevm "github.com/open-agentpay/agentpay-facilitator-go/internal/evm"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

var (
	ErrNonceAlreadyUsed   = errors.New("NONCE_ALREADY_USED")
	ErrChallengeExpired   = errors.New("CHALLENGE_EXPIRED")
	ErrInvalidRequest     = errors.New("INVALID_REQUEST")
	ErrInvalidSignature   = errors.New("INVALID_SIGNATURE")
	ErrSettlementNotFound = errors.New("SETTLEMENT_NOT_FOUND")
)

type VerificationRecord struct {
	Challenge     models.PaymentChallenge
	Authorization models.PaymentAuthorization
	NonceKey      string
	VerifiedAt    int64
	ExpiresAt     int64
}

// FacilitatorService coordinates off-chain verification and EVM settlement.
type FacilitatorService struct {
	mu                 sync.RWMutex
	clock              func() int64
	evmClient          agentevm.Client
	facilitatorKey     *ecdsa.PrivateKey
	facilitatorAddress common.Address
	defaultChainID     *big.Int
	defaultTokenAddr   common.Address
	autoPollReceipts   bool

	nonceStore         *MemoryNonceStore
	verificationTokens map[string]*VerificationRecord
	settlements        map[string]*models.SettlementStatus
	idempotencyCache   map[string]*models.SettleResponse
}

// Options configure the FacilitatorService.
type Options struct {
	Clock            func() int64
	EVMClient        agentevm.Client
	PrivateKeyHex    string
	DefaultChainID   *big.Int
	DefaultTokenAddr string
	AutoPollReceipts bool
}

// NewFacilitatorService creates a configured FacilitatorService.
func NewFacilitatorService(opts Options) (*FacilitatorService, error) {
	clock := opts.Clock
	if clock == nil {
		clock = func() int64 { return time.Now().Unix() }
	}

	chainID := opts.DefaultChainID
	if chainID == nil {
		chainID = big.NewInt(421614) // Arbitrum Sepolia default
	}

	var privKey *ecdsa.PrivateKey
	var facilitatorAddr common.Address

	if opts.PrivateKeyHex != "" {
		clean := strings.TrimPrefix(strings.TrimSpace(opts.PrivateKeyHex), "0x")
		k, err := crypto.HexToECDSA(clean)
		if err != nil {
			return nil, fmt.Errorf("invalid facilitator private key: %w", err)
		}
		privKey = k
		facilitatorAddr = crypto.PubkeyToAddress(k.PublicKey)
	} else {
		// Ephemeral key for development/mocking
		k, err := crypto.GenerateKey()
		if err == nil {
			privKey = k
			facilitatorAddr = crypto.PubkeyToAddress(k.PublicKey)
		}
	}

	var tokenAddr common.Address
	if opts.DefaultTokenAddr != "" {
		tokenAddr = common.HexToAddress(opts.DefaultTokenAddr)
	}

	return &FacilitatorService{
		clock:              clock,
		evmClient:          opts.EVMClient,
		facilitatorKey:     privKey,
		facilitatorAddress: facilitatorAddr,
		defaultChainID:     chainID,
		defaultTokenAddr:   tokenAddr,
		autoPollReceipts:   opts.AutoPollReceipts,
		nonceStore:         NewMemoryNonceStore(),
		verificationTokens: make(map[string]*VerificationRecord),
		settlements:        make(map[string]*models.SettlementStatus),
		idempotencyCache:   make(map[string]*models.SettleResponse),
	}, nil
}

func (s *FacilitatorService) FacilitatorAddress() common.Address {
	return s.facilitatorAddress
}

func (s *FacilitatorService) Now() int64 {
	return s.clock()
}

// Verify implements POST /v1/verify.
func (s *FacilitatorService) Verify(ctx context.Context, req *models.VerifyRequest) (*models.VerifyResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty verify request body", ErrInvalidRequest)
	}

	ch := &req.Challenge
	auth := &req.Authorization
	now := s.Now()

	// 1. Time bounds checks
	if now > ch.ValidBefore {
		return nil, fmt.Errorf("%w: challenge expired (validBefore=%d, now=%d)", ErrChallengeExpired, ch.ValidBefore, now)
	}
	if now < ch.ValidAfter {
		return nil, fmt.Errorf("%w: challenge not yet valid (validAfter=%d, now=%d)", ErrInvalidRequest, ch.ValidAfter, now)
	}

	// 2. Correlation check
	if auth.ChallengeID != ch.ChallengeID {
		return nil, fmt.Errorf("%w: challenge ID mismatch: auth=%s, challenge=%s", ErrInvalidRequest, auth.ChallengeID, ch.ChallengeID)
	}

	// 3. In-Memory Nonce Replay Check (P0-7)
	if s.nonceStore.IsUsed(ch.Network, ch.Asset, auth.Payer, auth.Nonce) {
		return nil, fmt.Errorf("%w: nonce %s has already been used for payer %s", ErrNonceAlreadyUsed, auth.Nonce, auth.Payer)
	}

	// 4. On-Chain authorizationState check (if EVM RPC client configured)
	if s.evmClient != nil && ch.Asset != "" && auth.Payer != "" && auth.Nonce != "" {
		tokenContract := common.HexToAddress(ch.Asset)
		payerAddr := common.HexToAddress(auth.Payer)
		nonceBytes, err := agentcrypto.NormalizeBytes32(auth.Nonce)
		if err == nil {
			used, checkErr := s.evmClient.CheckAuthorizationState(ctx, tokenContract, payerAddr, nonceBytes)
			if checkErr == nil && used {
				_, _ = s.nonceStore.CheckAndRecord(ch.Network, ch.Asset, auth.Payer, auth.Nonce)
				return nil, fmt.Errorf("%w: nonce %s was already consumed on-chain", ErrNonceAlreadyUsed, auth.Nonce)
			}
		}
	}

	if s.evmClient != nil && auth.Payer != "" {
		code, err := s.evmClient.CodeAt(ctx, common.HexToAddress(auth.Payer))
		if err != nil {
			return nil, fmt.Errorf("%w: cannot check payer account code: %v", ErrInvalidRequest, err)
		}
		if len(code) > 0 {
			return nil, fmt.Errorf("%w: payer %s has contract code or EIP-7702 delegation; ECDSA authorization not accepted by USDC", ErrInvalidSignature, auth.Payer)
		}
	}

	// 5. Cryptographic EIP-712 / EIP-3009 Signature Verification
	isValid, recovered, err := agentcrypto.VerifyAuthorization(auth, ch, "", "2")
	if err != nil || !isValid {
		return nil, fmt.Errorf("%w: cryptographic signature verification failed: recovered=%s expected=%s err=%v", ErrInvalidSignature, recovered, auth.Payer, err)
	}

	// 6. Generate Verification Token
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	vtok := fmt.Sprintf("vtok_%s", hex.EncodeToString(randBytes))

	s.mu.Lock()
	s.verificationTokens[vtok] = &VerificationRecord{
		Challenge:     *ch,
		Authorization: *auth,
		NonceKey:      makeNonceKey(ch.Network, ch.Asset, auth.Payer, auth.Nonce),
		VerifiedAt:    now,
		ExpiresAt:     ch.ValidBefore,
	}
	s.mu.Unlock()

	return &models.VerifyResponse{
		Valid:             true,
		VerifiedPayer:     auth.Payer,
		Payer:             auth.Payer,
		VerifiedAt:        now,
		VerificationToken: vtok,
	}, nil
}

// Settle implements POST /v1/settle.
func (s *FacilitatorService) Settle(ctx context.Context, req *models.SettleRequest) (*models.SettleResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty settle request body", ErrInvalidRequest)
	}

	auth := &req.Authorization
	vtok := req.VerificationToken
	now := s.Now()

	// Idempotency cache check
	if req.IdempotencyKey != "" {
		idemKey := fmt.Sprintf("%s:%s", strings.ToLower(auth.Payer), req.IdempotencyKey)
		s.mu.RLock()
		if cached, ok := s.idempotencyCache[idemKey]; ok {
			s.mu.RUnlock()
			return cached, nil
		}
		s.mu.RUnlock()
	}

	s.mu.Lock()
	record, exists := s.verificationTokens[vtok]
	if !exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: unknown or expired verification token: %s", ErrInvalidRequest, vtok)
	}

	if now > record.ExpiresAt {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: verification token expired", ErrChallengeExpired)
	}

	// Atomically reserve nonce
	ch := record.Challenge
	recorded, _ := s.nonceStore.CheckAndRecord(ch.Network, ch.Asset, auth.Payer, auth.Nonce)
	if !recorded {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: nonce already consumed for settlement", ErrNonceAlreadyUsed)
	}
	s.mu.Unlock()

	// Build calldata for transferWithAuthorization
	amountVal, _ := new(big.Int).SetString(ch.Amount, 10)
	payeeAddr := common.HexToAddress(ch.Payee)
	calldata, err := agentevm.EncodeTransferFromAuthorization(auth, payeeAddr, amountVal)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to encode EIP-3009 calldata: %v", ErrInvalidRequest, err)
	}

	tokenContract := common.HexToAddress(ch.Asset)
	if tokenContract == (common.Address{}) {
		tokenContract = s.defaultTokenAddr
	}

	var txHashStr string

	if s.evmClient != nil && s.facilitatorKey != nil {
		// Live on-chain transaction submission
		txNonce, err := s.evmClient.GetNonce(ctx, s.facilitatorAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to get facilitator nonce: %w", err)
		}

		tipCap, err := s.evmClient.SuggestGasTipCap(ctx)
		if err != nil || tipCap == nil || tipCap.Sign() <= 0 {
			tipCap = big.NewInt(1_000_000) // 0.001 Gwei fallback
		}

		gasPrice, err := s.evmClient.SuggestGasPrice(ctx)
		if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
			gasPrice = big.NewInt(100_000_000) // 0.1 Gwei fallback
		}

		// Buffer fee 1.5x on base gas
		baseGasBuffered := new(big.Int).Div(new(big.Int).Mul(gasPrice, big.NewInt(3)), big.NewInt(2))
		feeCap := new(big.Int).Add(baseGasBuffered, tipCap)

		gasLimit, err := s.evmClient.EstimateGas(ctx, ethereum.CallMsg{
			From: s.facilitatorAddress,
			To:   &tokenContract,
			Data: calldata,
		})
		if err != nil || gasLimit == 0 {
			return nil, fmt.Errorf("%w: simulation failed, not broadcasting: %v", ErrInvalidSignature, err)
		}
		gasLimit = gasLimit * 125 / 100

		chainID := s.evmClient.ChainID()
		if chainID == nil {
			chainID = s.defaultChainID
		}

		rawTx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     txNonce,
			GasTipCap: tipCap,
			GasFeeCap: feeCap,
			Gas:       gasLimit,
			To:        &tokenContract,
			Value:     big.NewInt(0),
			Data:      calldata,
		})

		signer := types.LatestSignerForChainID(chainID)
		signedTx, err := types.SignTx(rawTx, signer, s.facilitatorKey)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sign settlement transaction: %v", ErrInvalidRequest, err)
		}

		txHash, err := s.evmClient.SendRawTransaction(ctx, signedTx)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to broadcast transaction to EVM RPC: %v", ErrInvalidRequest, err)
		}
		txHashStr = txHash.Hex()
	} else {
		// Mock hash for dry-run / unit tests
		randHash := make([]byte, 32)
		_, _ = rand.Read(randHash)
		txHashStr = "0x" + hex.EncodeToString(randHash)
	}

	randID := make([]byte, 12)
	_, _ = rand.Read(randID)
	settlementID := fmt.Sprintf("stl_%s", hex.EncodeToString(randID))

	s.mu.Lock()
	queuePos := len(s.settlements) + 1
	resp := &models.SettleResponse{
		SettlementID:  settlementID,
		Status:        models.SettlementStateSubmitted,
		QueuePosition: queuePos,
		TxHash:        txHashStr,
	}

	s.settlements[settlementID] = &models.SettlementStatus{
		SettlementID: settlementID,
		Status:       models.SettlementStateSubmitted,
		TxHash:       txHashStr,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if req.IdempotencyKey != "" {
		idemKey := fmt.Sprintf("%s:%s", strings.ToLower(auth.Payer), req.IdempotencyKey)
		s.idempotencyCache[idemKey] = resp
	}
	s.mu.Unlock()

	// Optional background receipt polling
	if s.autoPollReceipts && s.evmClient != nil && txHashStr != "" {
		go s.pollReceipt(settlementID, common.HexToHash(txHashStr))
	}

	return resp, nil
}

func (s *FacilitatorService) pollReceipt(settlementID string, txHash common.Hash) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	receipt, err := agentevm.WaitForReceipt(ctx, s.evmClient, txHash, 45*time.Second, 1*time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()

	status, exists := s.settlements[settlementID]
	if !exists {
		return
	}

	status.UpdatedAt = s.Now()
	if err != nil {
		slog.Warn("settlement receipt polling failed", slog.String("settlement_id", settlementID), slog.Any("error", err))
		status.Status = models.SettlementStateFailed
		status.Error = err.Error()
		return
	}

	if receipt.Status == types.ReceiptStatusSuccessful {
		status.Status = models.SettlementStateConfirmed
	} else {
		status.Status = models.SettlementStateFailed
		status.Error = "EVM transaction reverted on-chain"
	}
}

// GetSettlement implements GET /v1/settle/{id}.
func (s *FacilitatorService) GetSettlement(settlementID string) (*models.SettlementStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, exists := s.settlements[settlementID]
	if !exists {
		return nil, ErrSettlementNotFound
	}
	return res, nil
}

// IsNonceUsed checks whether a nonce is already spent.
func (s *FacilitatorService) IsNonceUsed(network, asset, payer, nonce string) bool {
	return s.nonceStore.IsUsed(network, asset, payer, nonce)
}
