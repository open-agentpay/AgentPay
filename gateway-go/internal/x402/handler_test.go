package x402

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/proxy"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func setupTestProcessor(walletSvc WalletService, verifier PaymentVerifier, nonceStore NonceStore) (*PaymentProcessor, *metrics.Metrics) {
	m := metrics.New()
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"data":"upstream-response"}`)),
			Request:    r,
		}, nil
	})
	proxyHandler := proxy.NewHandler(m, mockTransport)
	processor := NewPaymentProcessor(verifier, nonceStore, walletSvc, proxyHandler, m)
	return processor, m
}

func TestUnauthenticatedIssues402Challenge(t *testing.T) {
	processor, m := setupTestProcessor(nil, nil, nil)
	route := &rcu.RouteConfig{
		RouteID:               "rou_market",
		PriceBaseUnits:        50000,
		Asset:                 "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		PayeeAddress:          "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		AllowedPaymentMethods: "both",
	}

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	w := httptest.NewRecorder()

	processor.HandleRequest(w, req, route, "req_001")

	// 1. Status must be 402 Payment Required
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status 402, got %d", w.Code)
	}

	// 2. Must contain PAYMENT-REQUIRED challenge header
	chHeader := w.Header().Get(HeaderPaymentRequired)
	if chHeader == "" {
		t.Fatalf("missing %s response header", HeaderPaymentRequired)
	}

	// 3. Challenge must decode cleanly
	ch, err := DecodeChallenge(chHeader)
	if err != nil {
		t.Fatalf("failed to decode challenge: %v", err)
	}
	if ch.Amount != "50000" || ch.AssetSymbol != "USDC" {
		t.Errorf("challenge content mismatch: %+v", ch)
	}

	// 4. Response body must match canonical error schema
	var errEnvelope ProtocolErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errEnvelope); err != nil {
		t.Fatalf("failed to parse json error envelope: %v", err)
	}
	if errEnvelope.Error.Code != ErrCodePaymentRequired {
		t.Errorf("expected error code %s, got %s", ErrCodePaymentRequired, errEnvelope.Error.Code)
	}
	if errEnvelope.Error.RequestID != "req_001" {
		t.Errorf("expected requestId req_001, got %s", errEnvelope.Error.RequestID)
	}

	// 5. Metrics updated
	if m.ChallengesIssuedTotal != 1 {
		t.Errorf("expected 1 challenge metric incremented, got %d", m.ChallengesIssuedTotal)
	}
}

func TestWalletPaymentSuccessAndAccounting(t *testing.T) {
	walletSvc := NewMemoryWalletService()
	walletSvc.RegisterWallet("apk_live_valid123", &WalletAccount{
		WalletID:         "wlt_tenant_01",
		TenantID:         "ten_01",
		Status:           "active",
		AvailableBalance: 1000000,
		HeldBalance:      0,
	})

	processor, m := setupTestProcessor(walletSvc, nil, nil)
	route := &rcu.RouteConfig{
		RouteID:               "rou_market",
		UpstreamURL:           "http://upstream.internal:8000",
		PriceBaseUnits:        50000,
		Asset:                 "USDC",
		AllowedPaymentMethods: "both",
	}

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	req.Header.Set("Authorization", "Bearer apk_live_valid123")
	w := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w, req, route, "req_002")

	// Must succeed and proxy through
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", w.Code, w.Body.String())
	}

	// Check wallet accounting: 50,000 deducted from available, held then captured
	wallet, _ := walletSvc.GetWalletByAPIKey(req.Context(), "apk_live_valid123")
	if wallet.AvailableBalance != 950000 {
		t.Errorf("expected available balance 950000, got %d", wallet.AvailableBalance)
	}
	if wallet.HeldBalance != 0 {
		t.Errorf("expected held balance 0 after capture, got %d", wallet.HeldBalance)
	}

	// Verify metrics
	if m.HoldsCreatedTotal != 1 || m.HoldsCapturedTotal != 1 || m.PaymentsVerifiedTotal != 1 {
		t.Errorf("metrics mismatch: holdsCreated=%d, holdsCaptured=%d, paymentsVerified=%d",
			m.HoldsCreatedTotal, m.HoldsCapturedTotal, m.PaymentsVerifiedTotal)
	}
}

func TestWalletInsufficientBalanceStrictNoDowngrade(t *testing.T) {
	walletSvc := NewMemoryWalletService()
	walletSvc.RegisterWallet("apk_live_poor", &WalletAccount{
		WalletID:         "wlt_tenant_02",
		TenantID:         "ten_02",
		Status:           "active",
		AvailableBalance: 20000, // less than route price of 50000
		HeldBalance:      0,
	})

	processor, m := setupTestProcessor(walletSvc, nil, nil)
	route := &rcu.RouteConfig{
		RouteID:               "rou_market",
		PriceBaseUnits:        50000,
		Asset:                 "USDC",
		AllowedPaymentMethods: "both",
	}

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	req.Header.Set("Authorization", "Bearer apk_live_poor")
	w := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w, req, route, "req_003")

	// STRICT W5 RULE: Must return 402 with ERR_WALLET_INSUFFICIENT_BALANCE
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", w.Code)
	}

	var errEnvelope ProtocolErrorEnvelope
	json.Unmarshal(w.Body.Bytes(), &errEnvelope)
	if errEnvelope.Error.Code != ErrCodeWalletInsufficientBalance {
		t.Errorf("expected %s, got %s", ErrCodeWalletInsufficientBalance, errEnvelope.Error.Code)
	}

	// Verify no challenge header is attached to prevent auto-downgrade confusion
	if w.Header().Get(HeaderPaymentRequired) != "" {
		t.Errorf("payment-required header should not be returned on wallet insufficient error")
	}

	if m.PaymentsFailedTotal != 1 {
		t.Errorf("expected payments failed metric to be 1")
	}
}

func TestWalletFrozenArrearsRejectedWith403(t *testing.T) {
	walletSvc := NewMemoryWalletService()
	walletSvc.RegisterWallet("apk_live_frozen", &WalletAccount{
		WalletID:         "wlt_tenant_03",
		TenantID:         "ten_03",
		Status:           "frozen_arrears",
		AvailableBalance: -10000,
		HeldBalance:      0,
	})

	processor, _ := setupTestProcessor(walletSvc, nil, nil)
	route := &rcu.RouteConfig{
		RouteID:               "rou_market",
		PriceBaseUnits:        50000,
		AllowedPaymentMethods: "both",
	}

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	req.Header.Set("Authorization", "Bearer apk_live_frozen")
	w := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w, req, route, "req_004")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", w.Code)
	}
	var errEnvelope ProtocolErrorEnvelope
	json.Unmarshal(w.Body.Bytes(), &errEnvelope)
	if errEnvelope.Error.Code != ErrCodeWalletFrozen {
		t.Errorf("expected %s, got %s", ErrCodeWalletFrozen, errEnvelope.Error.Code)
	}
}

func TestPaymentMethodMismatch(t *testing.T) {
	processor, _ := setupTestProcessor(nil, nil, nil)
	// Route allows wallet only
	route := &rcu.RouteConfig{
		RouteID:               "rou_wallet_only",
		PriceBaseUnits:        10000,
		AllowedPaymentMethods: "wallet",
	}

	// Client sends PAYMENT-SIGNATURE
	req := httptest.NewRequest("GET", "/v1/wallet-only", nil)
	req.Header.Set(HeaderPaymentSignature, "eyJ2ZXJzaW9uIjoiMi4wIn0")
	w := httptest.NewRecorder()

	processor.HandleRequest(w, req, route, "req_005")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", w.Code)
	}
	var errEnvelope ProtocolErrorEnvelope
	json.Unmarshal(w.Body.Bytes(), &errEnvelope)
	if errEnvelope.Error.Code != ErrCodePaymentMethodNotAllowed {
		t.Errorf("expected %s, got %s", ErrCodePaymentMethodNotAllowed, errEnvelope.Error.Code)
	}
}

func TestOnChainPaymentSignatureAndNonceReplayGuard(t *testing.T) {
	nonceStore := NewMemoryNonceStore()
	verifier := &MockPaymentVerifier{
		ShouldPass: true,
		Payer:      "0x1111111111111111111111111111111111111111",
	}
	processor, m := setupTestProcessor(nil, verifier, nonceStore)

	route := &rcu.RouteConfig{
		RouteID:               "rou_onchain",
		UpstreamURL:           "http://upstream.internal:8000",
		PriceBaseUnits:        50000,
		Asset:                 "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		PayeeAddress:          "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		AllowedPaymentMethods: "onchain",
	}

	now := time.Now().Unix()
	auth := &PaymentAuthorization{
		Version:     "2.0",
		ChallengeID: "ch_valid_01",
		Payer:       "0x1111111111111111111111111111111111111111",
		Scheme:      "EIP-3009",
		Nonce:       "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		ValidAfter:  now - 10,
		ValidBefore: now + 300,
		Signature:   NewSignatureFromString("0xmock_signature"),
	}
	authHeader, _ := EncodeAuthorization(auth, true)

	// 1. First submission succeeds
	req1 := httptest.NewRequest("GET", "/v1/onchain", nil)
	req1.Header.Set(HeaderPaymentSignature, authHeader)
	w1 := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w1, req1, route, "req_006")
	if w1.Code != http.StatusOK {
		t.Fatalf("first submission expected 200 OK, got %d, body: %s", w1.Code, w1.Body.String())
	}
	if m.PaymentsVerifiedTotal != 1 {
		t.Errorf("expected 1 verified payment, got %d", m.PaymentsVerifiedTotal)
	}

	// 2. Replayed submission with exact same Nonce fails with 409 ERR_NONCE_ALREADY_USED
	req2 := httptest.NewRequest("GET", "/v1/onchain", nil)
	req2.Header.Set(HeaderPaymentSignature, authHeader)
	w2 := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w2, req2, route, "req_007")
	if w2.Code != http.StatusConflict {
		t.Fatalf("replayed nonce expected 409 Conflict, got %d", w2.Code)
	}
	var errEnvelope ProtocolErrorEnvelope
	json.Unmarshal(w2.Body.Bytes(), &errEnvelope)
	if errEnvelope.Error.Code != ErrCodeNonceAlreadyUsed {
		t.Errorf("expected %s, got %s", ErrCodeNonceAlreadyUsed, errEnvelope.Error.Code)
	}
}

func TestFreeRouteBypassesPayment(t *testing.T) {
	processor, m := setupTestProcessor(nil, nil, nil)
	route := &rcu.RouteConfig{
		RouteID:        "rou_free",
		UpstreamURL:    "http://upstream.internal:8000",
		PriceBaseUnits: 0, // Free!
	}

	req := httptest.NewRequest("GET", "/v1/free", nil)
	w := httptest.NewRecorder()

	serveAuthorizedWalletRequest(t, processor, w, req, route, "req_free")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for free route, got %d", w.Code)
	}
	if m.ChallengesIssuedTotal != 0 || m.PaymentsVerifiedTotal != 0 {
		t.Errorf("free route should not trigger payment metrics")
	}
}
