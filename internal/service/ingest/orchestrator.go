package ingest

import (
	"context"
	"fmt"
	"indexer/internal/service/rpc"
	"log"
	"sync"
	"time"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/network"
)

// OrchestratorService coordinates the ingestion of ledgers from the Stellar network
type OrchestratorService struct {
	ledgerBackend rpc.LedgerBackendHandlerService
	processors    []Processor
	checkpointMgr CheckpointStore

	// Lifecycle control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewIngestService creates a new orchestrator service for ledger ingestion
func NewIngestService(ledgerBackend rpc.LedgerBackendHandlerService, processors []Processor) *OrchestratorService {
	ctx, cancel := context.WithCancel(context.Background())

	return &OrchestratorService{
		ledgerBackend: ledgerBackend,
		processors:    processors,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the ledger ingestion process from the specified starting ledger
func (s *OrchestratorService) StartUnboundedRange(startLedger uint32) error {
	log.Printf("🚀 Starting ingestion from ledger %d", startLedger)

	// Prepare unbounded range for continuous streaming
	if err := s.ledgerBackend.PrepareRange(s.ctx, &startLedger, nil); err != nil {
		return fmt.Errorf("error preparing ledger range: %w", err)
	}

	s.wg.Add(1)
	go s.ingestLoop(startLedger)

	return nil
}

// ingestLoop is the main ingestion loop that continuously processes ledgers
func (s *OrchestratorService) ingestLoop(startLedger uint32) {
	defer s.wg.Done()

	currentLedger := startLedger
	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("⏹️  Stopping ingestion...")
			return

		case <-ticker.C:
			// Attempt to process the next ledger
			if err := s.processLedger(currentLedger); err != nil {
				consecutiveErrors++
				log.Printf("❌ Error processing ledger %d (attempt %d/%d): %v",
					currentLedger, consecutiveErrors, maxConsecutiveErrors, err)

				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("🔴 Too many consecutive errors, stopping...")
					return
				}

				// Exponential backoff
				time.Sleep(time.Duration(consecutiveErrors) * time.Second)
				continue
			}

			// Success - reset counter and advance
			consecutiveErrors = 0
			log.Printf("✅ Ledger %d processed successfully", currentLedger)
			currentLedger++
		}
	}
}

// processLedger processes an individual ledger and its transactions
func (s *OrchestratorService) processLedger(sequence uint32) error {
	// Get the backend instance
	backend, err := s.ledgerBackend.HandleBackend()
	if err != nil {
		return fmt.Errorf("error getting backend: %w", err)
	}

	// Fetch ledger from backend
	ledger, err := backend.GetLedger(s.ctx, sequence)
	if err != nil {
		return fmt.Errorf("error fetching ledger: %w", err)
	}

	// Create transaction reader
	txReader, err := ingest.NewLedgerTransactionReader(
		s.ctx,
		backend,
		network.TestNetworkPassphrase,
		sequence,
	)
	if err != nil {
		return fmt.Errorf("error creating transaction reader: %w", err)
	}
	defer txReader.Close()

	// Process the ledger with each processor
	for _, processor := range s.processors {
		if err := processor.ProcessLedger(s.ctx, ledger); err != nil {
			log.Printf("⚠️  Processor %s failed on ledger: %v", processor.Name(), err)
			// Continue with other processors
		}
	}

	// Iterate through transactions
	for {
		tx, err := txReader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break // End of transactions
			}
			return fmt.Errorf("error reading transaction: %w", err)
		}

		// Process transaction with each processor
		for _, processor := range s.processors {
			if err := processor.ProcessTransaction(s.ctx, tx); err != nil {
				log.Printf("⚠️  Processor %s failed on transaction: %v", processor.Name(), err)
				// Continue with other processors
			}
		}
	}

	return nil
}

// Stop gracefully stops the ingestion service
func (s *OrchestratorService) Stop() {
	log.Println("🛑 Requesting ingestion shutdown...")
	s.cancel()
	s.wg.Wait()
	log.Println("✅ Ingestion stopped")
}
