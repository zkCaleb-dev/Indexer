package ledger

import (
	"context"
	"log/slog"
	"time"

	"indexer/internal/ledger/retry"
	"indexer/internal/storage"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/xdr"
)

// Streamer continuously polls ledgers from the backend and processes them
type Streamer struct {
	backend            ledgerbackend.LedgerBackend
	processor          *Processor
	retryStrategy      retry.Strategy
	repository         storage.Repository
	checkpointInterval uint32 // Save progress every N ledgers (0 = disable)
}

// NewStreamer creates a new Streamer instance
func NewStreamer(backend ledgerbackend.LedgerBackend, processor *Processor, retryStrategy retry.Strategy, repository storage.Repository, checkpointInterval uint32) *Streamer {
	slog.Info("Streamer created",
		"retry_strategy", retryStrategy.Name(),
		"checkpoint_interval", checkpointInterval,
	)
	return &Streamer{
		backend:            backend,
		processor:          processor,
		retryStrategy:      retryStrategy,
		repository:         repository,
		checkpointInterval: checkpointInterval,
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

		startTime := time.Now()
		var ledger xdr.LedgerCloseMeta
		var fetchDuration time.Duration

		// Get the ledger with retry strategy
		err := s.retryStrategy.Execute(ctx, func() error {
			fetchStart := time.Now()
			l, err := s.backend.GetLedger(ctx, currentSeq)
			if err != nil {
				return err
			}
			ledger = l
			fetchDuration = time.Since(fetchStart)
			return nil
		})

		if err != nil {
			slog.Error("Failed to get ledger after retry", "sequence", currentSeq, "error", err)
			return err
		}

		// Process the ledger with retry strategy, propagating context
		err = s.retryStrategy.Execute(ctx, func() error {
			return s.processor.Process(ctx, ledger)
		})

		if err != nil {
			slog.Error("Failed to process ledger after retry", "sequence", currentSeq, "error", err)
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

		// Save checkpoint periodically (if enabled)
		if s.checkpointInterval > 0 && currentSeq%s.checkpointInterval == 0 {
			if err := s.repository.SaveProgress(ctx, currentSeq); err != nil {
				slog.Warn("Failed to save progress checkpoint",
					"ledger", currentSeq,
					"error", err,
				)
				// Don't fail the entire stream if checkpoint fails
			} else {
				slog.Info("Progress checkpoint saved", "ledger", currentSeq)
			}
		}

		// Move to next ledger
		currentSeq++
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
