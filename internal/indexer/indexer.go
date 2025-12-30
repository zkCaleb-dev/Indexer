package indexer

import (
	"fmt"
	"indexer/internal/service/ingest"
	"log"
	"os"
	"os/signal"
	"syscall"

	"indexer/internal/indexer/processors"
	"indexer/internal/integration/rpc_backend"
	"indexer/internal/service/rpc"
)

// Indexer is the main coordinator that manages the ledger backend, ingest service, and processors
type Indexer struct {
	ingestService *ingest.OrchestratorService
	processors    []ingest.Processor
}

// New creates a new indexer instance with the given configuration
func New() (*Indexer, error) {

	// Create RPC client configuration
	clientConfig := rpc_backend.ClientConfig{
		BufferSize: 25,
		TimeoutConfig: rpc_backend.ClientTimeoutConfig{
			Timeout:  30,
			Retries:  3,
			Interval: 5,
		},
	}

	// Create ledger backend
	ledgerBackend := &rpc.LedgerBackend{
		ClientConfig: clientConfig,
	}

	// Start the backend
	if err := ledgerBackend.Start(); err != nil {
		return nil, fmt.Errorf("error starting ledger backend: %w", err)
	}

	// Create processors
	usdcProcessor := processors.NewUSDCTransferProcessor()
	processorList := []ingest.Processor{usdcProcessor}

	// Create ingest service
	ingestService := ingest.NewIngestService(ledgerBackend, processorList)

	// Start background event consumer
	go consumeEvents(usdcProcessor)

	return &Indexer{
		ingestService: ingestService,
		processors:    processorList,
	}, nil
}

// Start initializes and runs the indexer, blocking until a termination signal is received
func (idx *Indexer) Start() error {
	log.Printf("🚀 Starting indexer with RPC: %s")

	// Start ingestion
	if err := idx.ingestService.StartUnboundedRange(0); err != nil {
		return fmt.Errorf("error starting ingest: %w", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	sig := <-sigChan
	log.Printf("📡 Signal received: %v", sig)

	// Stop services
	idx.Stop()

	return nil
}

// Stop gracefully shuts down the indexer by stopping the ingest service and closing the ledger backend
func (idx *Indexer) Stop() {
	log.Println("🛑 Stopping indexer...")

	// Stop ingestion
	idx.ingestService.Stop()

	log.Println("✅ Indexer stopped")
}

// consumeEvents continuously processes events from the processor's buffer channel
func consumeEvents(processor *processors.USDCTransferProcessor) {
	for event := range processor.GetBuffer() {
		// Currently just logging, will persist later
		log.Printf("📊 USDC event processed: %+v", event)
		// TODO: Add persistence logic to MongoDB here
	}
}
