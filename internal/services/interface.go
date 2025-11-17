package services

import (
	"context"
	"time"

	"github.com/stellar/go/ingest"
)

// ProcessedTx represents a normalized transaction ready for service processing
type ProcessedTx struct {
	// Transaction data
	Tx              ingest.LedgerTransaction
	Hash            string
	LedgerSeq       uint32
	LedgerCloseTime time.Time // Actual ledger close timestamp
	Success         bool
	IsSoroban       bool

	// Extracted data for easy filtering
	ContractIDs []string // All contract IDs from footprint
}

// Service defines the interface that all specialized services must implement
type Service interface {
	// Process handles a single transaction
	// Returns error only for critical failures that should stop the indexer
	// Returns nil if processing succeeded or if transaction should be skipped
	// Note: tx is passed by reference (pointer) to avoid copying large structs
	Process(ctx context.Context, tx *ProcessedTx) error

	// Name returns the service name for logging
	Name() string
}

// Flushable is an optional interface for services that accumulate data per ledger
// Services implementing this interface will have FlushLedger called when the ledger changes
type Flushable interface {
	// FlushLedger is called when moving to a new ledger
	// Services should save accumulated data and reset their state
	FlushLedger(ctx context.Context) error
}
