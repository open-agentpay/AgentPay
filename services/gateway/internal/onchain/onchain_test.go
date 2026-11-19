package onchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestGasPricingEngineModes(t *testing.T) {
	cfg := GasPolicyConfig{
		DefaultMode:         GasModeSubsidized,
		SlippageBufferPct:   10.0,
		MaxAcceptableGasUSD: 0.05,
		Networks: map[string]NetworkGasPolicy{
			"eip155:421614": {
				Mode:                GasModeDynamicSurcharge,
				SlippageBufferPct:   10.0,
				MaxAcceptableGasUSD: 0.05,
			},
			"eip155:84532": {
				Mode:                GasModeSubsidized,
				SlippageBufferPct:   10.0,
				MaxAcceptableGasUSD: 0.05,
			},
			"eip155:10": {
				Mode:                GasModeBatchAggregated,
				SlippageBufferPct:   10.0,
				MaxAcceptableGasUSD: 0.05,
				BatchAmortizeFactor: 5,
			},
		},
	}

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody, _ := json.Marshal(map[string]interface{}{
				"surchargeBaseUnits":  int64(500), // 0.0005 USDC
				"estimatedGasCostUsd": "0.000500",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}),
	}

	engine := NewGasPricingEngine(cfg, "http://facilitator.internal", "test_key")
	engine.httpClient = mockClient

	ctx := context.Background()

	// 1. Mode 1: Subsidized (Base Sepolia) -> 0 surcharge
	total, breakdown, err := engine.CalculateChallengePrice(ctx, "eip155:84532", "0xUSDC", 1000)
	if err != nil {
		t.Fatalf("unexpected error for subsidized: %v", err)
	}
	if total != 1000 || breakdown.GasSurcharge != 0 || breakdown.GasMode != GasModeSubsidized {
		t.Fatalf("expected total 1000 and 0 surcharge, got total=%d breakdown=%+v", total, breakdown)
	}

	// 2. Mode 2: Dynamic Surcharge (Arbitrum Sepolia) -> 1000 + 500 = 1500
	total, breakdown, err = engine.CalculateChallengePrice(ctx, "eip155:421614", "0xUSDC", 1000)
	if err != nil {
		t.Fatalf("unexpected error for dynamic surcharge: %v", err)
	}
	if total != 1500 || breakdown.GasSurcharge != 500 || breakdown.GasMode != GasModeDynamicSurcharge {
		t.Fatalf("expected total 1500 and 500 surcharge, got total=%d breakdown=%+v", total, breakdown)
	}

	// 3. Mode 3: Batch Aggregated (Optimism) -> 1000 + (500 / 5) = 1100
	total, breakdown, err = engine.CalculateChallengePrice(ctx, "eip155:10", "0xUSDC", 1000)
	if err != nil {
		t.Fatalf("unexpected error for batch aggregated: %v", err)
	}
	if total != 1100 || breakdown.GasSurcharge != 100 || breakdown.GasMode != GasModeBatchAggregated {
		t.Fatalf("expected total 1100 and 100 surcharge, got total=%d breakdown=%+v", total, breakdown)
	}
}

func TestGasPricingEngineCircuitBreaker(t *testing.T) {
	cfg := GasPolicyConfig{
		DefaultMode:         GasModeDynamicSurcharge,
		MaxAcceptableGasUSD: 0.01, // Max $0.01
	}

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			respBody, _ := json.Marshal(map[string]interface{}{
				"surchargeBaseUnits":  int64(50000), // $0.05
				"estimatedGasCostUsd": "0.050000",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}),
	}

	engine := NewGasPricingEngine(cfg, "http://facilitator.internal", "test_key")
	engine.httpClient = mockClient

	_, _, err := engine.CalculateChallengePrice(context.Background(), "eip155:1", "0xUSDC", 1000)
	if err == nil || !errors.Is(err, ErrGasSurchargeExceedsLimit) {
		t.Fatalf("expected ErrGasSurchargeExceedsLimit circuit breaker, got: %v", err)
	}
}

func TestFacilitatorVerifierAsyncSettlement(t *testing.T) {
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			auth := req.Header.Get("Authorization")
			if auth != "Bearer test_api_key" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"message":"unauthorized"}`))),
				}, nil
			}

			if path == "/v1/settle" && req.Method == http.MethodPost {
				respBody, _ := json.Marshal(map[string]interface{}{
					"settlementId": "stl_test_123",
					"status":       "submitted",
					"txHash":       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				})
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(respBody)),
				}, nil
			}

			if path == "/v1/settle/stl_test_123" && req.Method == http.MethodGet {
				respBody, _ := json.Marshal(map[string]interface{}{
					"settlementId": "stl_test_123",
					"status":       "confirmed",
					"txHash":       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				})
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(respBody)),
				}, nil
			}

			if path == "/v1/settle/lookup" && req.Method == http.MethodGet {
				nonce := req.URL.Query().Get("nonce")
				if nonce == "0xnonce123" {
					respBody, _ := json.Marshal(map[string]interface{}{
						"settlementId": "stl_test_123",
						"status":       "confirmed",
						"txHash":       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					})
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(bytes.NewReader(respBody)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"SETTLEMENT_NOT_FOUND"}`))),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"not found"}`))),
			}, nil
		}),
	}

	verifier := NewFacilitatorVerifierWithAuth("http://facilitator.internal", "test_api_key", 2*time.Second)
	verifier.SetHTTPClient(mockClient)

	// Verify interface compliance
	var asyncVerifier x402.AsyncSettlementVerifier = verifier
	if asyncVerifier == nil {
		t.Fatalf("expected verifier to implement x402.AsyncSettlementVerifier")
	}

	ctx := context.Background()

	// 1. SettlePayment test
	auth := &x402.PaymentAuthorization{
		ChallengeID: "ch_test",
		Payer:       "0x1111111111111111111111111111111111111111",
		Nonce:       "0xnonce123",
	}
	settleRes, err := verifier.SettlePayment(ctx, "vtok_123", auth, "idem_abc")
	if err != nil {
		t.Fatalf("SettlePayment failed: %v", err)
	}
	if settleRes.SettlementID != "stl_test_123" || settleRes.Status != "submitted" || settleRes.TxHash == "" {
		t.Fatalf("unexpected SettlePayment response: %+v", settleRes)
	}

	// 2. GetSettlementStatus test
	status, err := verifier.GetSettlementStatus(ctx, "stl_test_123")
	if err != nil {
		t.Fatalf("GetSettlementStatus failed: %v", err)
	}
	if status.SettlementID != "stl_test_123" || status.Status != "confirmed" {
		t.Fatalf("unexpected GetSettlementStatus response: %+v", status)
	}

	// 3. GetSettlementStatusByNonce test
	lookupStatus, err := verifier.GetSettlementStatusByNonce(ctx, "eip155:421614", "0xUSDC", "0x1111111111111111111111111111111111111111", "0xnonce123")
	if err != nil {
		t.Fatalf("GetSettlementStatusByNonce failed: %v", err)
	}
	if lookupStatus.SettlementID != "stl_test_123" || lookupStatus.Status != "confirmed" {
		t.Fatalf("unexpected GetSettlementStatusByNonce response: %+v", lookupStatus)
	}
}
