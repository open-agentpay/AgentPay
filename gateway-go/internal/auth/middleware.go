package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const (
	ClaimsContextKey   contextKey = "agentpay.auth.claims"
	MemberIDContextKey contextKey = "agentpay.auth.member_id"
	TenantIDContextKey contextKey = "agentpay.auth.tenant_id"
	RoleContextKey     contextKey = "agentpay.auth.role"
)

// Middleware returns an HTTP middleware that extracts and validates Bearer JWT tokens.
func Middleware(jwtMgr *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := jwtMgr.ValidateToken(tokenStr)
				if err != nil {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]string{
							"code":    "ERR_UNAUTHORIZED",
							"message": err.Error(),
						},
					})
					return
				}

				// Inject claims into context
				ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
				ctx = context.WithValue(ctx, MemberIDContextKey, claims.Subject)
				ctx = context.WithValue(ctx, TenantIDContextKey, claims.TenantID)
				ctx = context.WithValue(ctx, RoleContextKey, claims.Role)
				r = r.WithContext(ctx)

				// Propagate to headers for downstream handler compatibility
				r.Header.Set("X-Actor-Member-Id", claims.Subject)
				r.Header.Set("X-Tenant-Id", claims.TenantID)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClaims retrieves Claims from request context if present.
func GetClaims(ctx context.Context) *Claims {
	if val, ok := ctx.Value(ClaimsContextKey).(*Claims); ok {
		return val
	}
	return nil
}

// GetMemberID retrieves authenticated member ID from request context.
func GetMemberID(ctx context.Context) string {
	if val, ok := ctx.Value(MemberIDContextKey).(string); ok {
		return val
	}
	return ""
}

// GetTenantID retrieves authenticated tenant ID from request context.
func GetTenantID(ctx context.Context) string {
	if val, ok := ctx.Value(TenantIDContextKey).(string); ok {
		return val
	}
	return ""
}
