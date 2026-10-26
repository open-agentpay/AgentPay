package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-agentpay/agentpay-gateway-go/internal/upstreamauth"
)

func TestUpstreamAuthEndpointsAndLifecycle(t *testing.T) {
	svc := NewMemoryService(nil, nil)
	handler := NewHandler(svc)

	tenant, _, err := svc.CreateTenant(context.Background(), "Upstream Auth Org", "ten_upstream_001", "admin@upstream.io")
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// 1. Test GET /api/v1/auth-types
	reqTypes := httptest.NewRequest(http.MethodGet, "/api/v1/auth-types", nil)
	recTypes := httptest.NewRecorder()
	handler.ServeHTTP(recTypes, reqTypes)

	if recTypes.Code != http.StatusOK {
		t.Fatalf("expected 200 from /auth-types, got %d: %s", recTypes.Code, recTypes.Body.String())
	}
	var typesResp struct {
		Types []upstreamauth.AuthTypeSchema `json:"types"`
	}
	if err := json.Unmarshal(recTypes.Body.Bytes(), &typesResp); err != nil {
		t.Fatalf("failed to decode auth-types: %v", err)
	}
	if len(typesResp.Types) < 7 {
		t.Errorf("expected at least 7 auth schemas, got %d", len(typesResp.Types))
	}

	// 2. Test POST /api/v1/tenants/:id/routes with UpstreamAuth
	routePayload := map[string]any{
		"path_pattern":            "/v1/models/deepseek",
		"upstream_url":            "https://api.deepseek.com/v1/chat/completions",
		"price_base_units":        20000,
		"allowed_payment_methods": "wallet",
		"upstream_auth": map[string]any{
			"type": "api_key",
			"params": map[string]any{
				"in":     "header",
				"name":   "Authorization",
				"prefix": "Bearer ",
			},
			"secrets": map[string]string{
				"value": "sk-live-deepseek-123456",
			},
		},
	}
	bodyBytes, _ := json.Marshal(routePayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/"+tenant.TenantID+"/routes", bytes.NewReader(bodyBytes))
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	handler.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 created route, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createResp struct {
		Route *Route `json:"route"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode created route: %v", err)
	}
	createdRoute := createResp.Route
	if createdRoute.UpstreamAuth == nil {
		t.Fatal("expected UpstreamAuth to be populated")
	}
	if createdRoute.UpstreamAuth.Type != "api_key" {
		t.Errorf("expected type api_key, got %s", createdRoute.UpstreamAuth.Type)
	}
	if !createdRoute.UpstreamAuth.HasSecret {
		t.Error("expected HasSecret to be true")
	}
	// Secrets must NEVER be echoed back in plain text!
	if len(createdRoute.UpstreamAuth.Secrets) != 0 {
		t.Errorf("expected Secrets to be empty (write-only), got %v", createdRoute.UpstreamAuth.Secrets)
	}

	// 3. Test PATCH without updating secrets preserves existing encrypted secret
	updatePayload := map[string]any{
		"price_base_units": 25000,
		"upstream_auth": map[string]any{
			"type": "api_key",
			"params": map[string]any{
				"in":     "header",
				"name":   "Authorization",
				"prefix": "Bearer ",
			},
			// no secrets provided
		},
	}
	updateBytes, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/"+tenant.TenantID+"/routes/"+createdRoute.RouteID, bytes.NewReader(updateBytes))
	reqUpdate.Header.Set("Content-Type", "application/json")
	recUpdate := httptest.NewRecorder()
	handler.ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 updated route, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}
	var updateResp struct {
		Route *Route `json:"route"`
	}
	_ = json.Unmarshal(recUpdate.Body.Bytes(), &updateResp)
	if !updateResp.Route.UpstreamAuth.HasSecret {
		t.Error("expected HasSecret to remain true after partial update")
	}

	// 4. Test POST /api/v1/tenants/:id/routes/test-upstream
	testPayload := map[string]any{
		"upstream_url": "https://httpbin.org/get",
		"method":       "GET",
		"upstream_auth": map[string]any{
			"type": "bearer",
			"secrets": map[string]string{
				"token": "test-token-xyz-12345678",
			},
		},
	}
	testBytes, _ := json.Marshal(testPayload)
	reqTest := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/"+tenant.TenantID+"/routes/test-upstream", bytes.NewReader(testBytes))
	reqTest.Header.Set("Content-Type", "application/json")
	recTest := httptest.NewRecorder()
	handler.ServeHTTP(recTest, reqTest)

	if recTest.Code != http.StatusOK {
		t.Fatalf("expected 200 from test-upstream, got %d: %s", recTest.Code, recTest.Body.String())
	}

	var testResp struct {
		Result *TestUpstreamResult `json:"result"`
	}
	if err := json.Unmarshal(recTest.Body.Bytes(), &testResp); err != nil {
		t.Fatalf("failed to decode test result: %v", err)
	}

	res := testResp.Result
	if res == nil {
		t.Fatal("expected test result to be non-nil")
	}
	// Verify sentHeaders is present and masked
	authHeader, hasAuth := res.SentHeaders["Authorization"]
	if !hasAuth {
		t.Error("expected Authorization in sentHeaders")
	}
	if authHeader != "" && !bytes.Contains([]byte(authHeader), []byte("●●●●●●")) {
		t.Errorf("expected masked Authorization header, got: %s", authHeader)
	}
}
