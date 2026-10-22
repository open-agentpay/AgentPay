package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

func setupWalletE2EServer(t *testing.T, upstreamHandler func(*http.Request) (*http.Response, error)) (*server.GatewayServer, wallet.Service) {
	configPath := filepath.Join("..", "configs", "gateway.example.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load example config: %v", err)
	}

	// Routes setup:
	// /v1/market-data: allowed_payment_methods: both, price: 50000
	// /v1/onchain-only: allowed_payment_methods: onchain, price: 50000
	bootstrapRoutes := []rcu.RouteConfig{
		{
			RouteID:               "route_market_data",
			PathPattern:           "/v1/market-data",
			UpstreamURL:           "http://mock-upstream/v1/market-data",
			PriceBaseUnits:        50000,
			Asset:                 "USDC",
			PayeeAddress:          "0x1111111111111111111111111111111111111111",
			AllowedPaymentMethods: "both",
			Status:                "active",
		},
		{
			RouteID:               "route_onchain_only",
			PathPattern:           "/v1/onchain-only",
			UpstreamURL:           "http://mock-upstream/v1/onchain-only",
			PriceBaseUnits:        50000,
			Asset:                 "USDC",
			PayeeAddress:          "0x1111111111111111111111111111111111111111",
			AllowedPaymentMethods: "onchain",
			Status:                "active",
		},
	}

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_test",
		VersionID:     "ver_wallet_e2e_1",
		Routes:        bootstrapRoutes,
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	walletSvc := wallet.NewMemoryService()

	// 1. Rich wallet with 1,000,000 base units (1 USDC)
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_rich",
		TenantID:       "ten_rich",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 1000000,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_rich_key", "ten_rich", "prj_rich")

	// 2. Poor wallet with 100 base units
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_poor",
		TenantID:       "ten_poor",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 100,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_poor_key", "ten_poor", "prj_poor")

	// 3. Frozen/Arrears wallet with -50000 base units
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_frozen",
		TenantID:       "ten_frozen",
		Asset:          "USDC",
		Status:         wallet.StatusFrozenArrears,
		Version:        1,
		AvailableCache: -50000,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_frozen_key", "ten_frozen", "prj_frozen")

	m := metrics.New()
	transport := roundTripFunc(upstreamHandler)

	srv, err := server.NewWithWalletAndTransport(cfg, rcuMgr, m, walletSvc, transport)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return srv, walletSvc
}

// TestWalletPaymentPathSuccessCapture verifies 200 OK upstream captures funds.
func TestWalletPaymentPathSuccessCapture(t *testing.T) {
	ctx := context.Background()
	srv, walletSvc := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"data":"market_data_payload"}`)),
			Request:    r,
		}, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(`{"symbol":"ETH"}`))
	req.Header.Set("Authorization", "Bearer apk_live_rich_key")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify wallet available balance is reduced by 50,000 and held is 0
	wlt, err := walletSvc.GetWallet(ctx, "wlt_rich")
	if err != nil {
		t.Fatalf("failed to get wallet: %v", err)
	}

	if wlt.AvailableCache != 950000 || wlt.HeldCache != 0 {
		t.Fatalf("expected available=950000, held=0; got available=%d, held=%d", wlt.AvailableCache, wlt.HeldCache)
	}

	// Verify immutable ledger entry
	entries, err := walletSvc.GetLedgerEntries(ctx, "wlt_rich")
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got: %d", len(entries))
	}
	if entries[0].Type != wallet.LedgerTypeCapture || entries[0].Amount != -50000 {
		t.Fatalf("unexpected ledger entry: %+v", entries[0])
	}

	// Verify reconciliation is balanced
	report, err := walletSvc.ReconcileBalance(ctx, "wlt_rich")
	if err != nil || !report.IsBalanced {
		t.Fatalf("reconciliation failed: %+v", report)
	}
}

// TestWalletPaymentPathUpstream4xxRelease verifies 4xx upstream releases funds.
func TestWalletPaymentPathUpstream4xxRelease(t *testing.T) {
	ctx := context.Background()
	srv, walletSvc := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"bad_input_symbol"}`)),
			Request:    r,
		}, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(`{"invalid":true}`))
	req.Header.Set("X-API-Key", "apk_live_rich_key")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d, body: %s", w.Code, w.Body.String())
	}

	// Funds must be fully released back to 1,000,000
	wlt, err := walletSvc.GetWallet(ctx, "wlt_rich")
	if err != nil {
		t.Fatalf("failed to get wallet: %v", err)
	}

	if wlt.AvailableCache != 1000000 || wlt.HeldCache != 0 {
		t.Fatalf("funds not released: available=%d, held=%d", wlt.AvailableCache, wlt.HeldCache)
	}

	// No capture entry written to ledger
	entries, _ := walletSvc.GetLedgerEntries(ctx, "wlt_rich")
	if len(entries) != 0 {
		t.Fatalf("expected 0 ledger entries, got: %d", len(entries))
	}
}

