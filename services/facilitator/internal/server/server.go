package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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

	// Facilitator core API
	mux.HandleFunc("/v1/verify", handleVerify(svc))
	mux.HandleFunc("/v1/settle", handleSettle(svc))
	mux.HandleFunc("/v1/settle/", handleGetSettlement(svc))
	mux.HandleFunc("/v1/nonce", handleNonceCheck(svc))
	mux.HandleFunc("/v1/nonces", handleNonceCheck(svc))

	handler := corsMiddleware(loggingMiddleware(mux))

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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Idempotency-Key")

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
