package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Throughput metrics - Track processing volume
var (
	LedgersProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "indexer_ledgers_processed_total",
		Help: "Total number of ledgers processed",
	})

	TransactionsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "indexer_transactions_processed_total",
		Help: "Total number of transactions processed",
	})

	DeploymentsDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "indexer_deployments_detected_total",
		Help: "Total number of contract deployments detected",
	})

	EventsSaved = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "indexer_events_saved_total",
			Help: "Total number of events saved by type",
		},
		[]string{"event_type"},
	)

	StorageChangesSaved = promauto.NewCounter(prometheus.CounterOpts{
		Name: "indexer_storage_changes_saved_total",
		Help: "Total number of storage changes saved",
	})
)

// Performance metrics - Track processing speed and latency
var (
	LedgerProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "indexer_ledger_processing_duration_seconds",
		Help:    "Time taken to process a single ledger",
		Buckets: prometheus.DefBuckets,
	})

	DatabaseBatchInsertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "indexer_db_batch_insert_duration_seconds",
		Help:    "Time taken to execute batch INSERT operations",
		Buckets: prometheus.DefBuckets,
	})

	CompactorFlushDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "indexer_compactor_flush_duration_seconds",
		Help:    "Time taken to flush and compact storage changes",
		Buckets: prometheus.DefBuckets,
	})
)

// State metrics - Track current system state
var (
	CurrentLedger = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_current_ledger",
		Help: "Current ledger sequence being processed",
	})

	TrackedContracts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_tracked_contracts",
		Help: "Number of contracts currently being tracked",
	})

	BufferSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_buffer_size",
		Help: "Configured buffer size for RPC ledger retrieval",
	})
)

// Optimization metrics - Track effectiveness of optimizations
var (
	CompactorReductionPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_compactor_reduction_percent",
		Help: "Percentage reduction achieved by ChangeCompactor (0-100)",
	})

	BatchInsertSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "indexer_batch_insert_size",
		Help:    "Number of items in each batch INSERT operation",
		Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
	})
)

// Error metrics - Track failures
var (
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "indexer_errors_total",
			Help: "Total number of errors by service",
		},
		[]string{"service"},
	)
)

// Pipeline metrics - Track parallel processing pipeline
var (
	PipelineMode = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_pipeline_mode",
		Help: "Pipeline mode: 0=sequential, 1=parallel",
	})

	PipelineWorkerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_pipeline_worker_count",
		Help: "Number of active pipeline workers",
	})

	PipelineQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_pipeline_queue_depth",
		Help: "Number of ledgers waiting to be checkpointed in order",
	})

	PipelineLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_pipeline_lag",
		Help: "Number of ledgers behind the latest ledger (current lag)",
	})
)
