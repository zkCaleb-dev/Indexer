package pipeline

import (
	"context"
	"log/slog"
	"time"

	"indexer/internal/metrics"
	"indexer/internal/storage"
)

// Orderer receives processed ledger results and ensures they are checkpointed in sequential order
// Even though workers process ledgers in parallel, we need to save checkpoints sequentially
// to ensure we can resume from the correct ledger after a restart
type Orderer struct {
	repository         storage.Repository
	checkpointInterval uint32

	// State tracking
	nextExpected uint32                            // Next ledger we expect to checkpoint
	pending      map[uint32]*ProcessedLedgerData // Buffered out-of-order results
	lastCheckpoint time.Time
}

// NewOrderer creates a new orderer for sequential checkpoint saving
func NewOrderer(repository storage.Repository, startLedger uint32, checkpointInterval uint32) *Orderer {
	return &Orderer{
		repository:         repository,
		checkpointInterval: checkpointInterval,
		nextExpected:       startLedger,
		pending:            make(map[uint32]*ProcessedLedgerData),
		lastCheckpoint:     time.Now(),
	}
}

// ProcessResult processes a ledger result from a worker
// Ensures checkpoints are saved in sequential order
func (o *Orderer) ProcessResult(ctx context.Context, result *ProcessedLedgerData) error {
	// Add to pending buffer
	o.pending[result.Sequence] = result

	slog.Debug("Orderer received result",
		"sequence", result.Sequence,
		"worker_id", result.WorkerID,
		"pending_count", len(o.pending),
		"next_expected", o.nextExpected,
	)

	// Process all sequential ledgers starting from nextExpected
	for {
		data, exists := o.pending[o.nextExpected]
		if !exists {
			// Next expected ledger hasn't been processed yet
			break
		}

		// Process this ledger (save checkpoint if needed)
		if err := o.processInOrder(ctx, data); err != nil {
			slog.Error("Orderer: Failed to process ledger in order",
				"sequence", data.Sequence,
				"error", err,
			)
			return err
		}

		// Remove from pending and move to next
		delete(o.pending, o.nextExpected)
		o.nextExpected++
	}

	// Update queue depth metric
	metrics.PipelineQueueDepth.Set(float64(len(o.pending)))

	return nil
}

// processInOrder handles a single ledger result in sequential order
func (o *Orderer) processInOrder(ctx context.Context, data *ProcessedLedgerData) error {
	// Note: Data was already saved to DB by worker's services
	// We only need to save checkpoint progress

	// Save checkpoint if interval reached
	if o.checkpointInterval > 0 && data.Sequence%o.checkpointInterval == 0 {
		if err := o.repository.SaveProgress(ctx, data.Sequence); err != nil {
			slog.Warn("Orderer: Failed to save checkpoint",
				"sequence", data.Sequence,
				"error", err,
			)
			// Don't return error - continue processing
		} else {
			timeSinceLastCheckpoint := time.Since(o.lastCheckpoint)
			o.lastCheckpoint = time.Now()

			slog.Info("📊 Checkpoint saved",
				"sequence", data.Sequence,
				"time_since_last", timeSinceLastCheckpoint.Round(time.Second),
			)
		}
	}

	// Update metrics
	metrics.CurrentLedger.Set(float64(data.Sequence))

	slog.Debug("Orderer processed ledger",
		"sequence", data.Sequence,
		"worker_id", data.WorkerID,
		"processing_time_ms", data.ProcessingTime.Milliseconds(),
	)

	return nil
}

// GetPendingCount returns the number of ledgers waiting to be checkpointed
func (o *Orderer) GetPendingCount() int {
	return len(o.pending)
}

// GetNextExpected returns the next ledger sequence we're waiting for
func (o *Orderer) GetNextExpected() uint32 {
	return o.nextExpected
}
