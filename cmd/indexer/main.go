package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"indexer/internal/config"
	"indexer/internal/ledger"
	"indexer/internal/orchestrator"
	"indexer/internal/services"
	"indexer/internal/storage"

	"github.com/joho/godotenv"
	rpcclient "github.com/stellar/go/clients/rpcclient"
	"github.com/stellar/go/ingest/ledgerbackend"
)

func main() {
	fmt.Println("🌟 Starting Stellar Indexer...")

	// 1. Load configuration
	_ = godotenv.Load()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("❌ Invalid configuration: %v", err)
	}
	// Validar factory contract ID
	if cfg.FactoryContractID == "" {
		log.Fatal("❌ Factory Contract ID is required in config")
	}

	// 2. Configure logger
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Configuration loaded",
		"rpc_server", cfg.RPCServerURL,
		"network", cfg.NetworkPassphrase,
		"log_level", cfg.LogLevel,
	)

	// 3. Initialize database connection
	ctx := context.Background()
	repository, err := storage.NewPostgresRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer repository.Close()
	slog.Info("Database connected successfully")

	// 4. Create RPC client to get latest ledger
	rpcClient := rpcclient.NewClient(cfg.RPCServerURL, &http.Client{})

	// Get latest ledger from RPC if not specified in config
	startLedger := cfg.StartLedger
	if startLedger == 0 {
		health, err := rpcClient.GetHealth(ctx)
		if err != nil {
			log.Fatalf("❌ Failed to get health from RPC: %v", err)
		}
		// Start from 10 ledgers before latest to be safe
		startLedger = health.LatestLedger - 10
		slog.Info("Starting from recent ledger",
			"latest", health.LatestLedger,
			"starting_from", startLedger,
		)
	}

	// 5. Create RPCLedgerBackend
	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: cfg.RPCServerURL,
		BufferSize:   cfg.BufferSize,
		HttpClient:   &http.Client{},
	})

	// 6. Create processor with database repository
	processor := ledger.NewProcessor(cfg.NetworkPassphrase, cfg.FactoryContractID, repository)

	// 6.5. PHASE 3: Create orchestrator with services (ACTIVE MODE)
	factoryService := services.NewFactoryService(cfg.FactoryContractID, cfg.NetworkPassphrase, repository)
	orch := orchestrator.New([]services.Service{
		factoryService,
	})
	processor.SetOrchestrator(orch)
	slog.Info("Orchestrator enabled in ACTIVE mode",
		"services", len(orch.Services()),
	)

	// 7. Create streamer
	streamer := ledger.NewStreamer(backend, processor)

	// 7. Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start streaming in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := streamer.Start(ctx, startLedger); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt or error
	select {
	case <-sigChan:
		slog.Warn("Interrupt received, shutting down...")
		cancel()
		if err := streamer.Stop(); err != nil {
			slog.Error("Error stopping streamer", "error", err)
		}
	case err := <-errChan:
		slog.Error("Streamer error", "error", err)
		os.Exit(1)
	}

	slog.Info("Indexer stopped")
}
