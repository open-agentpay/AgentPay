package tests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/agent"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/proxy"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/x402"
)

func TestAgentE2ESpoofedHeaderStrippingAndIdentityResolution(t *testing.T) {
	var receivedAgentID, receivedSessionID string
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		receivedAgentID = r.Header.Get("X-Agent-ID")
		receivedSessionID = r.Header.Get("X-Session-ID")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"upstream":"ok"}`)),
			Request:    r,
		}, nil
	})

	// Setup Gateway components
	m := metrics.New()
	proxyHandler := proxy.NewHandler(m, mockTransport)
	walletService := x402.NewMemoryWalletService()
	processor := x402.NewPaymentProcessor(nil, nil, walletService, proxyHandler, m)

	// Setup virtual wallet
	walletService.RegisterWallet("apk_live_agent_key_001", &x402.WalletAccount{
		WalletID:         "wal_agent_01",
		TenantID:         "ten_corp_01",
		Status:           "active",
		AvailableBalance: 100000,
	})

	// Register Agent in Registry
	agentID := "agt_autonomous_001"
	processor.GetAgentRegistry().RegisterAgent(&agent.AgentIdentity{
		AgentID:   agentID,
		TenantID:  "ten_corp_01",
		ProjectID: "prj_prod",
		Name:      "TradingBot",
		Status:    agent.AgentStatusActive,
	}, nil)
	processor.GetAgentRegistry().BindAPIKey("apk_live_agent_key_001", agentID)

	route := &rcu.RouteConfig{
		RouteID:               "rou_market",
		PathPattern:           "/api/v1/market",
		UpstreamURL:           "http://mock-upstream/api/v1/market",
		PriceBaseUnits:        100,
		Asset:                 "USDC",
		AllowedPaymentMethods: "wallet",
	}


	// 1. Client attempts to spoof X-Agent-ID and inject malicious characters into session_id
	req := httptest.NewRequest("POST", "/api/v1/market", strings.NewReader(`{"action":"buy"}`))
	req.Header.Set("Authorization", "Bearer apk_live_agent_key_001")
	req.Header.Set("X-Agent-ID", "spoofed_evil_agent_id_666")
	req.Header.Set("X-Session-ID", "sess_valid_part\r\nInjectedHeader: evil<script>")

	rec := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec, req, route, nil, nil, "req_e2e_01")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Invariant 1: Upstream must see the verified agent_id, NOT the spoofed one
	if receivedAgentID != "agt_autonomous_001" {
		t.Fatalf("spoofed header bypass: expected agt_autonomous_001, got %s", receivedAgentID)
	}

	// Invariant 2: Session ID must be sanitized without CRLF or HTML injection tags
	if strings.Contains(receivedSessionID, "\r") || strings.Contains(receivedSessionID, "\n") ||
		strings.Contains(receivedSessionID, "<") || strings.Contains(receivedSessionID, ">") {
		t.Fatalf("session_id not sanitized: %s", receivedSessionID)
	}
}

func TestAgentE2EKillSwitchFastPath(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
			Request:    r,
		}, nil
	})

	m := metrics.New()
	proxyHandler := proxy.NewHandler(m, mockTransport)
	walletService := x402.NewMemoryWalletService()
	processor := x402.NewPaymentProcessor(nil, nil, walletService, proxyHandler, m)

	walletService.RegisterWallet("apk_live_kill_test", &x402.WalletAccount{
		WalletID:         "wal_test_01",
		TenantID:         "ten_01",
		Status:           "active",
		AvailableBalance: 100000,
	})

	agentID := "agt_suspend_target"
	processor.GetAgentRegistry().RegisterAgent(&agent.AgentIdentity{
		AgentID:   agentID,
		TenantID:  "ten_01",
		ProjectID: "prj_01",
		Status:    agent.AgentStatusActive,
	}, nil)
	processor.GetAgentRegistry().BindAPIKey("apk_live_kill_test", agentID)

	route := &rcu.RouteConfig{
		RouteID:               "rou_order",
		PathPattern:           "/api/v1/order",
		UpstreamURL:           "http://mock-upstream/api/v1/order",
		PriceBaseUnits:        50,
		Asset:                 "USDC",
		AllowedPaymentMethods: "wallet",
	}

	// Call 1: Active agent succeeds
	req1 := httptest.NewRequest("POST", "/api/v1/order", strings.NewReader(`{}`))
	req1.Header.Set("Authorization", "Bearer apk_live_kill_test")
	rec1 := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec1, req1, route, nil, nil, "req_1")
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK before suspension, got %d", rec1.Code)
	}

	// Administrator activates Kill Switch
	_ = processor.GetAgentRegistry().SetAgentStatus(agentID, agent.AgentStatusSuspended)

	// Call 2: Suspended agent must be blocked in fast path with 403 ERR_AGENT_SUSPENDED
	req2 := httptest.NewRequest("POST", "/api/v1/order", strings.NewReader(`{}`))
	req2.Header.Set("Authorization", "Bearer apk_live_kill_test")
	rec2 := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec2, req2, route, nil, nil, "req_2")

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden after suspension, got %d", rec2.Code)
	}
	body := rec2.Body.String()
	if !strings.Contains(body, "ERR_AGENT_SUSPENDED") {
		t.Fatalf("expected ERR_AGENT_SUSPENDED, got %s", body)
	}

	// Verify audit emitter recorded the policy denial
	emitter, ok := processor.GetAuditEmitter().(*agent.MemoryAuditEmitter)
	if !ok {
		t.Fatalf("expected MemoryAuditEmitter")
	}
	policyEvents := emitter.EventsByType(agent.EventTypeAgentPolicyDecision)
	foundDenial := false
	for _, pe := range policyEvents {
		if pe.AgentID == agentID && pe.Data["action"] == "deny" {
			foundDenial = true
			if pe.Data["reason_code"] != "ERR_AGENT_SUSPENDED" {
				t.Fatalf("expected reason_code ERR_AGENT_SUSPENDED, got %v", pe.Data["reason_code"])
			}
		}
	}
	if !foundDenial {
		t.Fatalf("audit event for agent suspension policy denial was not emitted")
	}
}

func TestAgentE2EPolicyEnforcementAndAuditStream(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"upstream":"success"}`)),
			Request:    r,
		}, nil
	})

	m := metrics.New()
	proxyHandler := proxy.NewHandler(m, mockTransport)
	walletService := x402.NewMemoryWalletService()
	processor := x402.NewPaymentProcessor(nil, nil, walletService, proxyHandler, m)

	walletService.RegisterWallet("apk_live_policy_agent", &x402.WalletAccount{
		WalletID:         "wal_policy_01",
		TenantID:         "ten_corp",
		Status:           "active",
		AvailableBalance: 100000,
	})

	agentID := "agt_policy_enforced"
	policy := &agent.AgentPolicyConfig{
		AgentID:             agentID,
		MaxAmountPerRequest: 200,                  // Single tx max = 200
		RouteAllowlist:      []string{"rou_whitelisted"}, // Only rou_whitelisted is permitted
	}

	processor.GetAgentRegistry().RegisterAgent(&agent.AgentIdentity{
		AgentID:   agentID,
		TenantID:  "ten_corp",
		ProjectID: "prj_ml",
		Status:    agent.AgentStatusActive,
	}, policy)
	processor.GetAgentRegistry().BindAPIKey("apk_live_policy_agent", agentID)

	// Route 1: Not whitelisted
	routeUnauthorized := &rcu.RouteConfig{
		RouteID:               "rou_unauthorized",
		PathPattern:           "/api/v1/private",
		UpstreamURL:           "http://mock-upstream/api/v1/private",
		PriceBaseUnits:        100,
		Asset:                 "USDC",
		AllowedPaymentMethods: "wallet",
	}

	// Route 2: Whitelisted, price exceeds limit (price = 300 > 200)
	routeOverpriced := &rcu.RouteConfig{
		RouteID:               "rou_whitelisted",
		PathPattern:           "/api/v1/compute/heavy",
		UpstreamURL:           "http://mock-upstream/api/v1/compute/heavy",
		PriceBaseUnits:        300,
		Asset:                 "USDC",
		AllowedPaymentMethods: "wallet",
	}

	// Route 3: Whitelisted and compliant (price = 150 <= 200)
	routeCompliant := &rcu.RouteConfig{
		RouteID:               "rou_whitelisted",
		PathPattern:           "/api/v1/compute/light",
		UpstreamURL:           "http://mock-upstream/api/v1/compute/light",
		PriceBaseUnits:        150,
		Asset:                 "USDC",
		AllowedPaymentMethods: "wallet",
	}


	// Test 1: Route Allowlist Rejection
	req1 := httptest.NewRequest("POST", "/api/v1/private", strings.NewReader(`{}`))
	req1.Header.Set("Authorization", "Bearer apk_live_policy_agent")
	rec1 := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec1, req1, routeUnauthorized, nil, nil, "req_unauth")

	if rec1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized route, got %d", rec1.Code)
	}
	if !strings.Contains(rec1.Body.String(), "ERR_ROUTE_NOT_ALLOWED") {
		t.Fatalf("expected ERR_ROUTE_NOT_ALLOWED, got %s", rec1.Body.String())
	}

	// Test 2: Max Per Tx Rejection
	req2 := httptest.NewRequest("POST", "/api/v1/compute/heavy", strings.NewReader(`{}`))
	req2.Header.Set("Authorization", "Bearer apk_live_policy_agent")
	rec2 := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec2, req2, routeOverpriced, nil, nil, "req_overpriced")

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for overpriced route, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "ERR_POLICY_MAX_PER_TX") {
		t.Fatalf("expected ERR_POLICY_MAX_PER_TX, got %s", rec2.Body.String())
	}

	// Test 3: Compliant execution succeeds
	req3 := httptest.NewRequest("POST", "/api/v1/compute/light", strings.NewReader(`{}`))
	req3.Header.Set("Authorization", "Bearer apk_live_policy_agent")
	req3.Header.Set("X-Task-ID", "task_inference_42")
	rec3 := httptest.NewRecorder()
	processor.HandleRequestWithSnapshot(rec3, req3, routeCompliant, nil, nil, "req_compliant")

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for compliant call, got %d: %s", rec3.Code, rec3.Body.String())
	}

	// Test 4: Verify complete audit stream (policy_decision, spend, request)
	emitter := processor.GetAuditEmitter().(*agent.MemoryAuditEmitter)
	spendEvents := emitter.EventsByType(agent.EventTypeAgentSpend)
	if len(spendEvents) == 0 {
		t.Fatalf("expected agent.spend events to be emitted")
	}
	lastSpend := spendEvents[len(spendEvents)-1]
	if lastSpend.Data["action"] != "capture" || lastSpend.Data["amount_base_units"] != int64(150) {
		t.Fatalf("invalid spend audit payload: %+v", lastSpend.Data)
	}

	reqEvents := emitter.EventsByType(agent.EventTypeAgentRequest)
	if len(reqEvents) == 0 {
		t.Fatalf("expected agent.request events to be emitted")
	}
	lastReq := reqEvents[len(reqEvents)-1]
	if lastReq.Data["http_status"] != http.StatusOK {
		t.Fatalf("expected 200 status in request audit, got %v", lastReq.Data["http_status"])
	}
	if lastReq.Declared == nil || lastReq.Declared.TaskID != "task_inference_42" {
		t.Fatalf("expected declared task_id task_inference_42, got %+v", lastReq.Declared)
	}
}

// Compile-time check
var _ = io.EOF
var _ = context.Background
var _ = time.Second
