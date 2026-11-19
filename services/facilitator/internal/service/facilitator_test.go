package service

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	agentcrypto "github.com/open-agentpay/agentpay-facilitator-go/internal/crypto"
	agentevm "github.com/open-agentpay/agentpay-facilitator-go/internal/evm"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

func createTestService(mockClient agentevm.Client) *FacilitatorService {
	currentTime := int64(1729000100)
	svc, _ := NewFacilitatorService(Options{
		Clock:            func() int64 { return currentTime },
		EVMClient:        mockClient,
		DefaultChainID:   big.NewInt(84532),
		DefaultTokenAddr: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AutoPollReceipts: false,
	})
	return svc
}

func getValidTestData() (*models.PaymentChallenge, *models.PaymentAuthorization) {
	ch := &models.PaymentChallenge{
		Version:     "2.0",
		ChallengeID: "ch_01j7xyz890abcdef12345678",
		Network:     "eip155:84532",
		Asset:       "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AssetSymbol: "USDC",
		Decimals:    6,
		Amount:      "100000",
		Payee:       "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		Scheme:      "EIP-3009",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
	}

	auth := &models.PaymentAuthorization{
		Version:     "2.0",
		ChallengeID: "ch_01j7xyz890abcdef12345678",
		Payer:       "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		Scheme:      "EIP-3009",
		Nonce:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
		Signature: models.SignatureData{
			V:          27,
			R:          "0x31207f584bc8e976a0ce72ea1c9c85cdb4da7686401c5b6efdc27040e5b0f22",
			S:          "0x2eacb1d50c27d4f5052b776e4bb5f1b42ce5c17fb7f553d44d0c42105444f4f1",
			Serialized: "031207f584bc8e976a0ce72ea1c9c85cdb4da7686401c5b6efdc27040e5b0f222eacb1d50c27d4f5052b776e4bb5f1b42ce5c17fb7f553d44d0c42105444f4f11b",
		},
	}
	return ch, auth
}

func TestVerifySuccessAndSettleLifecycle(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ctx := context.Background()

	ch, auth := getValidTestData()

	// 1. Verify
	verifyResp, err := svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err != nil {
		t.Fatalf("unexpected verify error: %v", err)
	}
	if !verifyResp.Valid {
		t.Fatalf("expected verifyResp.Valid to be true")
	}
	if verifyResp.VerificationToken == "" {
		t.Fatalf("expected non-empty verification token")
	}

	// 2. Settle
	settleResp, err := svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     *auth,
		IdempotencyKey:    "idem_12345",
	})
	if err != nil {
		t.Fatalf("unexpected settle error: %v", err)
	}
	if settleResp.SettlementID == "" || settleResp.TxHash == "" {
		t.Fatalf("expected settlementID and txHash populated, got %+v", settleResp)
	}

	// 3. Idempotent Retry Settle
	retryResp, err := svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     *auth,
		IdempotencyKey:    "idem_12345",
	})
	if err != nil {
		t.Fatalf("unexpected idempotent retry error: %v", err)
	}
	if retryResp.SettlementID != settleResp.SettlementID {
		t.Fatalf("expected same settlement ID on idempotent retry")
	}

	// 4. Query Settlement Status
	status, err := svc.GetSettlement(settleResp.SettlementID)
	if err != nil {
		t.Fatalf("failed to get settlement: %v", err)
	}
	if status.Status != models.SettlementStateSubmitted {
		t.Fatalf("expected SUBMITTED status, got %s", status.Status)
	}

	// 5. Verify Replay Rejection
	_, err = svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err == nil {
		t.Fatalf("expected verify replay to fail because nonce is consumed")
	}
}

func TestVerifyExpiredChallenge(t *testing.T) {
	svc := createTestService(nil)
	ctx := context.Background()
	ch, auth := getValidTestData()

	ch.ValidBefore = 1729000050 // in the past relative to clock (1729000100)
	_, err := svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err == nil {
		t.Fatalf("expected error for expired challenge")
	}
}

