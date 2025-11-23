package ingest

import "context"

// IngestService defines the interface for ledger ingestion services that process ledger data
type IngestService interface {
	Run(ctx context.Context, startLedger uint32, endLedger uint32) error
}
