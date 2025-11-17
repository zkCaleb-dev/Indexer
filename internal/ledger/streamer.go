package ledger

import (
	"context"
	"log/slog"
	"time"

	"indexer/internal/ledger/retry"
	"indexer/internal/pipeline"
	"indexer/internal/storage"

	rpcclient "github.com/stellar/go/clients/rpcclient"
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

	// Parallel processing pipeline (optional)
	pipeline  *pipeline.Pipeline
	rpcClient *rpcclient.Client
}

// NewStreamer creates a new Streamer instance with optional pipeline support
func NewStreamer(
	backend ledgerbackend.LedgerBackend,
	processor *Processor,
	retryStrategy retry.Strategy,
	repository storage.Repository,
	checkpointInterval uint32,
	pipelineInstance *pipeline.Pipeline,
	rpcClient *rpcclient.Client,
) *Streamer {
	pipelineMode := "disabled"
	if pipelineInstance != nil {
		pipelineMode = "enabled (auto-detect)"
	}

	slog.Info("Streamer created",
		"retry_strategy", retryStrategy.Name(),
		"checkpoint_interval", checkpointInterval,
		"pipeline", pipelineMode,
	)
	return &Streamer{
		backend:            backend,
		processor:          processor,
		retryStrategy:      retryStrategy,
		repository:         repository,
		checkpointInterval: checkpointInterval,
		pipeline:           pipelineInstance,
		rpcClient:          rpcClient,
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
			// Stop pipeline if running
			if s.pipeline != nil && s.pipeline.IsRunning() {
				s.pipeline.Stop()
			}
			return ctx.Err()
		default:
		}

		// Check if we should enable/disable parallel mode (every 50 ledgers)
		if s.pipeline != nil && currentSeq%50 == 0 {
			shouldEnable, err := s.pipeline.ShouldEnableParallel(ctx, currentSeq)
			if err == nil && shouldEnable != s.pipeline.IsRunning() {
				if shouldEnable {
					// Start parallel mode
					workerConfig := pipeline.WorkerConfig{
						NetworkPassphrase: s.processor.GetNetworkPassphrase(),
						FactoryContracts:  s.processor.GetFactoryContracts(),
					}
					if err := s.pipeline.StartParallel(ctx, workerConfig, s.checkpointInterval, currentSeq); err != nil {
						slog.Error("Failed to start parallel pipeline", "error", err)
					}
				} else {
					// Stop parallel mode
					s.pipeline.Stop()
				}
			}
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

		// Route ledger to pipeline or processor based on mode
		if s.pipeline != nil && s.pipeline.IsRunning() {
			// Parallel mode - submit to pipeline
			if err := s.pipeline.SubmitLedger(ledger); err != nil {
				slog.Warn("Pipeline submission failed, falling back to sequential",
					"sequence", currentSeq,
					"error", err,
				)
				// Fallback to sequential processing
				err = s.retryStrategy.Execute(ctx, func() error {
					return s.processor.Process(ctx, ledger)
				})
				if err != nil {
					slog.Error("Failed to process ledger after retry", "sequence", currentSeq, "error", err)
					return err
				}
			}
		} else {
			// Sequential mode - process directly
			err = s.retryStrategy.Execute(ctx, func() error {
				return s.processor.Process(ctx, ledger)
			})

			if err != nil {
				slog.Error("Failed to process ledger after retry", "sequence", currentSeq, "error", err)
				return err
			}

			// Save checkpoint in sequential mode (pipeline handles its own checkpointing)
			if s.checkpointInterval > 0 && currentSeq%s.checkpointInterval == 0 {
				if err := s.repository.SaveProgress(ctx, currentSeq); err != nil {
					slog.Warn("Failed to save progress checkpoint",
						"ledger", currentSeq,
						"error", err,
					)
				} else {
					slog.Info("Progress checkpoint saved", "ledger", currentSeq)
				}
			}
		}

		processDuration := time.Since(startTime)

		// Log timing info every 10 ledgers in INFO, always in DEBUG
		mode := "sequential"
		if s.pipeline != nil && s.pipeline.IsRunning() {
			mode = "parallel"
		}

		if currentSeq%10 == 0 {
			slog.Info("Ledger processed",
				"sequence", currentSeq,
				"mode", mode,
				"fetch_ms", fetchDuration.Milliseconds(),
				"total_ms", processDuration.Milliseconds(),
			)
		} else {
			slog.Debug("Ledger processed",
				"sequence", currentSeq,
				"mode", mode,
				"fetch_ms", fetchDuration.Milliseconds(),
				"total_ms", processDuration.Milliseconds(),
			)
		}

		// Move to next ledger
		currentSeq++
	}
}

// Stop gracefully stops the streamer
func (s *Streamer) Stop() error {
	slog.Info("Stopping streamer...")

	// Stop pipeline if running
	if s.pipeline != nil && s.pipeline.IsRunning() {
		s.pipeline.Stop()
	}

	if err := s.backend.Close(); err != nil {
		slog.Error("Failed to close backend", "error", err)
		return err
	}
	slog.Info("Streamer stopped")
	return nil
}
