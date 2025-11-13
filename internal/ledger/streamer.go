package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/stellar/go/ingest/ledgerbackend"
)

// Streamer continuously polls ledgers from the backend and processes them
type Streamer struct {
	backend   ledgerbackend.LedgerBackend
	processor *Processor
}

// NewStreamer creates a new Streamer instance
func NewStreamer(backend ledgerbackend.LedgerBackend, processor *Processor) *Streamer {
	return &Streamer{
		backend:   backend,
		processor: processor,
	}
}

// Start begins the streaming process from the given starting ledger
func (s *Streamer) Start(ctx context.Context, startLedger uint32) error {
	fmt.Printf("🚀 Starting streamer from ledger %d\n", startLedger)

	// Prepare unbounded range (streaming mode)
	ledgerRange := ledgerbackend.UnboundedRange(startLedger)
	if err := s.backend.PrepareRange(ctx, ledgerRange); err != nil {
		return fmt.Errorf("failed to prepare range: %w", err)
	}

	fmt.Println("✅ Backend prepared, starting to stream ledgers...")

	// Current ledger sequence to fetch
	currentSeq := startLedger

	// Main streaming loop
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			fmt.Println("⚠️  Context cancelled, stopping streamer")
			return ctx.Err()
		default:
		}

		// Get the ledger
		startTime := time.Now()
		ledger, err := s.backend.GetLedger(ctx, currentSeq)
		if err != nil {
			return fmt.Errorf("failed to get ledger %d: %w", currentSeq, err)
		}

		fetchDuration := time.Since(startTime)

		// Process the ledger
		if err := s.processor.Process(ledger); err != nil {
			return fmt.Errorf("failed to process ledger %d: %w", currentSeq, err)
		}

		processDuration := time.Since(startTime)

		// Log timing info
		fmt.Printf("⏱️  Ledger %d - Fetch: %v, Total: %v\n",
			currentSeq, fetchDuration, processDuration)

		// Move to next ledger
		currentSeq++

		// TODO: Add checkpoint here to save progress
	}
}

// Stop gracefully stops the streamer
func (s *Streamer) Stop() error {
	fmt.Println("🛑 Stopping streamer...")
	if err := s.backend.Close(); err != nil {
		return fmt.Errorf("failed to close backend: %w", err)
	}
	fmt.Println("✅ Streamer stopped")
	return nil
}
