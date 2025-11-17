package pipeline

import (
	"context"
	"io"
	"log/slog"
	"time"

	"indexer/internal/extraction"
	"indexer/internal/metrics"
	"indexer/internal/orchestrator"
	"indexer/internal/services"
	"indexer/internal/storage"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Worker processes ledgers in parallel
// Each worker has its own orchestrator and services to avoid race conditions
type Worker struct {
	id                int
	networkPassphrase string
	factoryContracts  map[string]string
	extractor         *extraction.DataExtractor

	// Each worker has its own services (thread-safe)
	factoryService       *services.FactoryService
	activityService      *services.ActivityService
	eventService         *services.EventService
	storageChangeService *services.StorageChangeService
	orchestrator         *orchestrator.Orchestrator
}

// NewWorker creates a new pipeline worker with its own service instances
func NewWorker(ctx context.Context, cfg WorkerConfig, repository storage.Repository) *Worker {
	// Create independent service instances for this worker
	factoryService := services.NewFactoryService(cfg.FactoryContracts, cfg.NetworkPassphrase, repository)
	activityService := services.NewActivityService(cfg.NetworkPassphrase, repository)
	eventService := services.NewEventService(cfg.NetworkPassphrase, repository)
	storageChangeService := services.NewStorageChangeService(cfg.NetworkPassphrase, repository)

	// Wire services together
	factoryService.SetActivityService(activityService)
	activityService.SetEventService(eventService)
	activityService.SetStorageChangeService(storageChangeService)

	// Load existing deployed contracts into tracking
	// Critical for pipeline workers to track contracts deployed before pipeline started
	contractIDs, err := repository.GetTrackedContractIDs(ctx)
	if err != nil {
		slog.Warn("Worker: Failed to load tracked contracts",
			"worker_id", cfg.WorkerID,
			"error", err,
		)
	} else {
		for _, contractID := range contractIDs {
			activityService.AddTrackedContract(contractID)
		}
		slog.Debug("Worker: Loaded existing contracts into tracking",
			"worker_id", cfg.WorkerID,
			"count", len(contractIDs),
		)
	}

	// Create orchestrator with all services
	orch := orchestrator.New([]services.Service{
		factoryService,
		activityService,
		eventService,
		storageChangeService,
	})

	return &Worker{
		id:                   cfg.WorkerID,
		networkPassphrase:    cfg.NetworkPassphrase,
		factoryContracts:     cfg.FactoryContracts,
		extractor:            extraction.NewDataExtractor(cfg.NetworkPassphrase),
		factoryService:       factoryService,
		activityService:      activityService,
		eventService:         eventService,
		storageChangeService: storageChangeService,
		orchestrator:         orch,
	}
}

// ProcessLedger processes a single ledger and returns the extracted data
// This method does NOT save to database - that's done by the orderer
func (w *Worker) ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) (*ProcessedLedgerData, error) {
	start := time.Now()
	sequence := ledger.LedgerSequence()
	txCount := ledger.CountTransactions()
	ledgerCloseTime := ledger.ClosedAt()

	slog.Debug("Worker processing ledger",
		"worker_id", w.id,
		"sequence", sequence,
		"tx_count", txCount,
	)

	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		w.networkPassphrase,
		ledger,
	)
	if err != nil {
		slog.Error("Worker: Failed to create transaction reader",
			"worker_id", w.id,
			"sequence", sequence,
			"error", err,
		)
		return nil, err
	}
	defer reader.Close()

	txIndex := 0
	sorobanCount := 0

	// Process all transactions through orchestrator
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break
		}
		txIndex++

		if !tx.Successful() || !tx.IsSorobanTx() {
			continue
		}

		sorobanCount++
		metrics.TransactionsProcessed.Inc()

		// Process through orchestrator (services accumulate data)
		processedTx := &services.ProcessedTx{
			Tx:              tx,
			Hash:            tx.Hash.HexString(),
			LedgerSeq:       sequence,
			LedgerCloseTime: ledgerCloseTime,
			Success:         tx.Successful(),
			IsSoroban:       tx.IsSorobanTx(),
			ContractIDs:     extraction.ExtractAllContractIDs(tx),
		}

		if err := w.orchestrator.ProcessTx(ctx, processedTx); err != nil {
			slog.Error("Worker: Orchestrator processing failed",
				"worker_id", w.id,
				"error", err,
			)
		}
	}

	// Flush storage change service to compact and save
	// Note: Services save directly to DB (postgres handles concurrency)
	if err := w.storageChangeService.FlushLedger(ctx); err != nil {
		slog.Error("Worker: Failed to flush storage changes",
			"worker_id", w.id,
			"sequence", sequence,
			"error", err,
		)
	}

	processingTime := time.Since(start)

	// Return metadata only - actual data was saved by services
	// The orderer will use this to track completion and save checkpoints in order
	result := &ProcessedLedgerData{
		Sequence:         sequence,
		CloseTime:        ledgerCloseTime,
		TransactionCount: txIndex,
		ProcessingTime:   processingTime,
		WorkerID:         w.id,

		// Data counts for metrics (actual data saved by services)
		DeploymentsCount:    0, // Could track this if needed
		EventsCount:         0,
		StorageChangesCount: 0,

		// Not needed since services save directly
		Deployments:     nil,
		Events:          nil,
		StorageChanges:  nil,
	}

	slog.Debug("Worker completed ledger",
		"worker_id", w.id,
		"sequence", sequence,
		"soroban_txs", sorobanCount,
		"duration_ms", processingTime.Milliseconds(),
	)

	return result, nil
}
