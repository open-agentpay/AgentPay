package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/admin"
	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/payment"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

func TestPaymentCorrelationAndThreeLifecyclesE2E(t *testing.T) {
	configPath := filepath.Join("..", "configs", "gateway.example.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load example config: %v", err)
	}

	routes := []rcu.RouteConfig{
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
	}

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_alpha",
		VersionID:     "ver_payment_test_1",
		Routes:        routes,
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	walletSvc := wallet.NewMemoryService()

	// 1. Funded wallet for ten_alpha
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_alpha",
		TenantID:       "ten_alpha",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 1_000_000,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_alpha_key", "ten_alpha", "prj_alpha")

	// 2. Unfunded wallet for ten_beta
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_beta",
		TenantID:       "ten_beta",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 100, // Insufficient for 50000 price
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_beta_key", "ten_beta", "prj_beta")

	var upstreamStatus int32 = http.StatusOK
	mockTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := int(atomic.LoadInt32(&upstreamStatus))
		body := `{"ticker":"NVDA","price":135.50}`
		if status >= 400 {
			body = `{"error":"upstream error"}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Request:    req,
		}, nil
	})

	m := metrics.New()
	gwServer, err := server.NewWithWalletAndTransport(cfg, rcuMgr, m, walletSvc, mockTransport)
	if err != nil {
		t.Fatalf("failed to init gateway server: %v", err)
	}

	paySvc := gwServer.PaymentService()
	if paySvc == nil {
		t.Fatalf("expected payment service to be initialized")
	}

	handler := gwServer.Handler()

	// ------------------------------------------------------------------------
	// Test Case 1: Wallet Payment 200 OK -> COMPLETED
	// ------------------------------------------------------------------------
	t.Run("Wallet 200 OK -> COMPLETED with Hold Captured and Ledger Link", func(t *testing.T) {
		atomic.StoreInt32(&upstreamStatus, http.StatusOK)
		req := httptest.NewRequest("GET", "/v1/market-data?ticker=NVDA", nil)
		req.Header.Set("X-API-Key", "apk_live_alpha_key")
		req.Header.Set("Idempotency-Key", "idem_success_001")
		req.Header.Set("X-Request-ID", "req_success_001")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		audit, err := paySvc.GetPaymentAuditByRequestID(context.Background(), "ten_alpha", "req_success_001")
		if err != nil {
			t.Fatalf("failed to fetch payment audit: %v", err)
		}

		if audit.DisplayStatus != payment.DisplayCompleted {
			t.Errorf("expected DisplayStatus %s, got %s", payment.DisplayCompleted, audit.DisplayStatus)
		}
		if audit.IntentStatus != payment.IntentStatusCompleted {
			t.Errorf("expected IntentStatus %s, got %s", payment.IntentStatusCompleted, audit.IntentStatus)
		}
		if audit.SettlementStatus != payment.SettlementStatusConfirmed {
			t.Errorf("expected SettlementStatus %s, got %s", payment.SettlementStatusConfirmed, audit.SettlementStatus)
		}
		if audit.FulfillmentStatus != payment.FulfillmentStatusSucceeded {
			t.Errorf("expected FulfillmentStatus %s, got %s", payment.FulfillmentStatusSucceeded, audit.FulfillmentStatus)
		}
		if audit.Hold == nil || audit.Hold.Status != "captured" {
			t.Fatalf("expected Hold to be captured, got %+v", audit.Hold)
		}
		if audit.Ledger == nil || audit.Ledger.Type != "capture" || audit.Ledger.Amount != -50000 {
			t.Fatalf("expected Ledger capture with -50000, got %+v", audit.Ledger)
		}
	})

	// ------------------------------------------------------------------------
	// Test Case 2: Wallet Payment Upstream 4xx -> RELEASED_UNBILLED
	// ------------------------------------------------------------------------
	t.Run("Wallet 404 Upstream -> RELEASED_UNBILLED with Hold Released", func(t *testing.T) {
		atomic.StoreInt32(&upstreamStatus, http.StatusNotFound)
		req := httptest.NewRequest("GET", "/v1/market-data?ticker=NVDA", nil)
		req.Header.Set("X-API-Key", "apk_live_alpha_key")
		req.Header.Set("Idempotency-Key", "idem_4xx_002")
		req.Header.Set("X-Request-ID", "req_4xx_002")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}

		audit, err := paySvc.GetPaymentAuditByRequestID(context.Background(), "ten_alpha", "req_4xx_002")
		if err != nil {
			t.Fatalf("failed to fetch payment audit: %v", err)
		}

		if audit.DisplayStatus != payment.DisplayReleasedUnbilled {
			t.Errorf("expected DisplayStatus %s, got %s", payment.DisplayReleasedUnbilled, audit.DisplayStatus)
		}
		if audit.IntentStatus != payment.IntentStatusFailed {
			t.Errorf("expected IntentStatus %s, got %s", payment.IntentStatusFailed, audit.IntentStatus)
		}
		if audit.SettlementStatus != payment.SettlementStatusReleased {
			t.Errorf("expected SettlementStatus %s, got %s", payment.SettlementStatusReleased, audit.SettlementStatus)
		}
		if audit.FulfillmentStatus != payment.FulfillmentStatusFailed {
			t.Errorf("expected FulfillmentStatus %s, got %s", payment.FulfillmentStatusFailed, audit.FulfillmentStatus)
		}
		if audit.Hold == nil || audit.Hold.Status != "released" {
			t.Fatalf("expected Hold to be released, got %+v", audit.Hold)
		}
		if audit.Ledger == nil || audit.Ledger.Type != "hold_release" || audit.Ledger.Amount != 50000 {
			t.Fatalf("expected Ledger release with +50000, got %+v", audit.Ledger)
		}
	})

	// ------------------------------------------------------------------------
	// Test Case 3: Wallet Payment Upstream 5xx -> RECOVERY_REQUIRED
	// ------------------------------------------------------------------------
	t.Run("Wallet 502 Upstream -> RECOVERY_REQUIRED with Hold Preserved", func(t *testing.T) {
		atomic.StoreInt32(&upstreamStatus, http.StatusBadGateway)
		req := httptest.NewRequest("GET", "/v1/market-data?ticker=NVDA", nil)
		req.Header.Set("X-API-Key", "apk_live_alpha_key")
		req.Header.Set("Idempotency-Key", "idem_5xx_003")
		req.Header.Set("X-Request-ID", "req_5xx_003")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d", rec.Code)
		}

		audit, err := paySvc.GetPaymentAuditByRequestID(context.Background(), "ten_alpha", "req_5xx_003")
		if err != nil {
			t.Fatalf("failed to fetch payment audit: %v", err)
		}

		if audit.DisplayStatus != payment.DisplayRecoveryRequired {
			t.Errorf("expected DisplayStatus %s, got %s", payment.DisplayRecoveryRequired, audit.DisplayStatus)
		}
		if !audit.IsRecoveryRequired {
			t.Errorf("expected IsRecoveryRequired to be true")
		}
		if audit.IntentStatus != payment.IntentStatusNeedsReview {
			t.Errorf("expected IntentStatus %s, got %s", payment.IntentStatusNeedsReview, audit.IntentStatus)
		}
		if audit.SettlementStatus != payment.SettlementStatusHeld {
			t.Errorf("expected SettlementStatus %s (held), got %s", payment.SettlementStatusHeld, audit.SettlementStatus)
		}
		if audit.FulfillmentStatus != payment.FulfillmentStatusUnknown {
			t.Errorf("expected FulfillmentStatus %s (unknown), got %s", payment.FulfillmentStatusUnknown, audit.FulfillmentStatus)
		}
		if audit.Hold == nil || audit.Hold.Status != "held" {
			t.Fatalf("expected Hold to remain held for safety, got %+v", audit.Hold)
		}
	})

	// ------------------------------------------------------------------------
	// Test Case 4: Insufficient Balance -> PAYMENT_FAILED
	// ------------------------------------------------------------------------
	t.Run("Insufficient Balance -> PAYMENT_FAILED", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/market-data?ticker=NVDA", nil)
		req.Header.Set("X-API-Key", "apk_live_beta_key")
		req.Header.Set("Idempotency-Key", "idem_poor_004")
		req.Header.Set("X-Request-ID", "req_poor_004")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 Payment Required, got %d", rec.Code)
		}

		audit, err := paySvc.GetPaymentAuditByRequestID(context.Background(), "ten_beta", "req_poor_004")
		if err != nil {
			t.Fatalf("failed to fetch payment audit: %v", err)
		}

		if audit.DisplayStatus != payment.DisplayPaymentFailed {
			t.Errorf("expected DisplayStatus %s, got %s", payment.DisplayPaymentFailed, audit.DisplayStatus)
		}
		if audit.IntentStatus != payment.IntentStatusFailed {
			t.Errorf("expected IntentStatus %s, got %s", payment.IntentStatusFailed, audit.IntentStatus)
		}
		if audit.SettlementStatus != payment.SettlementStatusFailed {
			t.Errorf("expected SettlementStatus %s, got %s", payment.SettlementStatusFailed, audit.SettlementStatus)
		}
	})

	// ------------------------------------------------------------------------
	// Test Case 5: Control Plane Admin REST API & Multi-Tenant Scoping
	// ------------------------------------------------------------------------
	t.Run("Admin REST API & Multi-Tenant Scoping", func(t *testing.T) {
		adminSvc := gwServer.AdminService()
		_, ownerAlpha, err := adminSvc.CreateTenant(context.Background(), "Alpha Org", "ten_alpha", "owner@alpha.com")
		if err != nil {
			t.Fatalf("failed to create tenant alpha in admin service: %v", err)
		}
		_, _, err = adminSvc.CreateTenant(context.Background(), "Beta Org", "ten_beta", "owner@beta.com")
		if err != nil {
			t.Fatalf("failed to create tenant beta in admin service: %v", err)
		}

		authHeader := getTestJWTHeader(t, ownerAlpha.MemberID, ownerAlpha.Email, "ten_alpha", admin.RoleAdmin)

		// 1. GET /api/v1/tenants/ten_alpha/payments
		reqList := httptest.NewRequest("GET", "/api/v1/tenants/ten_alpha/payments", nil)
		reqList.Header.Set("Authorization", authHeader)
		recList := httptest.NewRecorder()
		handler.ServeHTTP(recList, reqList)

		if recList.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from payments list API, got %d: %s", recList.Code, recList.Body.String())
		}

		var listResp struct {
			Audits []*payment.PaymentAuditRecord `json:"payments"`
			Total  int                           `json:"total"`
		}
		if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to parse list response: %v", err)
		}

		if listResp.Total != 3 {
			t.Fatalf("expected 3 audits for ten_alpha, got %d", listResp.Total)
		}

		// 2. Query by Request ID REST endpoint
		reqByReq := httptest.NewRequest("GET", "/api/v1/tenants/ten_alpha/payments/by-request/req_success_001", nil)
		reqByReq.Header.Set("Authorization", authHeader)
		recByReq := httptest.NewRecorder()
		handler.ServeHTTP(recByReq, reqByReq)

		if recByReq.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from by-request API, got %d: %s", recByReq.Code, recByReq.Body.String())
		}

		var singleResp struct {
			Payment *payment.PaymentAuditRecord `json:"payment"`
		}
		if err := json.Unmarshal(recByReq.Body.Bytes(), &singleResp); err != nil {
			t.Fatalf("failed to parse single audit response: %v", err)
		}
		if singleResp.Payment == nil || singleResp.Payment.RequestID != "req_success_001" || singleResp.Payment.DisplayStatus != payment.DisplayCompleted {
			t.Errorf("unexpected audit response: %+v", singleResp)
		}

		// 3. Multi-tenant isolation: ten_alpha cannot query ten_beta's records (returns 403 Forbidden)
		reqCross := httptest.NewRequest("GET", "/api/v1/tenants/ten_alpha/payments/by-request/req_poor_004", nil)
		reqCross.Header.Set("Authorization", authHeader)
		recCross := httptest.NewRecorder()
		handler.ServeHTTP(recCross, reqCross)

		if recCross.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for cross-tenant query, got %d", recCross.Code)
		}

		// 4. Multi-tenant isolation: ten_alpha token cannot access /api/v1/tenants/ten_beta/payments
		reqForbidden := httptest.NewRequest("GET", "/api/v1/tenants/ten_beta/payments", nil)
		reqForbidden.Header.Set("Authorization", authHeader)
		recForbidden := httptest.NewRecorder()
		handler.ServeHTTP(recForbidden, reqForbidden)

		if recForbidden.Code == http.StatusOK {
			t.Fatalf("expected non-200 error for cross-tenant path, got 200 OK")
		}
	})
}

func getTestJWTHeader(t *testing.T, memberID, email, tenantID string, role admin.MemberRole) string {
	t.Helper()
	jwtMgr := auth.NewJWTManager("", 24*time.Hour)
	token, err := jwtMgr.GenerateToken(memberID, email, tenantID, string(role))
	if err != nil {
		t.Fatalf("failed to create test user token: %v", err)
	}
	return fmt.Sprintf("Bearer %s", token)
}