func TestVerifyOnChainUsedNonce(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ctx := context.Background()
	ch, auth := getValidTestData()

	var nonceB32 [32]byte
	copy(nonceB32[:], common.Hex2Bytes(auth.Nonce[2:]))
	mockClient.MarkAuthorizationUsed(
		common.HexToAddress(ch.Asset),
		common.HexToAddress(auth.Payer),
		nonceB32,
	)

	_, err := svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err == nil {
		t.Fatalf("expected verify to fail when nonce is used on-chain")
	}
}

func TestP05PayeeAllowlistSettleReverifyAndLowGasHalt(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ctx := context.Background()

	ch, auth := getValidTestData()

	// 1. Dynamic merchant payee: Facilitator accepts any payee specified by the route/merchant
	verifyResp, err := svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err != nil {
		t.Fatalf("unexpected verify error with merchant payee: %v", err)
	}

	// 2. Settle with tampered authorization (altered payer or corrupted signature) returns ErrInvalidSignature
	tamperedAuth := *auth
	tamperedAuth.Payer = "0x9999999999999999999999999999999999999999"
	_, err = svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     tamperedAuth,
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for tampered payer on Settle, got %v", err)
	}

	tamperedSigAuth := *auth
	tamperedSigAuth.Signature.R = "0x1111111111111111111111111111111111111111111111111111111111111111"
	tamperedSigAuth.Signature.Serialized = "11111111111111111111111111111111111111111111111111111111111111112eacb1d50c27d4f5052b776e4bb5f1b42ce5c17fb7f553d44d0c42105444f4f11b"
	_, err = svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     tamperedSigAuth,
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for tampered signature on Settle, got %v", err)
	}

	// 3. Low gas balance halts Verify and Settle
	svc.SetMinGasBalanceWei(big.NewInt(10_000_000_000_000_000)) // 0.01 ETH threshold
	mockClient.SetBalance(big.NewInt(100_000))                  // well below threshold

	_, err = svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     *auth,
	})
	if !errors.Is(err, ErrInsufficientGasBalance) {
		t.Fatalf("expected ErrInsufficientGasBalance when gas balance is low, got %v", err)
	}

	// Restore gas balance -> Settle succeeds
	mockClient.SetBalance(big.NewInt(1_000_000_000_000_000_000))
	settleResp, err := svc.Settle(ctx, &models.SettleRequest{
		VerificationToken: verifyResp.VerificationToken,
		Authorization:     *auth,
	})
	if err != nil || settleResp.SettlementID == "" {
		t.Fatalf("expected Settle to succeed after gas balance restored, got err=%v", err)
	}
}

func TestEstimateGasSurchargeDynamicFormula(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(421614))
	svc := createTestService(mockClient)
	svc.SetNativePriceUSD(2000.0) // 2000 USD/ETH

	res, err := svc.EstimateGasSurcharge(context.Background(), "eip155:421614", "0xUSDC", 10.0)
	if err != nil {
		t.Fatalf("unexpected error estimating gas surcharge: %v", err)
	}

	if res.GasLimit != 85000 {
		t.Errorf("expected gas limit 85000, got %d", res.GasLimit)
	}
	if res.SurchargeBaseUnits <= 0 {
		t.Errorf("expected positive surcharge base units, got %d", res.SurchargeBaseUnits)
	}
	if res.SlippageBufferPct != 10.0 {
		t.Errorf("expected slippage 10.0, got %f", res.SlippageBufferPct)
	}
}

