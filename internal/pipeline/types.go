package pipeline

import (
	"time"

	"indexer/internal/models"
)

// ProcessedLedgerData contains all extracted and processed data from a ledger
// This struct is passed from workers to the orderer for sequential saving
type ProcessedLedgerData struct {
	// Ledger metadata
	Sequence        uint32
	CloseTime       time.Time
	TransactionCount int

	// Extracted data (already processed through services)
	Deployments     []*models.DeployedContract
	Events          []models.ContractEvent
	StorageChanges  []*models.StorageChange

	// Processing metrics
	ProcessingTime  time.Duration
	WorkerID        int

	// Counts for logging
	DeploymentsCount    int
	EventsCount         int
	StorageChangesCount int
}

// WorkerConfig contains configuration for a pipeline worker
type WorkerConfig struct {
	WorkerID          int
	NetworkPassphrase string
	FactoryContracts  map[string]string
}

// PipelineConfig contains configuration for the entire pipeline
type PipelineConfig struct {
	Enabled                  bool
	WorkerCount              int
	ResultsBufferSize        int
	AutoEnableLagThreshold   uint32
	AutoDisableLagThreshold  uint32
}

// PipelineMode represents the current mode of the pipeline
type PipelineMode string

const (
	ModeSequential PipelineMode = "sequential" // Normal mode, no parallelization
	ModeParallel   PipelineMode = "parallel"   // Parallel processing enabled
)
