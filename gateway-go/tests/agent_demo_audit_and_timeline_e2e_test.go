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

	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/payment"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
	"github.com/open-agentpay/agentpay-gateway-go/internal/timeline"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

func TestAgentDemoAuditAndTimelineE2E(t *testing.T) {
	configPath := filepath.Join("..", "configs", "gateway.example.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load example config: %v", err)
	}

	routes := []rcu.RouteConfig{
		{
			RouteID:               "route_demo_api",
			PathPattern:           "/v1/demo-data",
			UpstreamURL:           "http://mock-upstream/v1/demo-data",
			PriceBaseUnits:        50000, // 0.05 USDC
			Asset:                 "USDC",
			PayeeAddress:          "0x1111111111111111111111111111111111111111",
			AllowedPaymentMethods: "both",
			Status:                "active",
		},
	}

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_demo_org",
		VersionID:     "ver_demo_test_1",
		Routes:        routes,
	}

	rcuMgr, err := rcu.NewManager(snap)
	if err != nil {
		t.Fatalf("failed to init rcu: %v", err)
	}

	walletSvc := wallet.NewMemoryService()

	mockTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"ticker":"NVDA","price":135.50,"status":"fulfilled"}`
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

	adminSvc := gwServer.AdminService()
	handler := gwServer.Handler()

	// 1. Create Tenant and Member via Admin Control Plane
	tenant, owner, err := adminSvc.Register(context.Background(), "Demo Organization", "demo-owner@example.com", "Password123!")
	if err != nil {
		t.Fatalf("failed to register tenant: %v", err)
	}

	jwtMgr := auth.MustNewJWTManager(cfg.Auth.JWTSecret, 24*time.Hour)
	adminToken, err := jwtMgr.GenerateToken(owner.MemberID, owner.Email, tenant.TenantID, string(owner.Role))
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	adminAuthHeader := fmt.Sprintf("Bearer %s", adminToken)

	projects, err := adminSvc.ListProjects(context.Background(), tenant.TenantID)
	if err != nil || len(projects) == 0 {
		t.Fatalf("failed to retrieve tenant projects: %v", err)
	}
	projectID := projects[0].ProjectID

	// 2. Create demo Agent via Admin Service
	demoAgent, err := adminSvc.CreateAgent(context.Background(), tenant.TenantID, projectID, "DemoMarketAgent", map[string]interface{}{
		"max_per_request": 100000,
	}, owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// 3. Generate API Key for demo Agent
	keyResult, err := adminSvc.CreateAPIKey(context.Background(), tenant.TenantID, projectID, demoAgent.AgentID, "demo-agent-key", []string{"gateway:call"}, nil, owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create agent api key: %v", err)
	}
	rawAPIKey := keyResult.RawKey

	// 4. Ensure Tenant Wallet has funds
	walletSvc.RegisterWallet(&wallet.Wallet{
		WalletID:       "wlt_" + tenant.TenantID,
		TenantID:       tenant.TenantID,
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		Version:        1,
		AvailableCache: 10_000_000, // 10 USDC
		HeldCache:      0,
	})

	// 5. Send Gateway request using the newly generated API Key
	req := httptest.NewRequest("GET", "/v1/demo-data", nil)
	req.Header.Set("Authorization", "Bearer "+rawAPIKey)
	req.Header.Set("X-Request-ID", "req_demo_e2e_001")
	req.Header.Set("X-Session-ID", "sess_demo_alpha")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from gateway, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Verify virtual wallet deducted 50,000 base units (from 10,000,000 to 9,950,000)
	wAcc, err := walletSvc.GetWalletByAPIKey(context.Background(), rawAPIKey)
	if err != nil {
		t.Fatalf("failed to get wallet: %v", err)
	}
	if wAcc.AvailableBalance != 9_950_000 {
		t.Errorf("expected wallet available balance 9950000, got %d", wAcc.AvailableBalance)
	}

	// 7. Verify Payment Audit (T5.3): GET /api/v1/tenants/:tenantId/payments
	reqPayments := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tenants/%s/payments", tenant.TenantID), nil)
	reqPayments.Header.Set("Authorization", adminAuthHeader)
	recPayments := httptest.NewRecorder()
	handler.ServeHTTP(recPayments, reqPayments)

	if recPayments.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from payments list API, got %d: %s", recPayments.Code, recPayments.Body.String())
	}

	var paymentsResp struct {
		Payments []*payment.PaymentAuditRecord `json:"payments"`
		Total    int64                         `json:"total"`
	}
	if err := json.Unmarshal(recPayments.Body.Bytes(), &paymentsResp); err != nil {
		t.Fatalf("failed to parse payments response: %v", err)
	}

	if paymentsResp.Total != 1 || len(paymentsResp.Payments) != 1 {
		t.Fatalf("expected exactly 1 payment record in audit, got total=%d, len=%d", paymentsResp.Total, len(paymentsResp.Payments))
	}

	pRec := paymentsResp.Payments[0]
	if pRec.AgentID != demoAgent.AgentID {
		t.Errorf("expected PaymentAudit.AgentID to be %s, got %s", demoAgent.AgentID, pRec.AgentID)
	}
	if pRec.DisplayStatus != payment.DisplayCompleted {
		t.Errorf("expected PaymentAudit.DisplayStatus to be %s, got %s", payment.DisplayCompleted, pRec.DisplayStatus)
	}

	// 8. Verify Payment Audit with agent_id query filter: GET /api/v1/tenants/:tenantId/payments?agent_id=...
	reqAgentPayments := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tenants/%s/payments?agent_id=%s", tenant.TenantID, demoAgent.AgentID), nil)
	reqAgentPayments.Header.Set("Authorization", adminAuthHeader)
	recAgentPayments := httptest.NewRecorder()
	handler.ServeHTTP(recAgentPayments, reqAgentPayments)

	if recAgentPayments.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from payments list API with agent filter, got %d", recAgentPayments.Code)
	}

	var agentPaymentsResp struct {
		Payments []*payment.PaymentAuditRecord `json:"payments"`
		Total    int64                         `json:"total"`
	}
	if err := json.Unmarshal(recAgentPayments.Body.Bytes(), &agentPaymentsResp); err != nil {
		t.Fatalf("failed to parse agent payments response: %v", err)
	}
	if agentPaymentsResp.Total != 1 {
		t.Errorf("expected 1 payment record when filtering by demo agent ID, got %d", agentPaymentsResp.Total)
	}

	// 9. Verify Agent Behavior Timeline (T5.8): GET /api/v1/tenants/:tenantId/agents/:agentId/timeline
	reqTimeline := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tenants/%s/agents/%s/timeline", tenant.TenantID, demoAgent.AgentID), nil)
	reqTimeline.Header.Set("Authorization", adminAuthHeader)
	recTimeline := httptest.NewRecorder()
	handler.ServeHTTP(recTimeline, reqTimeline)

	if recTimeline.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from timeline API, got %d: %s", recTimeline.Code, recTimeline.Body.String())
	}

	var timelineResp struct {
		Items []*timeline.TimelineEvent `json:"items"`
		Total int64                     `json:"total"`
	}
	if err := json.Unmarshal(recTimeline.Body.Bytes(), &timelineResp); err != nil {
		t.Fatalf("failed to parse timeline response: %v", err)
	}

	if timelineResp.Total == 0 || len(timelineResp.Items) == 0 {
		t.Fatalf("expected timeline events for demo agent, got 0")
	}

	foundSpend := false
	foundRequest := false
	for _, item := range timelineResp.Items {
		if item.Observed.AgentID != demoAgent.AgentID {
			t.Errorf("timeline event agentID mismatch: expected %s, got %s", demoAgent.AgentID, item.Observed.AgentID)
		}
		if item.EventType == "agent.spend" {
			foundSpend = true
		}
		if item.EventType == "agent.request" {
			foundRequest = true
		}
	}

	if !foundSpend {
		t.Errorf("expected agent.spend event in timeline, but was not found")
	}
	if !foundRequest {
		t.Errorf("expected agent.request event in timeline, but was not found")
	}
}
