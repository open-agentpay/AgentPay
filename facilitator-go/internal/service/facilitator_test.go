package service

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
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
