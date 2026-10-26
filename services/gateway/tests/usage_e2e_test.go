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
	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

func TestRequestAndUsageEventRecording(t *testing.T) {
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
		TenantID:      "ten_rich",
		VersionID:     "ver_usage_test_1",
		Routes:        routes,
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	walletSvc := wallet.NewMemoryService()

	// 1. Rich wallet for ten_rich
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_rich",
		TenantID:       "ten_rich",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 1_000_000,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_rich_key", "ten_rich", "prj_rich")

	// 2. Poor wallet for ten_poor
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

	var upstreamCallCount int32
	mockTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&upstreamCallCount, 1)
		body := `{"ticker":"NVDA","price":135.50}`
		return &http.Response{
			StatusCode: http.StatusOK,
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

	usageSvc := gwServer.UsageService()
	if usageSvc == nil {
		t.Fatalf("expected usage service to be initialized")
	}

	// ------------------------------------------------------------------------
	// Test Case 1: Successful Wallet Request -> RequestRecord + UsageEvent
	// ------------------------------------------------------------------------
	req1 := httptest.NewRequest(http.MethodGet, "http://gateway.test/v1/market-data?ticker=NVDA", nil)
	req1.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req1.Header.Set("Idempotency-Key", "idem_record_test_001")
	req1.Header.Set("X-Session-ID", "sess_001")
	req1.Header.Set("X-Task-ID", "task_001")
	rec1 := httptest.NewRecorder()

	serveWalletRequest(t, gwServer.Handler(), rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", rec1.Code, rec1.Body.String())
	}

	reqs, totalReqs, err := usageSvc.ListRequestRecords(context.Background(), usage.RequestFilter{TenantID: "ten_rich"})
	if err != nil {
		t.Fatalf("failed to list request records: %v", err)
	}
	if totalReqs != 1 || len(reqs) != 1 {
		t.Fatalf("expected 1 request record, got total=%d, len=%d", totalReqs, len(reqs))
	}
	r0 := reqs[0]
	if r0.TenantID != "ten_rich" || r0.StatusCode != 200 || r0.IsReplay != false || r0.ErrorCode != "" {
		t.Fatalf("unexpected request record details: %+v", r0)
	}
	if r0.RouteID != "route_market_data" || r0.IntentID == "" {
		t.Fatalf("missing routeID or intentID in record: %+v", r0)
	}

	usages, totalUsages, err := usageSvc.ListUsageEvents(context.Background(), usage.UsageFilter{TenantID: "ten_rich"})
	if err != nil {
		t.Fatalf("failed to list usage events: %v", err)
	}
	if totalUsages != 1 || len(usages) != 1 {
		t.Fatalf("expected 1 usage event, got total=%d, len=%d", totalUsages, len(usages))
	}
	u0 := usages[0]
	if u0.AmountBaseUnits != 50000 || u0.Asset != "USDC" || u0.PaymentMethod != "wallet" || !u0.IsSuccess {
		t.Fatalf("unexpected usage event details: %+v", u0)
	}
	if u0.IntentID != r0.IntentID {
		t.Fatalf("expected usage event IntentID (%s) to match request IntentID (%s)", u0.IntentID, r0.IntentID)
	}

	// ------------------------------------------------------------------------
	// Test Case 2: Idempotent Replay (Cache Hit) -> RequestRecord (IsReplay: true), NO additional UsageEvent
	// ------------------------------------------------------------------------
	req2 := httptest.NewRequest(http.MethodGet, "http://gateway.test/v1/market-data?ticker=NVDA", nil)
	req2.Header.Set("Authorization", "Bearer apk_live_rich_key")
	req2.Header.Set("Idempotency-Key", "idem_record_test_001")
	rec2 := httptest.NewRecorder()

	serveWalletRequest(t, gwServer.Handler(), rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on replay, got %d", rec2.Code)
	}

	reqsAfterReplay, totalAfterReplay, err := usageSvc.ListRequestRecords(context.Background(), usage.RequestFilter{TenantID: "ten_rich"})
	if err != nil {
		t.Fatalf("failed to list request records after replay: %v", err)
	}
	if totalAfterReplay != 2 || len(reqsAfterReplay) != 2 {
		t.Fatalf("expected 2 request records after replay, got total=%d", totalAfterReplay)
	}
	// Records are sorted descending by timestamp
	replayRec := reqsAfterReplay[0]
	if !replayRec.IsReplay {
		t.Fatalf("expected replay request to have IsReplay=true, got %+v", replayRec)
	}

	// Invariant: No additional usage event emitted for cached replay
	usagesAfterReplay, totalUsagesAfterReplay, err := usageSvc.ListUsageEvents(context.Background(), usage.UsageFilter{TenantID: "ten_rich"})
	if err != nil {
		t.Fatalf("failed to list usage events: %v", err)
	}
	if totalUsagesAfterReplay != 1 || len(usagesAfterReplay) != 1 {
		t.Fatalf("anti-double-billing invariant breached: expected exactly 1 usage event, got total=%d, len=%d", totalUsagesAfterReplay, len(usagesAfterReplay))
	}

	// ------------------------------------------------------------------------
	// Test Case 3: Failed Request (Insufficient Balance) -> ErrorCode recorded, 0 usage
	// ------------------------------------------------------------------------
	req3 := httptest.NewRequest(http.MethodGet, "http://gateway.test/v1/market-data?ticker=NVDA", nil)
	req3.Header.Set("Authorization", "Bearer apk_live_poor_key")
	rec3 := httptest.NewRecorder()

	serveWalletRequest(t, gwServer.Handler(), rec3, req3)
	if rec3.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required, got %d", rec3.Code)
	}

	poorReqs, totalPoorReqs, err := usageSvc.ListRequestRecords(context.Background(), usage.RequestFilter{TenantID: "ten_poor"})
	if err != nil {
		t.Fatalf("failed to list poor request records: %v", err)
	}
	if totalPoorReqs != 1 || len(poorReqs) != 1 {
		t.Fatalf("expected 1 request record for ten_poor, got %d", totalPoorReqs)
	}
	poorRec := poorReqs[0]
	if poorRec.StatusCode != http.StatusPaymentRequired || poorRec.ErrorCode != "ERR_WALLET_INSUFFICIENT_BALANCE" {
		t.Fatalf("expected 402 ERR_WALLET_INSUFFICIENT_BALANCE, got %+v", poorRec)
	}

	poorUsages, totalPoorUsages, err := usageSvc.ListUsageEvents(context.Background(), usage.UsageFilter{TenantID: "ten_poor"})
	if err != nil {
		t.Fatalf("failed to list poor usage events: %v", err)
	}
	if totalPoorUsages != 0 || len(poorUsages) != 0 {
		t.Fatalf("expected 0 usage events for rejected request, got total=%d, len=%d", totalPoorUsages, len(poorUsages))
	}

	// ------------------------------------------------------------------------
	// Test Case 4: Admin API Endpoints for Requests & Usages
	// ------------------------------------------------------------------------
	adminReq := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/ten_rich/requests?page=1&page_size=10", nil)
	adminRec := httptest.NewRecorder()
	gwServer.Handler().ServeHTTP(adminRec, adminReq)

	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected admin requests query to return 200, got %d, body: %s", adminRec.Code, adminRec.Body.String())
	}
	var reqResp struct {
		Requests []*usage.RequestRecord `json:"requests"`
		Total    int64                  `json:"total"`
	}
	if err := json.Unmarshal(adminRec.Body.Bytes(), &reqResp); err != nil {
		t.Fatalf("failed to parse admin requests response: %v", err)
	}
	if reqResp.Total != 2 || len(reqResp.Requests) != 2 {
		t.Fatalf("expected 2 admin request records, got total=%d, len=%d", reqResp.Total, len(reqResp.Requests))
	}

	adminUsageReq := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/ten_rich/usages?page=1&page_size=10", nil)
	adminUsageRec := httptest.NewRecorder()
	gwServer.Handler().ServeHTTP(adminUsageRec, adminUsageReq)

	if adminUsageRec.Code != http.StatusOK {
		t.Fatalf("expected admin usages query to return 200, got %d, body: %s", adminUsageRec.Code, adminUsageRec.Body.String())
	}
	var usageResp struct {
		Usages []*usage.UsageEvent `json:"usages"`
		Total  int64               `json:"total"`
	}
	if err := json.Unmarshal(adminUsageRec.Body.Bytes(), &usageResp); err != nil {
		t.Fatalf("failed to parse admin usages response: %v", err)
	}
	if usageResp.Total != 1 || len(usageResp.Usages) != 1 {
		t.Fatalf("expected 1 admin usage event, got total=%d, len=%d", usageResp.Total, len(usageResp.Usages))
	}
}
