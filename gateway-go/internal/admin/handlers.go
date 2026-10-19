package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/payment"
	"github.com/open-agentpay/agentpay-gateway-go/internal/timeline"
	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

// APIHandler dispatches control plane requests.
type APIHandler struct {
	svc    Service
	jwtMgr *auth.JWTManager
}

// NewHandler creates a new HTTP handler for control plane REST API with default JWT manager.
func NewHandler(svc Service) http.Handler {
	return NewHandlerWithJWT(svc, auth.NewJWTManager("", 24*time.Hour))
}

// NewHandlerWithJWT creates a new HTTP handler for control plane REST API with explicit JWT manager.
func NewHandlerWithJWT(svc Service, jwtMgr *auth.JWTManager) http.Handler {
	if jwtMgr == nil {
		jwtMgr = auth.NewJWTManager("", 24*time.Hour)
	}
	return &APIHandler{svc: svc, jwtMgr: jwtMgr}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	method := r.Method

	actorMemberID := r.Header.Get("X-Actor-Member-Id")
	tenantIDHeader := r.Header.Get("X-Tenant-Id")
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// 0. Auth endpoints
	if path == "/api/v1/auth/register" && method == http.MethodPost {
		var body struct {
			TenantName string `json:"tenant_name"`
			Email      string `json:"email"`
			Password   string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			h.writeError(w, http.StatusBadRequest, "ERR_INVALID_JSON", "invalid request body")
			return
		}
		tenant, member, err := h.svc.Register(r.Context(), body.TenantName, body.Email, body.Password)
		if err != nil {
			h.handleError(w, err)
			return
		}
		token, err := h.jwtMgr.GenerateToken(member.MemberID, member.Email, tenant.TenantID, string(member.Role))
		if err != nil {
			h.handleError(w, err)
			return
		}
		h.writeJSON(w, http.StatusCreated, map[string]interface{}{
			"token":  token,
			"user":   member,
			"tenant": tenant,
		})
		return
	}

	if path == "/api/v1/auth/login" && method == http.MethodPost {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			h.writeError(w, http.StatusBadRequest, "ERR_INVALID_JSON", "invalid request body")
			return
		}
		member, tenant, err := h.svc.Login(r.Context(), body.Email, body.Password)
		if err != nil {
			h.handleError(w, err)
			return
		}
		token, err := h.jwtMgr.GenerateToken(member.MemberID, member.Email, tenant.TenantID, string(member.Role))
		if err != nil {
			h.handleError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"token":  token,
			"user":   member,
			"tenant": tenant,
		})
		return
	}

	if path == "/api/v1/auth-types" && method == http.MethodGet {
		schemas := h.svc.GetAuthTypes()
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"types": schemas})
		return
	}

	if path == "/api/v1/auth/me" && method == http.MethodGet {
		effectiveMemberID := actorMemberID
		if effectiveMemberID == "" {
			effectiveMemberID = auth.GetMemberID(r.Context())
		}
		effectiveTenantID := tenantIDHeader
		if effectiveTenantID == "" {
			effectiveTenantID = auth.GetTenantID(r.Context())
		}

		if effectiveMemberID == "" {
			h.writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "unauthenticated user")
			return
		}

		member, err := h.svc.GetMember(r.Context(), effectiveTenantID, effectiveMemberID)
		if err != nil {
			h.handleError(w, err)
			return
		}

		tenant, err := h.svc.GetTenant(r.Context(), member.TenantID)
		if err != nil {
			h.handleError(w, err)
			return
		}

		permissions := GetRolePermissions(member.Role)
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"user":        member,
			"tenant":      tenant,
			"permissions": permissions,
		})
		return
	}

	if path == "/api/v1/auth/password" && method == http.MethodPost {
		effectiveMemberID := actorMemberID
		if effectiveMemberID == "" {
			effectiveMemberID = auth.GetMemberID(r.Context())
		}
		effectiveTenantID := tenantIDHeader
		if effectiveTenantID == "" {
			effectiveTenantID = auth.GetTenantID(r.Context())
		}
		if effectiveMemberID == "" {
			h.writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "unauthenticated user")
			return
		}
		var body struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			h.writeError(w, http.StatusBadRequest, "ERR_INVALID_JSON", "invalid request body")
			return
		}
		m, err := h.svc.GetMember(r.Context(), effectiveTenantID, effectiveMemberID)
		if err != nil {
			h.handleError(w, err)
			return
		}
		if m.PasswordHash != "" && body.OldPassword != "" {
			if !auth.VerifyPassword(body.OldPassword, m.PasswordHash) {
				h.writeError(w, http.StatusUnauthorized, "ERR_INVALID_CREDENTIALS", "old password is incorrect")
				return
			}
		}
		if err := h.svc.SetMemberPassword(r.Context(), m.TenantID, m.MemberID, body.NewPassword); err != nil {
			h.handleError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
		return
	}

	// 1. Audit events
	if method == http.MethodGet && path == "/api/v1/audit/events" {
		tID := r.URL.Query().Get("tenant_id")
		if tID == "" {
			tID = tenantIDHeader
		}
		targetType := r.URL.Query().Get("target_type")
		targetID := r.URL.Query().Get("target_id")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		events, err := h.svc.ListAuditEvents(r.Context(), tID, targetType, targetID, limit)
		if err != nil {
			h.handleError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(events), "items": events})
		return
	}

	// 2. Flat Agent endpoints
	if strings.HasPrefix(path, "/api/v1/agents/") {
		remainder := strings.TrimPrefix(path, "/api/v1/agents/")
		parts := strings.Split(remainder, "/")

		if len(parts) == 2 && parts[1] == "kill" && method == http.MethodPost {
			agentID := parts[0]
			var body struct {
				Reason string `json:"reason"`
				Revoke bool   `json:"revoke"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ag, keys, err := h.svc.KillAgent(r.Context(), tenantIDHeader, agentID, body.Reason, actorMemberID, body.Revoke, clientIP)
			if err != nil {
				h.handleError(w, err)
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]interface{}{
				"agent":        ag,
				"revoked_keys": keys,
				"killed":       true,
				"status":       ag.Status,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "timeline" && method == http.MethodGet {
			agentID := parts[0]
			h.handleAgentTimeline(w, r, tenantIDHeader, agentID, actorMemberID)
			return
		}

		if len(parts) == 1 {
			agentID := parts[0]
			switch method {
			case http.MethodGet:
				ag, err := h.svc.GetAgent(r.Context(), tenantIDHeader, agentID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"agent": ag})
				return
			case http.MethodPatch:
				var body struct {
					Name         string                 `json:"name"`
					PolicyConfig map[string]interface{} `json:"policy_config"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				ag, err := h.svc.UpdateAgent(r.Context(), tenantIDHeader, agentID, body.Name, body.PolicyConfig, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"agent": ag})
				return
			case http.MethodDelete:
				err := h.svc.DeleteAgent(r.Context(), tenantIDHeader, agentID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "agent_id": agentID})
				return
			}
		}
	}

	// 3. Flat Wallet endpoints
	if strings.HasPrefix(path, "/api/v1/wallets/") {
		remainder := strings.TrimPrefix(path, "/api/v1/wallets/")
		parts := strings.Split(remainder, "/")
		walletID := parts[0]

		if len(parts) == 1 && method == http.MethodGet {
			wlt, err := h.svc.GetWalletByID(r.Context(), walletID)
			if err != nil {
				h.handleError(w, err)
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
			return
		}

		if len(parts) == 2 {
			action := parts[1]
			switch action {
			case "adjust":
				if method == http.MethodPost {
					var body struct {
						Amount         int64  `json:"amount"`
						Reason         string `json:"reason"`
						IdempotencyKey string `json:"idempotency_key"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					err := h.svc.AdjustWallet(r.Context(), walletID, body.Amount, body.Reason, body.IdempotencyKey, actorMemberID, clientIP)
					if err != nil {
						h.handleError(w, err)
						return
					}
					wlt, _ := h.svc.GetWalletByID(r.Context(), walletID)
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
					return
				}
			case "freeze":
				if method == http.MethodPost {
					var body struct {
						Reason string `json:"reason"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					err := h.svc.FreezeWallet(r.Context(), walletID, body.Reason, actorMemberID, clientIP)
					if err != nil {
						h.handleError(w, err)
						return
					}
					wlt, _ := h.svc.GetWalletByID(r.Context(), walletID)
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
					return
				}
			case "unfreeze":
				if method == http.MethodPost {
					var body struct {
						Reason string `json:"reason"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					err := h.svc.UnfreezeWallet(r.Context(), walletID, body.Reason, actorMemberID, clientIP)
					if err != nil {
						h.handleError(w, err)
						return
					}
					wlt, _ := h.svc.GetWalletByID(r.Context(), walletID)
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
					return
				}
			case "recalculate":
				if method == http.MethodPost {
					report, err := h.svc.RecalculateWallet(r.Context(), walletID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"recalculation": report})
					return
				}
			case "ledger":
				if method == http.MethodGet {
					entries, err := h.svc.GetLedgerEntries(r.Context(), walletID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(entries), "items": entries})
					return
				}
			case "topups":
				if method == http.MethodGet {
					orders, err := h.svc.GetTopUpOrders(r.Context(), walletID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(orders), "items": orders})
					return
				}
			}
		}
	}

	// 4. Scoped Tenants endpoints: /api/v1/tenants/...
	if path == "/api/v1/tenants" {
		if method == http.MethodPost {
			var body struct {
				Name       string `json:"name"`
				TenantID   string `json:"tenant_id"`
				OwnerEmail string `json:"owner_email"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				h.writeError(w, http.StatusBadRequest, "ERR_INVALID_PARAM", "malformed JSON body")
				return
			}
			t, owner, err := h.svc.CreateTenant(r.Context(), body.Name, body.TenantID, body.OwnerEmail)
			if err != nil {
				h.handleError(w, err)
				return
			}
			resp := map[string]interface{}{"tenant": t}
			if owner != nil {
				resp["owner"] = owner
			}
			h.writeJSON(w, http.StatusCreated, resp)
			return
		}
		if method == http.MethodGet {
			tenants, err := h.svc.ListTenants(r.Context())
			if err != nil {
				h.handleError(w, err)
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]interface{}{"tenants": tenants})
			return
		}
	}

	if strings.HasPrefix(path, "/api/v1/tenants/") {
		rem := strings.TrimPrefix(path, "/api/v1/tenants/")
		parts := strings.Split(rem, "/")
		tenantID := parts[0]

		if len(parts) == 1 {
			switch method {
			case http.MethodGet:
				t, err := h.svc.GetTenant(r.Context(), tenantID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"tenant": t})
				return
			case http.MethodPatch:
				var body struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				t, err := h.svc.UpdateTenant(r.Context(), tenantID, body.Name, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"tenant": t})
				return
			}
		}

		if len(parts) == 2 && parts[1] == "status" && method == http.MethodPost {
			var body struct {
				Status string `json:"status"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			t, err := h.svc.UpdateTenantStatus(r.Context(), tenantID, TenantStatus(body.Status), actorMemberID)
			if err != nil {
				h.handleError(w, err)
				return
			}
			h.writeJSON(w, http.StatusOK, map[string]interface{}{"tenant": t})
			return
		}

		// Projects
		if parts[1] == "projects" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var body struct {
						Name               string `json:"name"`
						BudgetCapBaseUnits int64  `json:"budget_cap_base_units"`
						ProjectID          string `json:"project_id"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					p, err := h.svc.CreateProject(r.Context(), tenantID, body.Name, body.BudgetCapBaseUnits, body.ProjectID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{"project": p})
					return
				}
				if method == http.MethodGet {
					projects, err := h.svc.ListProjects(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"projects": projects})
					return
				}
			}
			if len(parts) == 3 {
				projectID := parts[2]
				switch method {
				case http.MethodGet:
					p, err := h.svc.GetProject(r.Context(), tenantID, projectID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"project": p})
					return
				case http.MethodPatch:
					var body struct {
						Name               string         `json:"name"`
						BudgetCapBaseUnits *int64         `json:"budget_cap_base_units"`
						Status             *ProjectStatus `json:"status"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					p, err := h.svc.UpdateProject(r.Context(), tenantID, projectID, body.Name, body.BudgetCapBaseUnits, body.Status, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"project": p})
					return
				case http.MethodDelete:
					err := h.svc.DeleteProject(r.Context(), tenantID, projectID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "project_id": projectID})
					return
				}
			}
		}

		// Members
		if parts[1] == "members" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var body struct {
						Email    string `json:"email"`
						Role     string `json:"role"`
						Password string `json:"password,omitempty"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					m, err := h.svc.AddMember(r.Context(), tenantID, body.Email, MemberRole(body.Role), actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					if body.Password != "" {
						_ = h.svc.SetMemberPassword(r.Context(), tenantID, m.MemberID, body.Password)
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{"member": m})
					return
				}
				if method == http.MethodGet {
					members, err := h.svc.ListMembers(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"members": members})
					return
				}
			}
			if len(parts) == 3 {
				memberID := parts[2]
				switch method {
				case http.MethodGet:
					m, err := h.svc.GetMember(r.Context(), tenantID, memberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"member": m})
					return
				case http.MethodPatch:
					var body struct {
						Role   *MemberRole   `json:"role"`
						Status *MemberStatus `json:"status"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					m, err := h.svc.UpdateMember(r.Context(), tenantID, memberID, body.Role, body.Status, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"member": m})
					return
				case http.MethodDelete:
					err := h.svc.RemoveMember(r.Context(), tenantID, memberID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "member_id": memberID})
					return
				}
			}
		}

		// Agents
		if parts[1] == "agents" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var body struct {
						ProjectID    string                 `json:"project_id"`
						Name         string                 `json:"name"`
						PolicyConfig map[string]interface{} `json:"policy_config"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					ag, err := h.svc.CreateAgent(r.Context(), tenantID, body.ProjectID, body.Name, body.PolicyConfig, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{"agent": ag})
					return
				}
				if method == http.MethodGet {
					projectID := r.URL.Query().Get("project_id")
					var statusPtr *AgentStatus
					if s := r.URL.Query().Get("status"); s != "" {
						st := AgentStatus(s)
						statusPtr = &st
					}
					agents, err := h.svc.ListAgents(r.Context(), tenantID, projectID, statusPtr)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"agents": agents})
					return
				}
			}
			if len(parts) >= 3 {
				agentID := parts[2]
				if len(parts) == 4 && parts[3] == "kill" && method == http.MethodPost {
					var body struct {
						Reason string `json:"reason"`
						Revoke bool   `json:"revoke"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					ag, keys, err := h.svc.KillAgent(r.Context(), tenantID, agentID, body.Reason, actorMemberID, body.Revoke, clientIP)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{
						"agent":        ag,
						"revoked_keys": keys,
						"killed":       true,
						"status":       ag.Status,
					})
					return
				}
				if len(parts) == 4 && parts[3] == "status" && method == http.MethodPost {
					var body struct {
						Status string `json:"status"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					ag, err := h.svc.UpdateAgentStatus(r.Context(), tenantID, agentID, AgentStatus(body.Status), actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"agent": ag})
					return
				}
				if len(parts) == 4 && parts[3] == "timeline" && method == http.MethodGet {
					h.handleAgentTimeline(w, r, tenantID, agentID, actorMemberID)
					return
				}
				if len(parts) == 3 {
					switch method {
					case http.MethodGet:
						ag, err := h.svc.GetAgent(r.Context(), tenantID, agentID)
						if err != nil {
							h.handleError(w, err)
							return
						}
						h.writeJSON(w, http.StatusOK, map[string]interface{}{"agent": ag})
						return
					case http.MethodPatch:
						var body struct {
							Name         string                 `json:"name"`
							PolicyConfig map[string]interface{} `json:"policy_config"`
						}
						_ = json.NewDecoder(r.Body).Decode(&body)
						ag, err := h.svc.UpdateAgent(r.Context(), tenantID, agentID, body.Name, body.PolicyConfig, actorMemberID)
						if err != nil {
							h.handleError(w, err)
							return
						}
						h.writeJSON(w, http.StatusOK, map[string]interface{}{"agent": ag})
						return
					case http.MethodDelete:
						err := h.svc.DeleteAgent(r.Context(), tenantID, agentID, actorMemberID)
						if err != nil {
							h.handleError(w, err)
							return
						}
						h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "agent_id": agentID})
						return
					}
				}
			}
		}

		// API Keys
		if parts[1] == "api-keys" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var body struct {
						ProjectID string   `json:"project_id"`
						AgentID   string   `json:"agent_id"`
						Name      string   `json:"name"`
						Scopes    []string `json:"scopes"`
						ExpiresAt *int64   `json:"expires_at"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					res, err := h.svc.CreateAPIKey(r.Context(), tenantID, body.ProjectID, body.AgentID, body.Name, body.Scopes, body.ExpiresAt, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{
						"api_key": res.APIKey,
						"raw_key": res.RawKey,
					})
					return
				}
				if method == http.MethodGet {
					agentID := r.URL.Query().Get("agent_id")
					keys, err := h.svc.ListAPIKeys(r.Context(), tenantID, agentID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"api_keys": keys})
					return
				}
			}
			if len(parts) == 4 {
				keyID := parts[2]
				action := parts[3]
				if action == "revoke" && method == http.MethodPost {
					k, err := h.svc.RevokeAPIKey(r.Context(), tenantID, keyID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"api_key": k})
					return
				}
				if action == "rotate" && method == http.MethodPost {
					res, err := h.svc.RotateAPIKey(r.Context(), tenantID, keyID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{
						"api_key": res.APIKey,
						"raw_key": res.RawKey,
					})
					return
				}
			}
		}

		// Routes
		if parts[1] == "routes" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var rBody Route
					_ = json.NewDecoder(r.Body).Decode(&rBody)
					rBody.TenantID = tenantID
					created, err := h.svc.CreateRoute(r.Context(), &rBody, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{"route": created})
					return
				}
				if method == http.MethodGet {
					routes, err := h.svc.ListRoutes(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					projectID := r.URL.Query().Get("project_id")
					if projectID != "" {
						filtered := make([]*Route, 0)
						for _, rt := range routes {
							if rt.ProjectID == projectID {
								filtered = append(filtered, rt)
							}
						}
						routes = filtered
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"routes": routes})
					return
				}
			}
			if len(parts) == 3 && parts[2] == "test-upstream" && method == http.MethodPost {
				var body TestUpstreamInput
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					h.writeError(w, http.StatusBadRequest, "ERR_INVALID_PARAM", "malformed JSON body")
					return
				}
				res, err := h.svc.TestUpstream(r.Context(), tenantID, &body, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"result": res})
				return
			}
			if len(parts) == 3 {
				routeID := parts[2]
				switch method {
				case http.MethodGet:
					route, err := h.svc.GetRoute(r.Context(), tenantID, routeID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"route": route})
					return
				case http.MethodPatch:
					var rBody UpdateRouteInput
					_ = json.NewDecoder(r.Body).Decode(&rBody)
					rBody.RouteID = routeID
					rBody.TenantID = tenantID
					updated, err := h.svc.UpdateRoute(r.Context(), &rBody, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"route": updated})
					return
				case http.MethodDelete:
					err := h.svc.DeleteRoute(r.Context(), tenantID, routeID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "route_id": routeID})
					return
				}
			}
		}

		// Policies
		if parts[1] == "policies" {
			if len(parts) == 2 {
				if method == http.MethodPost {
					var pBody Policy
					_ = json.NewDecoder(r.Body).Decode(&pBody)
					pBody.TenantID = tenantID
					created, err := h.svc.CreatePolicy(r.Context(), &pBody, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusCreated, map[string]interface{}{"policy": created})
					return
				}
				if method == http.MethodGet {
					projectID := r.URL.Query().Get("project_id")
					agentID := r.URL.Query().Get("agent_id")
					policies, err := h.svc.ListPolicies(r.Context(), tenantID, projectID, agentID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"policies": policies})
					return
				}
			}
			if len(parts) == 3 {
				policyID := parts[2]
				switch method {
				case http.MethodGet:
					pol, err := h.svc.GetPolicy(r.Context(), tenantID, policyID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"policy": pol})
					return
				case http.MethodPatch:
					var pBody Policy
					_ = json.NewDecoder(r.Body).Decode(&pBody)
					pBody.PolicyID = policyID
					pBody.TenantID = tenantID
					updated, err := h.svc.UpdatePolicy(r.Context(), &pBody, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"policy": updated})
					return
				case http.MethodDelete:
					err := h.svc.DeletePolicy(r.Context(), tenantID, policyID, actorMemberID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "policy_id": policyID})
					return
				}
			}
		}

		// Config Draft, Validation, Publishing & Rollback (T4.3 / T4.4)
		if parts[1] == "config" {
			if len(parts) >= 3 && parts[2] == "validate" && method == http.MethodPost {
				res, err := h.svc.ValidateConfig(r.Context(), tenantID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"result": res})
				return
			}
			if len(parts) >= 3 && parts[2] == "publish" && method == http.MethodPost {
				var body struct {
					ChangeSummary string `json:"change_summary"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				ver, err := h.svc.PublishConfig(r.Context(), tenantID, body.ChangeSummary, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"version": ver})
				return
			}
			if len(parts) >= 3 && parts[2] == "rollback" && method == http.MethodPost {
				var body struct {
					TargetVersionID string `json:"target_version_id"`
					Reason          string `json:"reason"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					h.writeError(w, http.StatusBadRequest, "ERR_INVALID_JSON", "invalid request body")
					return
				}
				ver, err := h.svc.RollbackConfig(r.Context(), tenantID, body.TargetVersionID, body.Reason, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"version": ver})
				return
			}
			if len(parts) >= 3 && parts[2] == "versions" {
				if len(parts) == 4 && parts[3] == "diff" && method == http.MethodGet {
					baseVer := r.URL.Query().Get("base")
					if baseVer == "" {
						baseVer = r.URL.Query().Get("from")
					}
					targetVer := r.URL.Query().Get("target")
					if targetVer == "" {
						targetVer = r.URL.Query().Get("to")
					}
					diff, err := h.svc.GetConfigVersionDiff(r.Context(), tenantID, baseVer, targetVer)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"diff": diff})
					return
				}
				if len(parts) == 3 && method == http.MethodGet {
					list, err := h.svc.ListConfigVersions(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"versions": list})
					return
				}
				if len(parts) == 4 && method == http.MethodGet {
					verID := parts[3]
					ver, err := h.svc.GetConfigVersion(r.Context(), tenantID, verID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"version": ver})
					return
				}
			}
			if len(parts) >= 3 && parts[2] == "draft" {
				if len(parts) == 4 && parts[3] == "diff" && method == http.MethodGet {
					diff, err := h.svc.GetDraftDiff(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"diff": diff})
					return
				}
				if len(parts) == 3 && method == http.MethodGet {
					summary, err := h.svc.GetDraftSummary(r.Context(), tenantID)
					if err != nil {
						h.handleError(w, err)
						return
					}
					h.writeJSON(w, http.StatusOK, map[string]interface{}{"draft": summary})
					return
				}
			}
		}

		// Wallets under tenant
		if parts[1] == "wallets" {
			if len(parts) == 2 && method == http.MethodGet {
				wlt, err := h.svc.GetWallet(r.Context(), tenantID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
				return
			}
			if len(parts) == 3 && parts[2] == "admin-grant" && method == http.MethodPost {
				var body struct {
					Amount         int64  `json:"amount"`
					Reason         string `json:"reason"`
					IdempotencyKey string `json:"idempotency_key"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				err := h.svc.AdminGrantWallet(r.Context(), tenantID, body.Amount, body.Reason, body.IdempotencyKey, actorMemberID, clientIP)
				if err != nil {
					h.handleError(w, err)
					return
				}
				wlt, _ := h.svc.GetWallet(r.Context(), tenantID)
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"wallet": wlt})
				return
			}
		}

		// Request records under tenant (T5.1)
		if parts[1] == "requests" {
			if len(parts) == 2 && method == http.MethodGet {
				q := r.URL.Query()
				limit, _ := strconv.Atoi(q.Get("limit"))
				offset, _ := strconv.Atoi(q.Get("offset"))
				statusCode, _ := strconv.Atoi(q.Get("status_code"))

				var isReplay *bool
				if rStr := q.Get("is_replay"); rStr != "" {
					b := rStr == "true" || rStr == "1"
					isReplay = &b
				}

				filter := usage.RequestFilter{
					ProjectID:  q.Get("project_id"),
					AgentID:    q.Get("agent_id"),
					RouteID:    q.Get("route_id"),
					StatusCode: statusCode,
					IsReplay:   isReplay,
					Limit:      limit,
					Offset:     offset,
				}

				list, total, err := h.svc.ListRequestRecords(r.Context(), tenantID, filter, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{
					"requests": list,
					"total":    total,
					"limit":    limit,
					"offset":   offset,
				})
				return
			}
			if len(parts) == 3 && method == http.MethodGet {
				reqID := parts[2]
				rec, err := h.svc.GetRequestRecord(r.Context(), tenantID, reqID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"request": rec})
				return
			}
		}

		// Usage events under tenant (T5.1)
		if parts[1] == "usages" {
			if len(parts) == 2 && method == http.MethodGet {
				q := r.URL.Query()
				limit, _ := strconv.Atoi(q.Get("limit"))
				offset, _ := strconv.Atoi(q.Get("offset"))

				var isSuccess *bool
				if sStr := q.Get("is_success"); sStr != "" {
					b := sStr == "true" || sStr == "1"
					isSuccess = &b
				}

				filter := usage.UsageFilter{
					ProjectID:     q.Get("project_id"),
					AgentID:       q.Get("agent_id"),
					RouteID:       q.Get("route_id"),
					IntentID:      q.Get("intent_id"),
					PaymentMethod: q.Get("payment_method"),
					IsSuccess:     isSuccess,
					Limit:         limit,
					Offset:        offset,
				}

				list, total, err := h.svc.ListUsageEvents(r.Context(), tenantID, filter, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{
					"usages": list,
					"total":  total,
					"limit":  limit,
					"offset": offset,
				})
				return
			}
			if len(parts) == 3 && method == http.MethodGet {
				eventID := parts[2]
				evt, err := h.svc.GetUsageEvent(r.Context(), tenantID, eventID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"usage": evt})
				return
			}
		}

		// Payment correlation under tenant (T5.2)
		if parts[1] == "payments" {
			if len(parts) == 2 && method == http.MethodGet {
				q := r.URL.Query()
				limit, _ := strconv.Atoi(q.Get("limit"))
				offset, _ := strconv.Atoi(q.Get("offset"))

				filter := payment.PaymentAuditFilter{
					ProjectID:     q.Get("project_id"),
					AgentID:       q.Get("agent_id"),
					RouteID:       q.Get("route_id"),
					PaymentMethod: q.Get("payment_method"),
					DisplayStatus: q.Get("display_status"),
					IntentStatus:  q.Get("intent_status"),
					Limit:         limit,
					Offset:        offset,
				}
				if st := q.Get("start_time"); st != "" {
					if t, err := time.Parse(time.RFC3339, st); err == nil {
						filter.StartTime = &t
					}
				}
				if et := q.Get("end_time"); et != "" {
					if t, err := time.Parse(time.RFC3339, et); err == nil {
						filter.EndTime = &t
					}
				}

				list, total, err := h.svc.ListPaymentAudits(r.Context(), tenantID, filter, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{
					"payments": list,
					"total":    total,
					"limit":    limit,
					"offset":   offset,
				})
				return
			}
			if len(parts) == 3 && method == http.MethodGet {
				intentID := parts[2]
				rec, err := h.svc.GetPaymentAudit(r.Context(), tenantID, intentID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"payment": rec})
				return
			}
			if len(parts) == 4 && parts[2] == "by-request" && method == http.MethodGet {
				reqID := parts[3]
				rec, err := h.svc.GetPaymentAuditByRequestID(r.Context(), tenantID, reqID, actorMemberID)
				if err != nil {
					h.handleError(w, err)
					return
				}
				h.writeJSON(w, http.StatusOK, map[string]interface{}{"payment": rec})
				return
			}
		}
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTenantNotFound),
		errors.Is(err, ErrProjectNotFound),
		errors.Is(err, ErrMemberNotFound),
		errors.Is(err, ErrAgentNotFound),
		errors.Is(err, ErrAPIKeyNotFound),
		errors.Is(err, ErrRouteNotFound),
		errors.Is(err, ErrPolicyNotFound),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, usage.ErrRequestRecordNotFound),
		errors.Is(err, usage.ErrUsageEventNotFound),
		errors.Is(err, payment.ErrIntentNotFound),
		errors.Is(err, payment.ErrSettlementNotFound),
		errors.Is(err, payment.ErrFulfillmentNotFound):
		h.writeError(w, http.StatusNotFound, "ERR_NOT_FOUND", err.Error())
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrMemberDisabled),
		errors.Is(err, ErrTenantSuspended),
		errors.Is(err, ErrTenantTerminated),
		errors.Is(err, ErrProjectArchived),
		errors.Is(err, ErrProjectTenantMismatch),
		errors.Is(err, ErrAgentTenantMismatch),
		errors.Is(err, ErrAgentSuspended),
		errors.Is(err, ErrAgentRevoked),
		errors.Is(err, usage.ErrTenantMismatch),
		errors.Is(err, payment.ErrTenantMismatch):
		h.writeError(w, http.StatusForbidden, "ERR_PERMISSION_DENIED", err.Error())
	case errors.Is(err, ErrTenantAlreadyExists),
		errors.Is(err, ErrProjectAlreadyExists),
		errors.Is(err, ErrMemberAlreadyExists),
		errors.Is(err, ErrAgentAlreadyExists),
		errors.Is(err, ErrRouteAlreadyExists):
		h.writeError(w, http.StatusConflict, "ERR_ALREADY_EXISTS", err.Error())
	case errors.Is(err, ErrMissingReason):
		h.writeError(w, http.StatusBadRequest, "ERR_MISSING_REASON", err.Error())
	case errors.Is(err, ErrInvalidStatusTransition),
		errors.Is(err, ErrCannotRemoveLastOwner),
		errors.Is(err, ErrCannotDemoteLastOwner),
		errors.Is(err, ErrInvalidAmount):
		h.writeError(w, http.StatusBadRequest, "ERR_INVALID_PARAM", err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		h.writeError(w, http.StatusUnauthorized, "ERR_INVALID_CREDENTIALS", err.Error())
	case errors.Is(err, ErrPasswordTooShort):
		h.writeError(w, http.StatusBadRequest, "ERR_PASSWORD_TOO_SHORT", err.Error())
	case errors.Is(err, ErrEmailAlreadyRegistered):
		h.writeError(w, http.StatusConflict, "ERR_EMAIL_ALREADY_REGISTERED", err.Error())
	case errors.Is(err, ErrConfigValidationFailed):
		h.writeError(w, http.StatusUnprocessableEntity, "ERR_CONFIG_VALIDATION_FAILED", err.Error())
	case errors.Is(err, ErrConfigVersionNotFound):
		h.writeError(w, http.StatusNotFound, "ERR_CONFIG_VERSION_NOT_FOUND", err.Error())
	case errors.Is(err, ErrCannotRollbackToActive):
		h.writeError(w, http.StatusBadRequest, "ERR_CANNOT_ROLLBACK_TO_ACTIVE", err.Error())
	case errors.Is(err, wallet.ErrInsufficientBalance):
		h.writeError(w, http.StatusPaymentRequired, "ERR_INSUFFICIENT_FUNDS", err.Error())
	default:
		h.writeError(w, http.StatusBadRequest, "ERR_BAD_REQUEST", err.Error())
	}
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	h.writeJSON(w, statusCode, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (h *APIHandler) handleAgentTimeline(w http.ResponseWriter, r *http.Request, tenantID, agentID, actorMemberID string) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	filter := timeline.TimelineFilter{
		SessionID: q.Get("session_id"),
		TaskID:    q.Get("task_id"),
		EventType: q.Get("event_type"),
		Limit:     limit,
		Offset:    offset,
	}

	if st := q.Get("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			filter.StartTime = &t
		} else if sec, err := strconv.ParseInt(st, 10, 64); err == nil {
			if sec > 1e11 {
				t := time.UnixMilli(sec)
				filter.StartTime = &t
			} else {
				t := time.Unix(sec, 0)
				filter.StartTime = &t
			}
		}
	}
	if et := q.Get("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			filter.EndTime = &t
		} else if sec, err := strconv.ParseInt(et, 10, 64); err == nil {
			if sec > 1e11 {
				t := time.UnixMilli(sec)
				filter.EndTime = &t
			} else {
				t := time.Unix(sec, 0)
				filter.EndTime = &t
			}
		}
	}

	events, total, sessions, err := h.svc.GetAgentTimeline(r.Context(), tenantID, agentID, filter, actorMemberID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if events == nil {
		events = []*timeline.TimelineEvent{}
	}
	if sessions == nil {
		sessions = []string{}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":    events,
		"total":    total,
		"count":    len(events),
		"sessions": sessions,
	})
}

