package ledger

import (
	"context"
	"log/slog"
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
	slog.Info("Starting ledger streamer", "start_ledger", startLedger)

	// Prepare unbounded range (streaming mode)
	ledgerRange := ledgerbackend.UnboundedRange(startLedger)
	if err := s.backend.PrepareRange(ctx, ledgerRange); err != nil {
		slog.Error("Failed to prepare range", "error", err)
		return err
	}

	slog.Info("Backend prepared, streaming ledgers...")

	// Current ledger sequence to fetch
	currentSeq := startLedger

	// Main streaming loop
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			slog.Warn("Context cancelled, stopping streamer")
			return ctx.Err()
		default:
		}

		// Get the ledger
		startTime := time.Now()
		ledger, err := s.backend.GetLedger(ctx, currentSeq)
		if err != nil {
			slog.Error("Failed to get ledger", "sequence", currentSeq, "error", err)
			return err
		}

		fetchDuration := time.Since(startTime)

		// Process the ledger
		if err := s.processor.Process(ledger); err != nil {
			slog.Error("Failed to process ledger", "sequence", currentSeq, "error", err)
			return err
		}

		processDuration := time.Since(startTime)

		// Log timing info every 10 ledgers in INFO, always in DEBUG
		if currentSeq%10 == 0 {
			slog.Info("Ledger processed",
				"sequence", currentSeq,
				"fetch_ms", fetchDuration.Milliseconds(),
				"total_ms", processDuration.Milliseconds(),
			)
		} else {
			slog.Debug("Ledger processed",
				"sequence", currentSeq,
				"fetch_ms", fetchDuration.Milliseconds(),
				"total_ms", processDuration.Milliseconds(),
			)
		}

		// Move to next ledger
		currentSeq++

		// TODO: Add checkpoint here to save progress
	}
}

// Stop gracefully stops the streamer
func (s *Streamer) Stop() error {
	slog.Info("Stopping streamer...")
	if err := s.backend.Close(); err != nil {
		slog.Error("Failed to close backend", "error", err)
		return err
	}
	slog.Info("Streamer stopped")
	return nil
}