func TestBatchSettle(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ch, auth := getValidTestData()
	ctx := context.Background()

	vResp, err := svc.Verify(ctx, &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	batchReq := &models.BatchSettleRequest{
		Settlements: []models.SettleRequest{
			{
				VerificationToken: vResp.VerificationToken,
				Authorization:     *auth,
			},
		},
	}

	bResp, err := svc.BatchSettle(ctx, batchReq)
	if err != nil {
		t.Fatalf("batch settle failed: %v", err)
	}
	if bResp.TotalCount != 1 || len(bResp.SettlementIDs) != 1 {
		t.Fatalf("expected 1 settled item in batch, got %d", bResp.TotalCount)
	}
}

// signTestAuthorization signs a challenge using the public Hardhat test account #0 private key.
func signTestAuthorization(t *testing.T, ch *models.PaymentChallenge, nonceHex string) *models.PaymentAuthorization {
	t.Helper()
	// Public Hardhat/Anvil account #0 key (test-only)
	privKey, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatalf("failed to load test key: %v", err)
	}
	payer := crypto.PubkeyToAddress(privKey.PublicKey)
	chainID, err := agentcrypto.ParseCAIP2ChainID(ch.Network)
	if err != nil {
		t.Fatalf("invalid chain id: %v", err)
	}
	amountVal, _ := new(big.Int).SetString(ch.Amount, 10)
	nonceBytes, err := agentcrypto.NormalizeBytes32(nonceHex)
	if err != nil {
		t.Fatalf("invalid nonce: %v", err)
	}
	domainSep := agentcrypto.ComputeDomainSeparator("USD Coin", "2", chainID, common.HexToAddress(ch.Asset))
	structHash := agentcrypto.ComputeTransferWithAuthorizationStructHash(
		payer,
		common.HexToAddress(ch.Payee),
		amountVal,
		ch.ValidAfter,
		ch.ValidBefore,
		nonceBytes,
	)
	digest := agentcrypto.ComputeEIP712Digest(domainSep, structHash)
	sig, err := crypto.Sign(digest[:], privKey)
	if err != nil {
		t.Fatalf("failed to sign digest: %v", err)
	}
	sig[64] += 27

	return &models.PaymentAuthorization{
		Version:     ch.Version,
		ChallengeID: ch.ChallengeID,
		Payer:       payer.Hex(),
		Scheme:      ch.Scheme,
		Nonce:       nonceHex,
		ValidAfter:  ch.ValidAfter,
		ValidBefore: ch.ValidBefore,
		Signature: models.SignatureData{
			V:          sig[64],
			R:          "0x" + hex.EncodeToString(sig[0:32]),
			S:          "0x" + hex.EncodeToString(sig[32:64]),
			Serialized: hex.EncodeToString(sig),
		},
	}
}

func TestVerifyInsufficientTokenBalance(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ch, auth := getValidTestData()

	// Payer only holds 50,000 base units (0.05 USDC), challenge requires 100,000 (0.10 USDC)
	mockClient.SetTokenBalance(common.HexToAddress(ch.Asset), common.HexToAddress(auth.Payer), big.NewInt(50_000))

	_, err := svc.Verify(context.Background(), &models.VerifyRequest{
		Challenge:     *ch,
		Authorization: *auth,
	})
	if !errors.Is(err, ErrInsufficientTokenBalance) {
		t.Fatalf("expected ErrInsufficientTokenBalance, got %v", err)
	}
	var balErr *InsufficientTokenBalanceError
	if !errors.As(err, &balErr) {
		t.Fatalf("expected *InsufficientTokenBalanceError, got %T", err)
	}
	if balErr.RequiredAmount != "100000" || balErr.AvailableBalance != "50000" || balErr.InFlightAmount != "0" {
		t.Fatalf("unexpected balance error details: %+v", balErr)
	}
}

