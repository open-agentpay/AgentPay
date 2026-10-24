package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/open-agentpay/agentpay-facilitator-go/internal/config"
	agentevm "github.com/open-agentpay/agentpay-facilitator-go/internal/evm"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/server"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/service"
)

const Version = "1.0.0"

func main() {
	configPath := flag.String("config", "", "Path to YAML configuration file (optional)")
	showVersion := flag.Bool("version", false, "Print service version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("AgentPay Facilitator (Go) v%s\n", Version)
		return
	}

	// Fallback to configs/facilitator.yaml if exists and not explicitly specified
	if *configPath == "" {
		if _, err := os.Stat("configs/facilitator.yaml"); err == nil {
			*configPath = "configs/facilitator.yaml"
		}
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	var logHandler slog.Handler
	logOpts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if cfg.Logger.Level == "debug" {
		logOpts.Level = slog.LevelDebug
	} else if cfg.Logger.Level == "warn" {
		logOpts.Level = slog.LevelWarn
	} else if cfg.Logger.Level == "error" {
		logOpts.Level = slog.LevelError
	}

	if cfg.Logger.Format == "text" {
		logHandler = slog.NewTextHandler(os.Stdout, logOpts)
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, logOpts)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("initializing AgentPay Facilitator",
		slog.String("version", Version),
		slog.String("network_chain_id", fmt.Sprintf("%d", cfg.EVM.ChainID)),
		slog.String("token_contract", cfg.EVM.TokenAddress),
	)

	// Initialize EVM Client
	var evmClient agentevm.Client
	if cfg.EVM.RPCURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rpcCli, err := agentevm.NewRPCClient(ctx, cfg.EVM.RPCURL, cfg.BigChainID())
		cancel()

		if err != nil {
			slog.Warn("failed to connect to EVM RPC endpoint, falling back to mock client",
				slog.String("rpc_url", cfg.EVM.RPCURL),
				slog.Any("error", err),
			)
			evmClient = agentevm.NewMockClient(cfg.BigChainID())
		} else {
			slog.Info("connected to live EVM RPC endpoint", slog.String("rpc_url", cfg.EVM.RPCURL))
			evmClient = rpcCli
		}
	} else {
		slog.Info("no RPC_URL provided, operating with in-memory mock EVM client")
		evmClient = agentevm.NewMockClient(cfg.BigChainID())
	}

	// Initialize Facilitator Service
	svc, err := service.NewFacilitatorService(service.Options{
		EVMClient:        evmClient,
		PrivateKeyHex:    cfg.EVM.PrivateKey,
		DefaultChainID:   cfg.BigChainID(),
		DefaultTokenAddr: cfg.EVM.TokenAddress,
		AutoPollReceipts: cfg.EVM.AutoPollReceipts,
	})
	if err != nil {
		slog.Error("failed to initialize Facilitator service", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("facilitator service ready",
		slog.String("facilitator_address", svc.FacilitatorAddress().Hex()),
	)

	// Initialize HTTP Server
	srv := server.New(cfg, svc)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			slog.Error("HTTP server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("shutting down Facilitator server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("facilitator server stopped cleanly")
}
