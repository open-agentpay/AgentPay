package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
)

func setupRouteTest(t *testing.T) (*MemoryService, http.Handler, *auth.JWTManager, string, string, string, string) {
	t.Helper()
	svc, _, _ := setupTestService()
	jwtMgr := auth.NewJWTManager("test-secret-key-32-bytes-long!!", 2*time.Hour)
	handler := auth.Middleware(jwtMgr)(NewHandlerWithJWT(svc, jwtMgr))

	ctx := context.Background()
	// Register tenant
	tenant, owner, err := svc.CreateTenant(ctx, "Route Org", "owner@route.io", "Password123!")
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Create project
	prj, err := svc.CreateProject(ctx, tenant.TenantID, "LLM Services", 1000000, "", owner.MemberID)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Add viewer member
	viewer, err := svc.AddMember(ctx, tenant.TenantID, "viewer@route.io", RoleViewer, owner.MemberID)
	if err != nil {
		t.Fatalf("failed to add viewer: %v", err)
	}

	ownerToken, _ := jwtMgr.GenerateToken(owner.MemberID, tenant.TenantID, string(owner.Role), owner.Email)
	viewerToken, _ := jwtMgr.GenerateToken(viewer.MemberID, tenant.TenantID, string(viewer.Role), viewer.Email)

	return svc, handler, jwtMgr, tenant.TenantID, prj.ProjectID, ownerToken, viewerToken
}

func TestRouteCRUDAndRBAC(t *testing.T) {
	_, handler, _, tenantID, projectID, ownerToken, viewerToken := setupRouteTest(t)

	// 1. Viewer tries to create route -> Should be Forbidden (403)
	createPayload := map[string]interface{}{
		"project_id":              projectID,
		"path_pattern":            "/api/v1/chat/completions",
		"upstream_url":            "https://api.openai.com/v1/chat/completions",
		"price_base_units":        50000,
		"payee_address":           "0x1234567890123456789012345678901234567890",
		"allowed_payment_methods": "both",
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/"+tenantID+"/routes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer creating route, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Owner creates route -> 201 Created with defaults
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants/"+tenantID+"/routes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for owner creating route, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Route *Route `json:"route"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Route == nil {
		t.Fatalf("expected route in response")
	}
	routeID := resp.Route.RouteID
	if resp.Route.TimeoutMs != 30000 {
		t.Errorf("expected default timeout 30000, got %d", resp.Route.TimeoutMs)
	}
	if resp.Route.Status != RouteStatusActive {
		t.Errorf("expected default status active, got %s", resp.Route.Status)
	}
	if resp.Route.Asset != "USDC" {
		t.Errorf("expected default asset USDC, got %s", resp.Route.Asset)
	}

	// 3. Duplicate route path pattern creation -> 409 Conflict
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants/"+tenantID+"/routes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate route path, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Update route (Partial PATCH): toggle status to disabled and increase price
	newPrice := int64(75000)
	disabledStatus := RouteStatusDisabled
	newTimeout := 45000
	updatePayload := map[string]interface{}{
		"price_base_units": newPrice,
		"status":           disabledStatus,
		"timeout_ms":       newTimeout,
	}
	upBody, _ := json.Marshal(updatePayload)

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/"+tenantID+"/routes/"+routeID, bytes.NewReader(upBody))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for updating route, got %d: %s", w.Code, w.Body.String())
	}

	var updateResp struct {
		Route *Route `json:"route"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &updateResp)
	if updateResp.Route.Status != RouteStatusDisabled {
		t.Errorf("expected status disabled, got %s", updateResp.Route.Status)
	}
	if updateResp.Route.PriceBaseUnits != newPrice {
		t.Errorf("expected price %d, got %d", newPrice, updateResp.Route.PriceBaseUnits)
	}
	if updateResp.Route.TimeoutMs != newTimeout {
		t.Errorf("expected timeout %d, got %d", newTimeout, updateResp.Route.TimeoutMs)
	}
	// Verify path was not wiped out
	if updateResp.Route.PathPattern != "/api/v1/chat/completions" {
		t.Errorf("expected path to be retained, got %s", updateResp.Route.PathPattern)
	}

	// 5. List routes with project_id filter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+tenantID+"/routes?project_id="+projectID, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list routes, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Routes []*Route `json:"routes"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(listResp.Routes))
	}

	// 6. Delete route by viewer -> 403
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tenants/"+tenantID+"/routes/"+routeID, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer deleting route, got %d", w.Code)
	}

	// 7. Delete route by owner -> 200 OK
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tenants/"+tenantID+"/routes/"+routeID, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner deleting route, got %d", w.Code)
	}
}
