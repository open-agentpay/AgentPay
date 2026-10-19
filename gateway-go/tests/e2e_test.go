package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGatewaySkeletonE2E(t *testing.T) {
	// 1. Find project root and example config
	configPath := filepath.Join("..", "configs", "gateway.example.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load example config: %v", err)
	}

	// 2. Bootstrap RCU snapshot from config
	bootstrapRoutes := make([]rcu.RouteConfig, len(cfg.Routes))
	for i, r := range cfg.Routes {
		bootstrapRoutes[i] = rcu.RouteConfig{
			RouteID:               r.RouteID,
			PathPattern:           r.Path,
			UpstreamURL:           r.Upstream,
			PriceBaseUnits:        r.PriceBaseUnits,
			Asset:                 r.Asset,
			PayeeAddress:          r.PayeeAddress,
			AllowedPaymentMethods: r.AllowedPaymentMethods,
			Status:                "active",
		}
	}
	initialSnapshot := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_test",
		VersionID:     "ver_e2e_1",
		Routes:        bootstrapRoutes,
	}

	rcuMgr, err := rcu.NewManager(initialSnapshot)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	m := metrics.New()
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"proxied","path":"` + r.URL.Path + `"}`)),
			Request:    r,
		}, nil
	})
	srv, err := server.NewWithTransport(cfg, rcuMgr, m, mockTransport)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := srv.Handler()

	// 3. Verify /healthz
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	wHealth := httptest.NewRecorder()
	handler.ServeHTTP(wHealth, reqHealth)

	if wHealth.Code != http.StatusOK {
		t.Fatalf("healthz check failed: %d", wHealth.Code)
	}
	var healthBody map[string]interface{}
	if err := json.Unmarshal(wHealth.Body.Bytes(), &healthBody); err != nil {
		t.Fatalf("failed to parse health JSON: %v", err)
	}
	if healthBody["status"] != "ok" {
		t.Errorf("expected health status ok, got %v", healthBody["status"])
	}

	// 4. Verify /readyz
	reqReady := httptest.NewRequest("GET", "/readyz", nil)
	wReady := httptest.NewRecorder()
	handler.ServeHTTP(wReady, reqReady)

	if wReady.Code != http.StatusOK {
		t.Fatalf("readyz check failed: %d", wReady.Code)
	}
	var readyBody map[string]interface{}
	if err := json.Unmarshal(wReady.Body.Bytes(), &readyBody); err != nil {
		t.Fatalf("failed to parse ready JSON: %v", err)
	}
	if readyBody["routes_loaded"] != float64(2) {
		t.Errorf("expected 2 routes loaded, got %v", readyBody["routes_loaded"])
	}

	// 5. Verify /version
	reqVer := httptest.NewRequest("GET", "/version", nil)
	wVer := httptest.NewRecorder()
	handler.ServeHTTP(wVer, reqVer)

	if wVer.Code != http.StatusOK {
		t.Fatalf("version check failed: %d", wVer.Code)
	}
	var verBody map[string]interface{}
	if err := json.Unmarshal(wVer.Body.Bytes(), &verBody); err != nil {
		t.Fatalf("failed to parse version JSON: %v", err)
	}
	if verBody["service"] != "agentpay-gateway" {
		t.Errorf("expected service agentpay-gateway, got %v", verBody["service"])
	}

	// 6. Verify matched paid route without auth -> issues 402 challenge
	reqRoute := httptest.NewRequest("GET", "/v1/market-data", nil)
	wRoute := httptest.NewRecorder()
	handler.ServeHTTP(wRoute, reqRoute)

	if wRoute.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required, got %d, body: %s", wRoute.Code, wRoute.Body.String())
	}
	chHeader := wRoute.Header().Get("PAYMENT-REQUIRED")
	if chHeader == "" {
		t.Fatalf("expected PAYMENT-REQUIRED header in 402 response")
	}

	// 7. Verify sending PAYMENT-SIGNATURE -> passes verification and proxies upstream
	validBefore := time.Now().Unix() + 300
	authPayload := fmt.Sprintf(`{"version":"2.0","challengeId":"ch_test","payer":"0x1234","scheme":"EIP-3009","nonce":"0xnonce001","validAfter":0,"validBefore":%d,"signature":"0xsig"}`, validBefore)
	reqPaid := httptest.NewRequest("GET", "/v1/market-data", nil)
	reqPaid.Header.Set("PAYMENT-SIGNATURE", authPayload)
	wPaid := httptest.NewRecorder()
	handler.ServeHTTP(wPaid, reqPaid)

	if wPaid.Code != http.StatusOK {
		t.Fatalf("paid request expected 200 OK, got %d, body: %s", wPaid.Code, wPaid.Body.String())
	}
	var routeBody map[string]interface{}
	if err := json.Unmarshal(wPaid.Body.Bytes(), &routeBody); err != nil {
		t.Fatalf("failed to parse route JSON: %v", err)
	}
	if routeBody["status"] != "proxied" {
		t.Errorf("expected status 'proxied', got %v", routeBody["status"])
	}

	// 8. Verify 404 for unknown route
	req404 := httptest.NewRequest("GET", "/nonexistent/endpoint", nil)
	w404 := httptest.NewRecorder()
	handler.ServeHTTP(w404, req404)

	if w404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent, got status %v", w404.Code)
	}

	// 8. Verify /metrics
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	wMetrics := httptest.NewRecorder()
	handler.ServeHTTP(wMetrics, reqMetrics)

	if wMetrics.Code != http.StatusOK {
		t.Fatalf("metrics endpoint failed: %d", wMetrics.Code)
	}
	metricsStr := wMetrics.Body.String()
	if !strings.Contains(metricsStr, "agentpay_requests_total") {
		t.Errorf("expected agentpay_requests_total in metrics, got:\n%s", metricsStr)
	}
}

func TestConfigVersionTraceabilityAndPolicyEnforcementE2E(t *testing.T) {
	routeID := "rou_expensive"
	policyID := "pol_strict_cap"

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: "2.0.0",
		TenantID:      "ten_trace_e2e",
		VersionID:     "ver_trace_v99",
		VersionNumber: 99,
		SnapshotHash:  "sha256:abcd998877665544332211abcd998877665544332211abcd998877665544332211",
		Routes: []rcu.RouteConfig{
			{
				RouteID:               routeID,
				PathPattern:           "/v1/expensive",
				UpstreamURL:           "http://upstream.mock/v1/expensive",
				PriceBaseUnits:        80000, // 80000 exceeds 50000 limit
				Asset:                 "USDC",
				AllowedPaymentMethods: "both",
				Status:                "active",
			},
			{
				RouteID:               "rou_affordable",
				PathPattern:           "/v1/affordable",
				UpstreamURL:           "http://upstream.mock/v1/affordable",
				PriceBaseUnits:        10000,
				Asset:                 "USDC",
				AllowedPaymentMethods: "both",
				Status:                "active",
			},
		},
		Policies: []rcu.PolicyConfig{
			{
				PolicyID:            policyID,
				RouteID:             &routeID,
				MaxAmountPerRequest: 50000, // max per tx 50000
				Rolling24hBudgetCap: 500000,
				IsActive:            true,
			},
		},
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	var upstreamReceivedVersion, upstreamReceivedHash string
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamReceivedVersion = r.Header.Get("X-AgentPay-Config-Version")
		upstreamReceivedHash = r.Header.Get("X-AgentPay-Config-Hash")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"result":"ok"}`)),
			Request:    r,
		}, nil
	})

	cfg := &config.GatewayConfig{
		Version: "2.0.0",
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Metrics: config.MetricsConfig{
			HealthPath: "/healthz",
			ReadyPath:  "/readyz",
		},
	}
	srv, err := server.NewWithTransport(cfg, rcuMgr, nil, mockTransport)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	handler := srv.Handler()

	// 1. Request to expensive route -> Triggers policy max per tx rejection (403)
	req1 := httptest.NewRequest("GET", "/v1/expensive", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for policy max per tx violation, got %d", w1.Code)
	}
	if w1.Header().Get("X-Config-Version") != "ver_trace_v99" {
		t.Fatalf("expected X-Config-Version: ver_trace_v99, got %s", w1.Header().Get("X-Config-Version"))
	}

	// 2. Request to affordable route with valid payment -> Succeeds and traces version
	validBefore := time.Now().Unix() + 300
	authPayload := fmt.Sprintf(`{"version":"2.0","challengeId":"ch_ok","payer":"0x1234","scheme":"EIP-3009","nonce":"0xnonce002","validAfter":0,"validBefore":%d,"signature":"0xsig"}`, validBefore)
	req2 := httptest.NewRequest("GET", "/v1/affordable", nil)
	req2.Header.Set("PAYMENT-SIGNATURE", authPayload)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", w2.Code, w2.Body.String())
	}
	// Verify downstream traceability header
	if w2.Header().Get("X-Config-Version") != "ver_trace_v99" {
		t.Fatalf("expected downstream X-Config-Version: ver_trace_v99, got %s", w2.Header().Get("X-Config-Version"))
	}
	// Verify upstream injected traceability headers
	if upstreamReceivedVersion != "ver_trace_v99" {
		t.Fatalf("upstream missed X-AgentPay-Config-Version: got %s", upstreamReceivedVersion)
	}
	if upstreamReceivedHash != snap.SnapshotHash {
		t.Fatalf("upstream missed X-AgentPay-Config-Hash: got %s", upstreamReceivedHash)
	}
}

