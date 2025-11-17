package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"indexer/internal/api"
	"indexer/internal/config"
	"indexer/internal/ledger"
	"indexer/internal/ledger/retry"
	"indexer/internal/metrics"
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
	// Validar factory contracts
	if len(cfg.FactoryContracts) == 0 {
		log.Fatal("❌ At least one factory contract is required in config")
	}

	// Build factory contracts map for easier lookup
	factoryMap := make(map[string]string)
	for _, factory := range cfg.FactoryContracts {
		factoryMap[factory.ID] = factory.Type
	}
	slog.Info("Factory contracts configured",
		"count", len(cfg.FactoryContracts),
		"types", factoryMap,
	)

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

	// 4. Create optimized HTTP client with connection pooling
	httpClient := cfg.NewHTTPClient()
	slog.Info("HTTP Client configured",
		"timeout_sec", cfg.HTTPTimeout,
		"max_idle_conns", cfg.HTTPMaxIdleConns,
		"max_conns_per_host", cfg.HTTPMaxConnsPerHost,
		"idle_timeout_sec", cfg.HTTPIdleConnTimeout,
	)

	// 5. Check for saved progress (checkpoint/resume)
	savedLedger, exists, err := repository.GetProgress(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to check progress: %v", err)
	}

	startLedger := cfg.StartLedger
	if exists {
		// Resume from saved checkpoint (+1 to start from next ledger)
		startLedger = savedLedger + 1
		slog.Info("Resuming from checkpoint",
			"last_processed", savedLedger,
			"resuming_from", startLedger,
		)
	} else if startLedger == 0 {
		// No checkpoint and no config - get latest from RPC
		rpcClient := rpcclient.NewClient(cfg.RPCServerURL, httpClient)
		health, err := rpcClient.GetHealth(ctx)
		if err != nil {
			log.Fatalf("❌ Failed to get health from RPC: %v", err)
		}
		// Start from 10 ledgers before latest to be safe
		startLedger = health.LatestLedger - 10
		slog.Info("Starting from recent ledger (no checkpoint)",
			"latest", health.LatestLedger,
			"starting_from", startLedger,
		)
	} else {
		slog.Info("Starting from configured ledger (no checkpoint)",
			"starting_from", startLedger,
		)
	}

	// 6. Create RPCLedgerBackend with shared HTTP client
	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: cfg.RPCServerURL,
		BufferSize:   cfg.BufferSize,
		HttpClient:   httpClient, // Use shared HTTP client with connection pooling
	})
	slog.Info("RPCLedgerBackend configured",
		"buffer_size", cfg.BufferSize,
		"estimated_buffer_time_min", float64(cfg.BufferSize)*5.0/60.0, // ~5 sec per ledger
		"estimated_memory_mb", cfg.BufferSize,                          // ~1MB per ledger
	)

	// 6. Create processor with database repository and factory map
	processor := ledger.NewProcessor(cfg.NetworkPassphrase, factoryMap, repository)

	// 6.5. Create orchestrator with all services (ACTIVE MODE)
	// Create all services
	factoryService := services.NewFactoryService(factoryMap, cfg.NetworkPassphrase, repository)
	activityService := services.NewActivityService(cfg.NetworkPassphrase, repository)
	eventService := services.NewEventService(cfg.NetworkPassphrase, repository)
	storageChangeService := services.NewStorageChangeService(cfg.NetworkPassphrase, repository)

	// Wire services together:
	// 1. FactoryService → ActivityService (notifies of new deployments)
	factoryService.SetActivityService(activityService)

	// 2. ActivityService → EventService + StorageChangeService (propagates tracking)
	activityService.SetEventService(eventService)
	activityService.SetStorageChangeService(storageChangeService)

	// Create orchestrator with all services in execution order
	orch := orchestrator.New([]services.Service{
		factoryService,        // 1. Detects deployments
		activityService,       // 2. Detects activity, updates tracking
		eventService,          // 3. Extracts and saves events (tw_* filtered)
		storageChangeService,  // 4. Extracts and saves storage changes
	})

	processor.SetOrchestrator(orch)
	slog.Info("Orchestrator enabled in ACTIVE mode",
		"services", len(orch.Services()),
	)

	// 7. Load retry configuration and create retry strategy
	retryConfig := retry.LoadConfig()
	retryStrategy := retry.NewStrategy(retryConfig)

	// 8. Create streamer with retry strategy and checkpointing
	streamer := ledger.NewStreamer(backend, processor, retryStrategy, repository, cfg.CheckpointInterval)
	slog.Info("Streamer configured",
		"checkpoint_interval", cfg.CheckpointInterval,
		"checkpoint_enabled", cfg.CheckpointInterval > 0,
	)

	// 9. Initialize metrics with static values
	metrics.BufferSize.Set(float64(cfg.BufferSize))

	// 10. Start API server for metrics and REST endpoints
	apiServer := api.NewServer(cfg.APIServerPort, repository)
	if err := apiServer.Start(); err != nil {
		log.Fatalf("❌ Failed to start API server: %v", err)
	}
	slog.Info("API server started successfully",
		"port", cfg.APIServerPort,
		"metrics_url", fmt.Sprintf("http://localhost:%d/metrics", cfg.APIServerPort),
		"health_url", fmt.Sprintf("http://localhost:%d/health", cfg.APIServerPort),
	)

	// 11. Setup graceful shutdown
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
		// Gracefully shutdown API server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error stopping API server", "error", err)
		}
	case err := <-errChan:
		slog.Error("Streamer error", "error", err)
		// Shutdown API server on error too
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		apiServer.Shutdown(shutdownCtx)
		os.Exit(1)
	}

	slog.Info("Indexer stopped")
}
