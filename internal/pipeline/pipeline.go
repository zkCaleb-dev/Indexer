package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"indexer/internal/metrics"
	"indexer/internal/storage"

	rpcclient "github.com/stellar/go/clients/rpcclient"
	"github.com/stellar/go/xdr"
)

// Pipeline manages parallel ledger processing with auto-enable based on lag
type Pipeline struct {
	config     PipelineConfig
	repository storage.Repository
	rpcClient  *rpcclient.Client

	// Workers and orderer
	workers []*Worker
	orderer *Orderer

	// State
	mu          sync.RWMutex
	currentMode PipelineMode
	isRunning   bool

	// Channels for parallel mode
	ledgerChan  chan xdr.LedgerCloseMeta
	resultsChan chan *ProcessedLedgerData
	errorChan   chan error
}

// NewPipeline creates a new pipeline instance
func NewPipeline(config PipelineConfig, repository storage.Repository, rpcClient *rpcclient.Client) *Pipeline {
	return &Pipeline{
		config:      config,
		repository:  repository,
		rpcClient:   rpcClient,
		currentMode: ModeSequential,
		isRunning:   false,
	}
}

// ShouldEnableParallel determines if parallel mode should be enabled based on lag
func (p *Pipeline) ShouldEnableParallel(ctx context.Context, currentLedger uint32) (bool, error) {
	// Manual override - always enabled
	if p.config.Enabled && p.config.AutoEnableLagThreshold == 0 {
		return true, nil
	}

	// Get latest ledger from RPC
	health, err := p.rpcClient.GetHealth(ctx)
	if err != nil {
		slog.Warn("Pipeline: Failed to get RPC health for lag detection",
			"error", err,
		)
		// If we can't determine lag, keep current mode
		p.mu.RLock()
		defer p.mu.RUnlock()
		return p.currentMode == ModeParallel, nil
	}

	latestLedger := health.LatestLedger
	lag := latestLedger - currentLedger

	// Update lag metric
	metrics.PipelineLag.Set(float64(lag))

	p.mu.Lock()
	previousMode := p.currentMode
	p.mu.Unlock()

	// Determine if we should switch modes
	shouldEnable := false

	if previousMode == ModeParallel {
		// Currently in parallel mode - check if we should disable
		if lag < p.config.AutoDisableLagThreshold {
			slog.Info("🔄 Pipeline auto-disabling: caught up with network",
				"lag", lag,
				"threshold", p.config.AutoDisableLagThreshold,
				"current_ledger", currentLedger,
				"latest_ledger", latestLedger,
			)
			shouldEnable = false
		} else {
			shouldEnable = true
		}
	} else {
		// Currently in sequential mode - check if we should enable
		if lag > p.config.AutoEnableLagThreshold {
			slog.Info("🚀 Pipeline auto-enabling: catching up",
				"lag", lag,
				"threshold", p.config.AutoEnableLagThreshold,
				"current_ledger", currentLedger,
				"latest_ledger", latestLedger,
			)
			shouldEnable = true
		} else {
			shouldEnable = false
		}
	}

	return shouldEnable, nil
}

// StartParallel starts the pipeline in parallel mode
func (p *Pipeline) StartParallel(ctx context.Context, workerConfig WorkerConfig, checkpointInterval uint32, startLedger uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return fmt.Errorf("pipeline is already running")
	}

	// Determine worker count
	workerCount := p.config.WorkerCount
	if workerCount == 0 {
		cpuCores := runtime.NumCPU()
		workerCount = int(float64(cpuCores) * 0.75) // Use 75% of cores
		if workerCount < 2 {
			workerCount = 2
		}
	}

	slog.Info("🚀 Starting parallel pipeline",
		"worker_count", workerCount,
		"buffer_size", p.config.ResultsBufferSize,
		"cpu_cores", runtime.NumCPU(),
	)

	// Create workers
	p.workers = make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		cfg := workerConfig
		cfg.WorkerID = i
		p.workers[i] = NewWorker(ctx, cfg, p.repository)
	}

	// Create orderer
	p.orderer = NewOrderer(p.repository, startLedger, checkpointInterval)

	// Create channels
	p.ledgerChan = make(chan xdr.LedgerCloseMeta, p.config.ResultsBufferSize)
	p.resultsChan = make(chan *ProcessedLedgerData, p.config.ResultsBufferSize)
	p.errorChan = make(chan error, workerCount)

	// Start workers
	var wg sync.WaitGroup
	for _, worker := range p.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			p.runWorker(ctx, w)
		}(worker)
	}

	// Start orderer
	go p.runOrderer(ctx)

	// Wait for workers to finish (in background)
	go func() {
		wg.Wait()
		close(p.resultsChan)
	}()

	p.currentMode = ModeParallel
	p.isRunning = true

	// Update metrics
	metrics.PipelineMode.Set(1) // 1 = parallel
	metrics.PipelineWorkerCount.Set(float64(workerCount))

	return nil
}

// runWorker runs a single worker goroutine
func (p *Pipeline) runWorker(ctx context.Context, worker *Worker) {
	for {
		select {
		case <-ctx.Done():
			return
		case ledger, ok := <-p.ledgerChan:
			if !ok {
				return
			}

			result, err := worker.ProcessLedger(ctx, ledger)
			if err != nil {
				slog.Error("Worker processing failed",
					"worker_id", worker.id,
					"sequence", ledger.LedgerSequence(),
					"error", err,
				)
				p.errorChan <- err
				continue
			}

			// Send result to orderer
			select {
			case p.resultsChan <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

// runOrderer runs the orderer goroutine
func (p *Pipeline) runOrderer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-p.resultsChan:
			if !ok {
				return
			}

			if err := p.orderer.ProcessResult(ctx, result); err != nil {
				slog.Error("Orderer processing failed",
					"sequence", result.Sequence,
					"error", err,
				)
			}
		}
	}
}

// SubmitLedger submits a ledger for processing (parallel mode)
func (p *Pipeline) SubmitLedger(ledger xdr.LedgerCloseMeta) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isRunning {
		return fmt.Errorf("pipeline is not running")
	}

	select {
	case p.ledgerChan <- ledger:
		return nil
	default:
		return fmt.Errorf("pipeline ledger channel is full")
	}
}

// Stop stops the parallel pipeline
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return
	}

	slog.Info("🛑 Stopping parallel pipeline")

	close(p.ledgerChan)
	p.isRunning = false
	p.currentMode = ModeSequential

	// Update metrics
	metrics.PipelineMode.Set(0) // 0 = sequential
	metrics.PipelineWorkerCount.Set(0)
	metrics.PipelineQueueDepth.Set(0)
}

// GetMode returns the current pipeline mode
func (p *Pipeline) GetMode() PipelineMode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentMode
}

// IsRunning returns whether the pipeline is currently running
func (p *Pipeline) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}
