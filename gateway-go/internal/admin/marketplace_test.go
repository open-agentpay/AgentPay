package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
)

func TestPublicMarketplaceEndpoints(t *testing.T) {
	ctx := context.Background()
	svc := NewMemoryService(nil, nil)
	usageSvc := usage.NewMemoryService()
	svc.SetUsageService(usageSvc)

	ten, owner, err := svc.CreateTenant(ctx, "Solana Alpha Labs", "ten_mkt_01", "owner@solana-alpha.io", "USDC")
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}
	prj, err := svc.CreateProject(ctx, ten.TenantID, "Marketplace Feeds", 500000000, "prj_mkt_01", owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	onchainPrice := int64(1000) // 0.001 USDC
	publicRoute, err := svc.CreateRoute(ctx, &Route{
		TenantID:              ten.TenantID,
		ProjectID:             prj.ProjectID,
		Slug:                  "solana-smart-money-tracker-x402-api",
		Title:                 "Solana Smart Money Tracker X402 API",
		IsPublic:              true,
		Category:              "Crypto & On-Chain Data",
		Tags:                  []string{"Solana", "Smart Money", "x402"},
		DescriptionMD:         "Real-time Solana whale & smart money wallet tracking feed with JSON Schema output.",
		Network:               "eip155:8453",
		PathPattern:           "/v1/solana/smart-money",
		UpstreamURL:           "https://upstream.solana-alpha.io/v1/smart-money",
		PriceBaseUnits:        1000,
		Asset:                 "USDC",
		OnchainPriceBaseUnits: &onchainPrice,
		OnchainAsset:          "USDC",
		PayeeAddress:          "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
		AllowedPaymentMethods: "both",
		RateLimitRPM:          1200,
		TimeoutMs:             15000,
		Status:                RouteStatusActive,
	}, owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create public route: %v", err)
	}

	// Create a private route that must NOT appear in marketplace results
	_, err = svc.CreateRoute(ctx, &Route{
		TenantID:              ten.TenantID,
		ProjectID:             prj.ProjectID,
		Slug:                  "internal-private-admin-rpc",
		Title:                 "Internal Private Admin RPC",
		IsPublic:              false,
		Category:              "Internal",
		PathPattern:           "/v1/internal/rpc",
		UpstreamURL:           "https://internal.solana-alpha.io/rpc",
		PriceBaseUnits:        5000,
		Asset:                 "USDC",
		PayeeAddress:          "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
		AllowedPaymentMethods: "wallet",
		Status:                RouteStatusActive,
	}, owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create private route: %v", err)
	}

	// Record sample request and usage events to verify SLA computation
	for i := 0; i < 10; i++ {
		_ = usageSvc.RecordRequest(ctx, &usage.RequestRecord{
			TenantID:    ten.TenantID,
			ProjectID:   prj.ProjectID,
			RouteID:     publicRoute.RouteID,
			HTTPMethod:  "GET",
			RequestPath: "/v1/solana/smart-money",
			StatusCode:  200,
			DurationMs:  int64(110 + i*5),
			CreatedAt:   time.Now(),
		})
		_ = usageSvc.RecordUsage(ctx, &usage.UsageEvent{
			TenantID:        ten.TenantID,
			ProjectID:       prj.ProjectID,
			RouteID:         publicRoute.RouteID,
			IntentID:        "pi_test",
			AmountBaseUnits: 1000,
			Asset:           "USDC",
			PaymentMethod:   "onchain",
			IsSuccess:       true,
			CreatedAt:       time.Now(),
		})
	}

	handler := NewPublicMarketplaceHandler(svc)

	// 1. Test GET /api/v1/public/marketplace/routes
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace/routes?q=solana", nil)
	recList := httptest.NewRecorder()
	handler.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from list endpoint, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Count  int                     `json:"count"`
		Routes []*MarketplaceRouteItem `json:"routes"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Count != 1 || len(listResp.Routes) != 1 {
		t.Fatalf("expected exactly 1 public route, got %d", listResp.Count)
	}
	item := listResp.Routes[0]
	if item.Slug != "solana-smart-money-tracker-x402-api" {
		t.Errorf("unexpected slug: %s", item.Slug)
	}
	if item.SLA.SuccessRate24h != 100.0 {
		t.Errorf("expected 100.0%% success rate, got %v", item.SLA.SuccessRate24h)
	}
	if item.SLA.PaidCalls24h != 10 {
		t.Errorf("expected 10 paid calls, got %d", item.SLA.PaidCalls24h)
	}
	if item.SLA.HealthStatus != "healthy" {
		t.Errorf("expected healthy status, got %s", item.SLA.HealthStatus)
	}

	// 2. Test GET /api/v1/public/marketplace/routes/:slug
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace/routes/solana-smart-money-tracker-x402-api", nil)
	recDetail := httptest.NewRecorder()
	handler.ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from detail endpoint, got %d: %s", recDetail.Code, recDetail.Body.String())
	}

	var detailResp struct {
		Route *MarketplaceRouteItem `json:"route"`
	}
	if err := json.Unmarshal(recDetail.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("failed to decode detail response: %v", err)
	}
	if detailResp.Route == nil || detailResp.Route.Title != "Solana Smart Money Tracker X402 API" {
		t.Fatalf("unexpected route detail response: %+v", detailResp.Route)
	}

	// 3. Ensure private route returns 404 on detail endpoint
	reqPrivate := httptest.NewRequest(http.MethodGet, "/api/v1/public/marketplace/routes/internal-private-admin-rpc", nil)
	recPrivate := httptest.NewRecorder()
	handler.ServeHTTP(recPrivate, reqPrivate)
	if recPrivate.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for private route slug, got %d", recPrivate.Code)
	}
}
