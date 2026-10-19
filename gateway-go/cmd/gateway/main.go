// AgentPay Go Gateway Service Entrypoint (T2.1)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-agentpay/agentpay-gateway-go/internal/config"
	"github.com/open-agentpay/agentpay-gateway-go/internal/logger"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/server"
)

func main() {
	configPath := flag.String("config", "", "path to gateway YAML configuration file")
	showVersion := flag.Bool("version", false, "print gateway version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("AgentPay Gateway v%s (commit: %s, built: %s)\n",
			server.Version, server.GitCommit, server.BuildDate)
		os.Exit(0)
	}

	// 1. Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: configuration failure: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize structured logger
	log := logger.InitGlobalLogger(logger.Config{
		Level:  cfg.Logger.Level,
		Format: cfg.Logger.Format,
	}, os.Stdout)

	log.Info("initializing AgentPay Go Gateway",
		slog.String("version", server.Version),
		slog.String("log_level", cfg.Logger.Level),
		slog.Int("port", cfg.Server.Port),
	)

	// 3. Initialize RCU configuration snapshot with bootstrap routes
	bootstrapRoutes := make([]rcu.RouteConfig, len(cfg.Routes))
	for i, r := range cfg.Routes {
		netID := r.Network
		if netID == "" {
			netID = cfg.Settlement.DefaultNetwork
		}
		bootstrapRoutes[i] = rcu.RouteConfig{
			RouteID:               r.RouteID,
			PathPattern:           r.Path,
			UpstreamURL:           r.Upstream,
			PriceBaseUnits:        r.PriceBaseUnits,
			Asset:                 r.Asset,
			Network:               netID,
			PayeeAddress:          r.PayeeAddress,
			AllowedPaymentMethods: r.AllowedPaymentMethods,
			RateLimitRPM:          r.RateLimitRPM,
			Status:                "active",
		}
	}
	initialSnapshot := &rcu.ConfigSnapshot{
		SchemaVersion: cfg.Version,
		TenantID:      "ten_bootstrap",
		VersionID:     "ver_bootstrap_0",
		VersionNumber: 0,
		Routes:        bootstrapRoutes,
	}

	rcuMgr, err := rcu.NewManager(initialSnapshot)
	if err != nil {
		log.Error("failed to initialize RCU configuration cache", slog.Any("error", err))
		os.Exit(1)
	}

	// 4. Initialize metrics and HTTP server
	m := metrics.New()
	srv, err := server.New(cfg, rcuMgr, m)
	if err != nil {
		log.Error("failed to create gateway server", slog.Any("error", err))
		os.Exit(1)
	}

	// 5. Start listener in background goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serverErrCh <- err
		}
	}()

	// 6. Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrCh:
		log.Error("gateway HTTP server encountered fatal error", slog.Any("error", err))
		os.Exit(1)
	case sig := <-sigCh:
		log.Info("received termination signal, initiating graceful shutdown", slog.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout())
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
			os.Exit(1)
		}
		log.Info("AgentPay Go Gateway terminated cleanly")
	}
}