func TestVerifyInFlightConcurrentBalanceGuardAndTTLExpiry(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	now := int64(1729000100)
	svc, err := NewFacilitatorService(Options{
		Clock:            func() int64 { return now },
		EVMClient:        mockClient,
		DefaultChainID:   big.NewInt(84532),
		DefaultTokenAddr: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AutoPollReceipts: false,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	svc.SetReservationTTL(60)

	ch1, auth1 := getValidTestData()
	// Wallet holds 150,000 (0.15 USDC): enough for one 100,000 payment, not two concurrently
	mockClient.SetTokenBalance(common.HexToAddress(ch1.Asset), common.HexToAddress(auth1.Payer), big.NewInt(150_000))

	if _, err := svc.Verify(context.Background(), &models.VerifyRequest{Challenge: *ch1, Authorization: *auth1}); err != nil {
		t.Fatalf("first verify should succeed, got %v", err)
	}

	ch2 := *ch1
	ch2.ChallengeID = "ch_second_concurrent"
	auth2 := signTestAuthorization(t, &ch2, "0x2222222222222222222222222222222222222222222222222222222222222222")

	// Second concurrent verify must be blocked by in-flight reservation
	_, err = svc.Verify(context.Background(), &models.VerifyRequest{Challenge: ch2, Authorization: *auth2})
	if !errors.Is(err, ErrInsufficientTokenBalance) {
		t.Fatalf("expected ErrInsufficientTokenBalance on second concurrent verify, got %v", err)
	}
	var balErr *InsufficientTokenBalanceError
	if !errors.As(err, &balErr) || balErr.AvailableBalance != "50000" || balErr.InFlightAmount != "100000" {
		t.Fatalf("unexpected in-flight balance error details: %+v", balErr)
	}

	// Advance clock beyond 60s reservation TTL -> expired reservation is lazily purged
	now += 61
	if _, err := svc.Verify(context.Background(), &models.VerifyRequest{Challenge: ch2, Authorization: *auth2}); err != nil {
		t.Fatalf("second verify should succeed after TTL expiry, got %v", err)
	}
}

func TestVerifyBalanceCheckFailClosedOnRPCError(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ch, auth := getValidTestData()

	mockClient.SetBalanceOfError(errors.New("rpc timeout"))
	_, err := svc.Verify(context.Background(), &models.VerifyRequest{Challenge: *ch, Authorization: *auth})
	if !errors.Is(err, ErrRPCUnavailable) {
		t.Fatalf("expected ErrRPCUnavailable (fail-closed) on BalanceOf error, got %v", err)
	}
}

func TestVerifySimulationRevertRejected(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc := createTestService(mockClient)
	ch, auth := getValidTestData()

	mockClient.SetCallContractError(errors.New("execution reverted: FiatTokenV2: account is blacklisted"))
	_, err := svc.Verify(context.Background(), &models.VerifyRequest{Challenge: *ch, Authorization: *auth})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature when simulation reverts, got %v", err)
	}
}

func TestPollReceiptReleasesInFlightReservation(t *testing.T) {
	mockClient := agentevm.NewMockClient(big.NewInt(84532))
	svc, err := NewFacilitatorService(Options{
		Clock:            func() int64 { return 1729000100 },
		EVMClient:        mockClient,
		DefaultChainID:   big.NewInt(84532),
		DefaultTokenAddr: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AutoPollReceipts: true,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ch, auth := getValidTestData()
	vResp, err := svc.Verify(context.Background(), &models.VerifyRequest{Challenge: *ch, Authorization: *auth})
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if got := svc.InFlightAmount(ch.Network, ch.Asset, auth.Payer); got.Cmp(big.NewInt(100_000)) != 0 {
		t.Fatalf("expected in-flight 100000 after verify, got %s", got.String())
	}

	if _, err := svc.Settle(context.Background(), &models.SettleRequest{
		VerificationToken: vResp.VerificationToken,
		Authorization:     *auth,
	}); err != nil {
		t.Fatalf("settle failed: %v", err)
	}

	// Wait briefly for pollReceipt goroutine to confirm mock receipt and release in-flight reservation
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if svc.InFlightAmount(ch.Network, ch.Asset, auth.Payer).Sign() == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected in-flight reservation to be released after receipt confirmation, still %s", svc.InFlightAmount(ch.Network, ch.Asset, auth.Payer).String())
}
