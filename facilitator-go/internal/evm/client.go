package evm

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client defines EVM RPC operations required by Facilitator.
type Client interface {
	CheckAuthorizationState(ctx context.Context, tokenContract common.Address, authorizer common.Address, nonce [32]byte) (bool, error)
	GetNonce(ctx context.Context, account common.Address) (uint64, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendRawTransaction(ctx context.Context, tx *types.Transaction) (common.Hash, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	ChainID() *big.Int
	CodeAt(ctx context.Context, account common.Address) ([]byte, error)
}

// RPCClient wraps go-ethereum's ethclient.
type RPCClient struct {
	client  *ethclient.Client
	chainID *big.Int
}

// NewRPCClient initializes a real JSON-RPC client connected to an Ethereum/Arbitrum/Base node.
func NewRPCClient(ctx context.Context, rpcURL string, expectedChainID *big.Int) (*RPCClient, error) {
	c, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to dial EVM RPC endpoint %s: %w", rpcURL, err)
	}

	chainID := expectedChainID
	if chainID == nil || chainID.Sign() == 0 {
		netID, err := c.ChainID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve chain ID from RPC: %w", err)
		}
		chainID = netID
	}

	return &RPCClient{
		client:  c,
		chainID: chainID,
	}, nil
}

func (r *RPCClient) ChainID() *big.Int {
	return r.chainID
}

func (r *RPCClient) CodeAt(ctx context.Context, account common.Address) ([]byte, error) {
	return r.client.CodeAt(ctx, account, nil)
}

func (r *RPCClient) CheckAuthorizationState(ctx context.Context, tokenContract common.Address, authorizer common.Address, nonce [32]byte) (bool, error) {
	calldata := EncodeAuthorizationState(authorizer, nonce)
	msg := ethereum.CallMsg{
		To:   &tokenContract,
		Data: calldata,
	}
	out, err := r.client.CallContract(ctx, msg, nil)
	if err != nil {
		return false, err
	}
	return DecodeAuthorizationStateResult(out), nil
}

func (r *RPCClient) GetNonce(ctx context.Context, account common.Address) (uint64, error) {
	return r.client.PendingNonceAt(ctx, account)
}

func (r *RPCClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return r.client.SuggestGasTipCap(ctx)
}

func (r *RPCClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return r.client.SuggestGasPrice(ctx)
}

func (r *RPCClient) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return r.client.EstimateGas(ctx, msg)
}

func (r *RPCClient) SendRawTransaction(ctx context.Context, tx *types.Transaction) (common.Hash, error) {
	err := r.client.SendTransaction(ctx, tx)
	if err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

func (r *RPCClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return r.client.TransactionReceipt(ctx, txHash)
}

// MockClient provides an in-memory client for testing and simulation without requiring live RPC.
type MockClient struct {
	mu             sync.RWMutex
	chainID        *big.Int
	usedAuthNonces map[string]bool
	receipts       map[common.Hash]*types.Receipt
	txNonce        uint64
	GasPriceVal    *big.Int
	GasTipCapVal   *big.Int
}

// NewMockClient creates an in-memory client.
func NewMockClient(chainID *big.Int) *MockClient {
	if chainID == nil {
		chainID = big.NewInt(421614) // Arbitrum Sepolia default
	}
	return &MockClient{
		chainID:        chainID,
		usedAuthNonces: make(map[string]bool),
		receipts:       make(map[common.Hash]*types.Receipt),
		GasPriceVal:    big.NewInt(100_000_000), // 0.1 Gwei
		GasTipCapVal:   big.NewInt(1_000_000),   // 0.001 Gwei
	}
}

func (m *MockClient) ChainID() *big.Int {
	return m.chainID
}

func (m *MockClient) CodeAt(ctx context.Context, account common.Address) ([]byte, error) {
	return nil, nil
}

func (m *MockClient) CheckAuthorizationState(ctx context.Context, tokenContract common.Address, authorizer common.Address, nonce [32]byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s:%x", tokenContract.Hex(), authorizer.Hex(), nonce)
	return m.usedAuthNonces[key], nil
}

func (m *MockClient) MarkAuthorizationUsed(tokenContract common.Address, authorizer common.Address, nonce [32]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s:%x", tokenContract.Hex(), authorizer.Hex(), nonce)
	m.usedAuthNonces[key] = true
}

func (m *MockClient) GetNonce(ctx context.Context, account common.Address) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.txNonce
	m.txNonce++
	return n, nil
}

func (m *MockClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return m.GasTipCapVal, nil
}

func (m *MockClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return m.GasPriceVal, nil
}

func (m *MockClient) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return 85000, nil
}

func (m *MockClient) SendRawTransaction(ctx context.Context, tx *types.Transaction) (common.Hash, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := tx.Hash()
	m.receipts[h] = &types.Receipt{
		Status:      types.ReceiptStatusSuccessful,
		TxHash:      h,
		BlockNumber: big.NewInt(12345),
	}
	return h, nil
}

func (m *MockClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.receipts[txHash]
	if !ok {
		return nil, ethereum.NotFound
	}
	return r, nil
}

// WaitForReceipt polls the receipt until confirmation or timeout.
func WaitForReceipt(ctx context.Context, client Client, txHash common.Hash, timeout time.Duration, pollInterval time.Duration) (*types.Receipt, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctxTimeout.Done():
			return nil, ctxTimeout.Err()
		case <-ticker.C:
			r, err := client.TransactionReceipt(ctxTimeout, txHash)
			if err == nil && r != nil {
				return r, nil
			}
		}
	}
}
