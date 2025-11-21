package ingest

import "context"

type IngestService interface {
	Run(ctx context.Context, startLedger uint32, endLedger uint32) error
}
