package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"indexer/internal/config"
	"indexer/internal/ledger"

	"github.com/joho/godotenv"
	rpcclient "github.com/stellar/go/clients/rpcclient"
	"github.com/stellar/go/ingest/ledgerbackend"
)

func main() {
	fmt.Println("🌟 Starting Stellar Indexer...")

	// 1. Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("❌ Invalid configuration: %v", err)
	}
	_ = godotenv.Load()

	fmt.Printf("📡 RPC Server: %s\n", cfg.RPCServerURL)
	fmt.Printf("🌐 Network: %s\n", cfg.NetworkPassphrase)

	// 2. Create RPC client to get latest ledger
	rpcClient := rpcclient.NewClient(cfg.RPCServerURL, &http.Client{})
	ctx := context.Background()

	// Get latest ledger from RPC if not specified in config
	startLedger := cfg.StartLedger
	if startLedger == 0 {
		health, err := rpcClient.GetHealth(ctx)
		if err != nil {
			log.Fatalf("❌ Failed to get health from RPC: %v", err)
		}
		// Start from 10 ledgers before latest to be safe
		startLedger = health.LatestLedger - 10
		fmt.Printf("📊 Latest ledger: %d, starting from: %d\n", health.LatestLedger, startLedger)
	}

	// 3. Create RPCLedgerBackend
	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: cfg.RPCServerURL,
		BufferSize:   cfg.BufferSize,
		HttpClient:   &http.Client{},
	})

	// 4. Create processor
	processor := ledger.NewProcessor(cfg.NetworkPassphrase)

	// 5. Create streamer
	streamer := ledger.NewStreamer(backend, processor)

	// 6. Setup graceful shutdown
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
		fmt.Println("\n⚠️  Interrupt received, shutting down...")
		cancel()
		if err := streamer.Stop(); err != nil {
			log.Printf("❌ Error stopping streamer: %v", err)
		}
	case err := <-errChan:
		log.Fatalf("❌ Streamer error: %v", err)
	}

	fmt.Println("👋 Indexer stopped")
}
