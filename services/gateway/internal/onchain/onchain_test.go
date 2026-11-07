package onchain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/open-agentpay/agent-core-sdk/gateway/settlement"
	"github.com/open-agentpay/agent-core-sdk/gateway/x402"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFacilitatorVerifierLifecycle(t *testing.T) {
	var receivedAuthHeader string
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			receivedAuthHeader = req.Header.Get("Authorization")
			respBody, _ := json.Marshal(map[string]interface{}{
				"valid":             true,
				"verifiedPayer":     "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
				"verificationToken": "tok_verify_test_123",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}),
	}

	verifier := NewFacilitatorVerifierWithAuth("http://facilitator.internal", "test_api_key", 2*time.Second)
	verifier.SetHTTPClient(mockClient)

	now := time.Now().Unix()
	ch := &x402.PaymentChallenge{
		ChallengeID: "ch_test_123",
		Network:     "eip155:421614",
		Asset:       "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d",
		ValidAfter:  now - 10,
		ValidBefore: now + 300,
	}
	auth := &x402.PaymentAuthorization{
		ChallengeID: "ch_test_123",
		Payer:       "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
		ValidAfter:  now - 10,
		ValidBefore: now + 300,
		Nonce:       "0x1234",
	}

	res, err := verifier.VerifyPayment(context.Background(), ch, auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid verification, got: %+v", res)
	}
	if res.VerifiedPayer != "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266" {
		t.Fatalf("unexpected verified payer: %s", res.VerifiedPayer)
	}
	if receivedAuthHeader != "Bearer test_api_key" {
		t.Fatalf("expected Authorization header Bearer test_api_key, got: %s", receivedAuthHeader)
	}
}

func TestCryptoSettlementAdapter(t *testing.T) {
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody, _ := json.Marshal(map[string]interface{}{
				"valid":             true,
				"verifiedPayer":     "0x1111111111111111111111111111111111111111",
				"verificationToken": "tok_settle_abc",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}),
	}

	verifier := NewFacilitatorVerifier("http://facilitator.internal", 2*time.Second)
	verifier.SetHTTPClient(mockClient)
	adapter := NewCryptoSettlementAdapter(verifier)

	if adapter.Name() != "crypto-facilitator" {
		t.Errorf("unexpected name: %s", adapter.Name())
	}
	if adapter.Mode() != "crypto" {
		t.Errorf("unexpected mode: %s", adapter.Mode())
	}

	now := time.Now().Unix()
	ch := &x402.PaymentChallenge{
		ChallengeID: "ch_adapter_test",
		ValidAfter:  now - 10,
		ValidBefore: now + 300,
	}
	auth := &x402.PaymentAuthorization{
		ChallengeID: "ch_adapter_test",
		Payer:       "0x1111111111111111111111111111111111111111",
		ValidAfter:  now - 10,
		ValidBefore: now + 300,
	}

	authReq := &settlement.AuthorizationRequest{
		Metadata: map[string]interface{}{
			"payment_challenge":     ch,
			"payment_authorization": auth,
		},
	}

	authRes, err := adapter.Authorize(context.Background(), authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	if !authRes.Authorized || authRes.AuthorizationID != "tok_settle_abc" {
		t.Fatalf("unexpected Authorize result: %+v", authRes)
	}

	settleReq := &settlement.SettlementRequest{
		AuthorizationID: authRes.AuthorizationID,
	}
	settleRes, err := adapter.Settle(context.Background(), settleReq)
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}
	if !settleRes.Success || settleRes.SettlementID != "tok_settle_abc" {
		t.Fatalf("unexpected Settle result: %+v", settleRes)
	}

	if err := adapter.Release(context.Background(), authRes.AuthorizationID); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}

func TestKnownNetworks(t *testing.T) {
	arb, exists := KnownNetworks["eip155:421614"]
	if !exists {
		t.Fatalf("expected Arbitrum Sepolia in KnownNetworks")
	}
	if arb.ChainID != 421614 {
		t.Errorf("unexpected chain ID: %d", arb.ChainID)
	}
}
