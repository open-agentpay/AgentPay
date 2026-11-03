package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/open-agentpay/agentpay-facilitator-go/internal/config"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/service"
)

// Server coordinates the HTTP listener and dependencies.
type Server struct {
	cfg        *config.Config
	svc        *service.FacilitatorService
	httpServer *http.Server
}

// New creates an initialized HTTP server.
func New(cfg *config.Config, svc *service.FacilitatorService) *Server {
	mux := http.NewServeMux()

	// Operational endpoints
	mux.HandleFunc("/health", handleHealth(svc))
	mux.HandleFunc("/v1/health", handleHealth(svc))

	// Facilitator core API (protected by authentication middleware)
	mux.HandleFunc("/v1/verify", authMiddleware(cfg, handleVerify(svc)))
	mux.HandleFunc("/v1/settle", authMiddleware(cfg, handleSettle(svc)))
	mux.HandleFunc("/v1/settle/lookup", authMiddleware(cfg, handleSettlementLookup(svc)))
	mux.HandleFunc("/v1/settle/", authMiddleware(cfg, handleGetSettlement(svc)))
	mux.HandleFunc("/v1/nonce", authMiddleware(cfg, handleNonceCheck(svc)))
	mux.HandleFunc("/v1/nonces", authMiddleware(cfg, handleNonceCheck(svc)))

	handler := corsMiddleware(cfg, loggingMiddleware(mux))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		cfg:        cfg,
		svc:        svc,
		httpServer: srv,
	}
}

// authMiddleware enforces API key authentication on Facilitator endpoints.
// # SECURITY-REVIEW
func authMiddleware(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedKey := ""
		if cfg != nil {
			expectedKey = strings.TrimSpace(cfg.Security.APIKey)
		}
		if expectedKey == "" {
			expectedKey = strings.TrimSpace(os.Getenv("FACILITATOR_API_KEY"))
		}
		if expectedKey == "" {
			expectedKey = strings.TrimSpace(os.Getenv("AGENTPAY_FACILITATOR_API_KEY"))
		}

		providedKey := extractFacilitatorAPIKey(r)
		if expectedKey == "" || providedKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(expectedKey)) != 1 {
			writeError(w, fmt.Errorf("%w: missing or invalid facilitator API key", service.ErrUnauthorized))
			return
		}
		next(w, r)
	}
}

func extractFacilitatorAPIKey(r *http.Request) string {
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if k := strings.TrimSpace(r.Header.Get("X-Facilitator-Key")); k != "" {
		return k
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

// Start begins listening on the configured network address.
func (s *Server) Start() error {
	slog.Info("AgentPay Facilitator (Go) starting HTTP listener",
		slog.String("address", s.httpServer.Addr),
		slog.String("facilitator_address", s.svc.FacilitatorAddress().Hex()),
	)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Handler returns the underlying http.Handler (for httptest).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func corsMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if cfg != nil && len(cfg.Security.AllowedOrigins) > 0 && origin != "" {
			for _, allowed := range cfg.Security.AllowedOrigins {
				if allowed == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Facilitator-Key, X-API-Key, X-Requested-With, Idempotency-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("facilitator HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