// TestWalletPaymentPathUpstream5xxHoldsFunds verifies W4 invariant (holds preserved on 5xx).
func TestWalletPaymentPathUpstream5xxHoldsFunds(t *testing.T) {
	ctx := context.Background()
	srv, walletSvc := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"internal_service_down"}`)),
			Request:    r,
		}, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(`{"symbol":"ETH"}`))
	req.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req.Header.Set("Idempotency-Key", "idem_5xx_hold_test")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got: %d", w.Code)
	}

	// W4 Invariant: Hold remains preserved in held state!
	wlt, _ := walletSvc.GetWallet(ctx, "wlt_rich")
	if wlt.AvailableCache != 950000 || wlt.HeldCache != 50000 {
		t.Fatalf("W4 VIOLATION: expected held=50000, available=950000; got available=%d, held=%d", wlt.AvailableCache, wlt.HeldCache)
	}
}

// TestWalletPaymentPathIdempotencyAndFreeReplay verifies deduplication & recovery.
func TestWalletPaymentPathIdempotencyAndFreeReplay(t *testing.T) {
	ctx := context.Background()
	var callCount int32
	srv, walletSvc := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		val := atomic.AddInt32(&callCount, 1)
		if val == 1 {
			// First call fails with 503
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":"temporarily_unavailable"}`)),
				Request:    r,
			}, nil
		}
		// Second call succeeds with 200
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"delivered"}`)),
			Request:    r,
		}, nil
	})

	handler := srv.Handler()

	idemKey := "idem_free_replay_999"
	body := `{"query":"historical_data"}`

	// 1. First attempt fails upstream with 503
	req1 := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(body))
	req1.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req1.Header.Set("Idempotency-Key", idemKey)
	w1 := httptest.NewRecorder()
	serveWalletRequest(t, handler, w1, req1)

	if w1.Code != http.StatusServiceUnavailable {
		t.Fatalf("attempt 1 expected 503, got: %d", w1.Code)
	}

	wlt1, _ := walletSvc.GetWallet(ctx, "wlt_rich")
	if wlt1.HeldCache != 50000 {
		t.Fatalf("attempt 1 must hold funds: %d", wlt1.HeldCache)
	}

	// 2. Retry with same Idempotency-Key triggers Free Replay (no second hold!)
	req2 := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(body))
	req2.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req2.Header.Set("Idempotency-Key", idemKey)
	w2 := httptest.NewRecorder()
	serveWalletRequest(t, handler, w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("attempt 2 expected 200 OK Free Replay, got: %d", w2.Code)
	}

	wlt2, _ := walletSvc.GetWallet(ctx, "wlt_rich")
	if wlt2.AvailableCache != 950000 || wlt2.HeldCache != 0 {
		t.Fatalf("after Free Replay capture, expected available=950000, held=0; got available=%d, held=%d", wlt2.AvailableCache, wlt2.HeldCache)
	}

	// 3. Third request with same Idempotency-Key returns cached response with X-Cache: HIT
	req3 := httptest.NewRequest("POST", "/v1/market-data", bytes.NewBufferString(body))
	req3.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req3.Header.Set("Idempotency-Key", idemKey)
	w3 := httptest.NewRecorder()
	serveWalletRequest(t, handler, w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("attempt 3 expected 200 OK, got: %d", w3.Code)
	}
	if w3.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache: HIT, got: %s", w3.Header().Get("X-Cache"))
	}

	// Still only 50,000 deducted in total, exactly 1 ledger capture entry!
	wlt3, _ := walletSvc.GetWallet(ctx, "wlt_rich")
	if wlt3.AvailableCache != 950000 {
		t.Fatalf("double charging detected: available=%d", wlt3.AvailableCache)
	}
	entries, _ := walletSvc.GetLedgerEntries(ctx, "wlt_rich")
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger capture entry, got: %d", len(entries))
	}
}

// TestWalletPaymentPathInsufficientBalanceNoDowngrade verifies W5 invariant.
func TestWalletPaymentPathInsufficientBalanceNoDowngrade(t *testing.T) {
	srv, _ := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		t.Fatal("upstream should not be called when balance is insufficient")
		return nil, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	req.Header.Set("Authorization", "Bearer apk_live_poor_key")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	// W5: Must return 402 Insufficient Balance
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required, got: %d", w.Code)
	}

	// W5: MUST NOT contain PAYMENT-REQUIRED challenge (strictly forbidden to silently downgrade to onchain!)
	if w.Header().Get("PAYMENT-REQUIRED") != "" {
		t.Fatalf("W5 INVARIANT VIOLATION: PAYMENT-REQUIRED challenge header emitted on wallet insufficient balance!")
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != "ERR_WALLET_INSUFFICIENT_BALANCE" {
		t.Fatalf("expected code ERR_WALLET_INSUFFICIENT_BALANCE, got: %v", errObj["code"])
	}
}

// TestWalletPaymentPathFrozenWalletRejected verifies W6 invariant.
func TestWalletPaymentPathFrozenWalletRejected(t *testing.T) {
	srv, _ := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		t.Fatal("upstream should not be called when wallet is frozen")
		return nil, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/v1/market-data", nil)
	req.Header.Set("Authorization", "Bearer apk_live_frozen_key")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	// W6: Must return 403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got: %d", w.Code)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != "ERR_WALLET_FROZEN" {
		t.Fatalf("expected code ERR_WALLET_FROZEN, got: %v", errObj["code"])
	}
}

// TestWalletPaymentPathOnchainOnlyRouteRejected verifies payment method routing guard.
func TestWalletPaymentPathOnchainOnlyRouteRejected(t *testing.T) {
	srv, _ := setupWalletE2EServer(t, func(r *http.Request) (*http.Response, error) {
		t.Fatal("upstream should not be called when payment method is disallowed")
		return nil, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/v1/onchain-only", nil)
	req.Header.Set("Authorization", "Bearer apk_live_rich_key")
	w := httptest.NewRecorder()

	serveWalletRequest(t, handler, w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got: %d", w.Code)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	errObj := errResp["error"].(map[string]interface{})
	if errObj["code"] != "ERR_PAYMENT_METHOD_NOT_ALLOWED" {
		t.Fatalf("expected code ERR_PAYMENT_METHOD_NOT_ALLOWED, got: %v", errObj["code"])
	}
}
