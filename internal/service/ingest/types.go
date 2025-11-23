package ingest

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Processor defines the interface for processing ledgers and transactions
type Processor interface {
	Name() string
	ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error
	ProcessTransaction(ctx context.Context, tx ingest.LedgerTransaction) error
}

// CheckpointStore defines the interface for managing ledger sequence checkpoints
type CheckpointStore interface {
	Save(ctx context.Context, ledgerSeq uint32) error
	Load(ctx context.Context) (uint32, error)
}
