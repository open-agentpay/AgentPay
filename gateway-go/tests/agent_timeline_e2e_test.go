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
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/agent"
	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
	"github.com/open-agentpay/agentpay-gateway-go/internal/timeline"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

func TestAgentTimelineE2E(t *testing.T) {
	configPath := filepath.Join("..", "configs", "gateway.example.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load example config: %v", err)
	}

	routes := []rcu.RouteConfig{
		{
			RouteID:               "route_agent_query",
			PathPattern:           "/v1/agent-query",
			UpstreamURL:           "http://mock-upstream/v1/agent-query",
			PriceBaseUnits:        10000, // 0.01 USDC
			Asset:                 "USDC",
			PayeeAddress:          "0x1111111111111111111111111111111111111111",
			AllowedPaymentMethods: "both",
			Status:                "active",
		},
	}

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_timeline_alpha",
		VersionID:     "ver_timeline_test_1",
		Routes:        routes,
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	walletSvc := wallet.NewMemoryService()
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_alpha",
		TenantID:       "ten_timeline_alpha",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 1_000_000,
		HeldCache:      0,
	})
	walletSvc.RegisterAPIKey("apk_live_alpha_agent", "ten_timeline_alpha", "prj_alpha")

	mockTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"answer":"success"}`
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
		t.Fatalf("failed to create gateway server: %v", err)
	}

	adminSvc := gwServer.AdminService()
	timelineSvc := gwServer.TimelineService()

	// 1. Setup Tenant Alpha and Agent Alpha
	tenantAlpha, ownerAlpha, err := adminSvc.Register(context.Background(), "Tenant Alpha", "owner-alpha@timeline.io", "Password123!")
	if err != nil {
		t.Fatalf("failed to register tenant alpha: %v", err)
	}
	jwtMgr := auth.NewJWTManager("", 24*time.Hour)
	tokenAlpha, _ := jwtMgr.GenerateToken(ownerAlpha.MemberID, ownerAlpha.Email, tenantAlpha.TenantID, string(ownerAlpha.Role))

	projects, err := adminSvc.ListProjects(context.Background(), tenantAlpha.TenantID)
	if err != nil || len(projects) == 0 {
		t.Fatalf("failed to list projects for tenant alpha: %v", err)
	}
	projectID := projects[0].ProjectID

	agentAlpha, err := adminSvc.CreateAgent(context.Background(), tenantAlpha.TenantID, projectID, "MarketAnalyst", map[string]interface{}{
		"max_per_request": 50000,
	}, ownerAlpha.MemberID)
	if err != nil {
		t.Fatalf("failed to create agent alpha: %v", err)
	}

	agIdent := &agent.AgentIdentity{
		AgentID:   agentAlpha.AgentID,
		TenantID:  tenantAlpha.TenantID,
		ProjectID: projectID,
		Name:      "MarketAnalyst",
	}

	// 2. Setup Tenant Beta (for isolation testing)
	tenantBeta, ownerBeta, err := adminSvc.Register(context.Background(), "Tenant Beta", "owner-beta@timeline.io", "Password123!")
	if err != nil {
		t.Fatalf("failed to register tenant beta: %v", err)
	}
	tokenBeta, _ := jwtMgr.GenerateToken(ownerBeta.MemberID, ownerBeta.Email, tenantBeta.TenantID, string(ownerBeta.Role))

	// 3. Emit test events simulating agent activity per T1.8 specification
	// Session 1: Market Research (Request + Spend)
	e1 := agent.BuildRequestEvent(
		agIdent,
		agent.CorrelationContext{RequestID: "req_session1_01", IdempotencyKey: "idem_s1_01"},
		"192.168.1.10",
		"AgentPay-Client/1.0",
		"route_agent_query",
		200,
		120,
		"hash123",
		&agent.DeclaredContext{SessionID: "session_market_001", TaskID: "task_ticker_nvda", ToolName: "fetch_quote"},
	)
	_ = timelineSvc.RecordEvent(context.Background(), e1)

	e2 := agent.BuildSpendEvent(
		agIdent,
		agent.CorrelationContext{
			RequestID:      "req_session1_01",
			IdempotencyKey: "idem_s1_01",
			IntentID:       "int_s1_01",
			HoldID:         "hld_s1_01",
			SettlementID:   "set_s1_01",
			FulfillmentID:  "ful_s1_01",
		},
		"192.168.1.10",
		"AgentPay-Client/1.0",
		10000,
		"USDC",
		"wallet",
		"capture",
		&agent.DeclaredContext{SessionID: "session_market_001", TaskID: "task_ticker_nvda", ToolName: "fetch_quote"},
	)
	_ = timelineSvc.RecordEvent(context.Background(), e2)

	// Session 2: Portfolio Rebalance (Policy Decision)
	e3 := agent.BuildPolicyDecisionEvent(
		agIdent,
		agent.PolicyDecision{Allowed: true, ReasonCode: "POLICY_ALLOW", Reason: "Under single tx limit"},
		"route_agent_query",
		agent.CorrelationContext{RequestID: "req_session2_01", IdempotencyKey: "idem_s2_01"},
		"192.168.1.10",
		"AgentPay-Client/1.0",
		&agent.DeclaredContext{SessionID: "session_rebalance_002", TaskID: "task_rebalance_execute", ToolName: "rebalance_portfolio"},
	)
	_ = timelineSvc.RecordEvent(context.Background(), e3)

	// Event without declared headers (T1.8 Graceful Handling - defaults without error)
	e4 := &agent.AuditEvent{
		EventID:     agent.GenerateEventID(),
		EventType:   agent.EventTypeAgentControl,
		Version:     "1.0",
		Timestamp:   agent.FormatTimestamp(time.Now()),
		TenantID:    tenantAlpha.TenantID,
		ProjectID:   "prj_alpha",
		AgentID:     agentAlpha.AgentID,
		Correlation: agent.CorrelationContext{RequestID: "req_ctrl_01"},
		Actor: agent.ActorContext{
			Type:      "operator",
			ID:        ownerAlpha.MemberID,
			ClientIP:  "10.0.0.1",
			UserAgent: "Console-Web/1.0",
		},
		Declared: nil,
		Data: map[string]interface{}{
			"action":         "agent.resumed",
			"control_reason": "Routine maintenance completed",
		},
	}
	_ = timelineSvc.RecordEvent(context.Background(), e4)

	// 4. Test REST API: Query all timeline events for Agent Alpha
	t.Run("Query All Timeline Events", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tenants/%s/agents/%s/timeline", tenantAlpha.TenantID, agentAlpha.AgentID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenAlpha)
		w := httptest.NewRecorder()
		gwServer.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp timeline.TimelineResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 4 {
			t.Errorf("expected total 4, got %d", resp.Total)
		}
		if len(resp.Items) != 4 {
			t.Errorf("expected 4 items, got %d", len(resp.Items))
		}

		// Verify distinct sessions aggregated
		if len(resp.Sessions) != 2 {
			t.Errorf("expected 2 distinct sessions, got %d: %v", len(resp.Sessions), resp.Sessions)
		}

		// Verify observed facts vs declared metadata integrity
		foundSpend := false
		for _, item := range resp.Items {
			if item.EventType == agent.EventTypeAgentSpend {
				foundSpend = true
				if item.Observed.CostBaseUnits != 10000 {
					t.Errorf("expected observed cost 10000, got: %+v", item.Observed)
				}
				if item.Declared == nil || item.Declared.SessionID != "session_market_001" {
					t.Errorf("expected declared session_market_001, got: %+v", item.Declared)
				}
				if item.Summary == "" {
					t.Errorf("expected non-empty summary for spend event")
				}
			}
		}
		if !foundSpend {
			t.Errorf("agent.spend event not found in timeline items")
		}
	})

	// 5. Test REST API: Session Filtering
	t.Run("Filter by Session ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tenants/%s/agents/%s/timeline?session_id=session_market_001", tenantAlpha.TenantID, agentAlpha.AgentID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenAlpha)
		w := httptest.NewRecorder()
		gwServer.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp timeline.TimelineResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected 2 events in session_market_001, got %d", resp.Total)
		}
		for _, item := range resp.Items {
			if item.Declared == nil || item.Declared.SessionID != "session_market_001" {
				t.Errorf("expected session_market_001, got: %+v", item.Declared)
			}
		}
	})

	// 6. Test REST API: Event Type Filtering
	t.Run("Filter by Event Type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tenants/%s/agents/%s/timeline?event_type=%s", tenantAlpha.TenantID, agentAlpha.AgentID, agent.EventTypeAgentPolicyDecision), nil)
		req.Header.Set("Authorization", "Bearer "+tokenAlpha)
		w := httptest.NewRecorder()
		gwServer.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp timeline.TimelineResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 1 {
			t.Errorf("expected 1 policy decision event, got %d", resp.Total)
		}
		if len(resp.Items) > 0 && resp.Items[0].EventType != agent.EventTypeAgentPolicyDecision {
			t.Errorf("expected agent.policy_decision, got %s", resp.Items[0].EventType)
		}
	})

	// 7. Test Flat Endpoint Compatibility (/api/v1/agents/:agent_id/timeline)
	t.Run("Flat Endpoint Compatibility", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/agents/%s/timeline", agentAlpha.AgentID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenAlpha)
		w := httptest.NewRecorder()
		gwServer.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp timeline.TimelineResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Total != 4 {
			t.Errorf("expected 4 events via flat endpoint, got %d", resp.Total)
		}
	})

	// 8. Test Multi-tenant Isolation (Tenant Beta cannot view Tenant Alpha's agent timeline)
	t.Run("Multi-tenant Isolation Gate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tenants/%s/agents/%s/timeline", tenantBeta.TenantID, agentAlpha.AgentID), nil)
		req.Header.Set("Authorization", "Bearer "+tokenBeta)
		w := httptest.NewRecorder()
		gwServer.Handler().ServeHTTP(w, req)

		// Must reject with 404 Not Found (or 403 Forbidden)
		if w.Code == http.StatusOK {
			t.Errorf("expected authorization failure for cross-tenant timeline query, but got 200 OK")
		}
	})
}
