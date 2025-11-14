package orchestrator

import (
	"context"
	"log/slog"

	"indexer/internal/services"
)

// Orchestrator coordinates multiple services to process transactions
type Orchestrator struct {
	services []services.Service
}

// New creates a new Orchestrator with the given services
func New(services []services.Service) *Orchestrator {
	return &Orchestrator{
		services: services,
	}
}

// ProcessTx runs a transaction through all registered services
func (o *Orchestrator) ProcessTx(ctx context.Context, tx *services.ProcessedTx) error {
	slog.Debug("Orchestrator: Processing transaction",
		"tx_hash", tx.Hash,
		"ledger", tx.LedgerSeq,
		"services_count", len(o.services),
	)

	// Execute each service in order
	for _, service := range o.services {
		if err := service.Process(ctx, tx); err != nil {
			slog.Error("Service processing failed",
				"service", service.Name(),
				"tx_hash", tx.Hash,
				"error", err,
			)
			// Continue processing with other services even if one fails
			// Only critical errors should stop the indexer
		}
	}

	return nil
}

// Services returns the list of registered services (for inspection/testing)
func (o *Orchestrator) Services() []services.Service {
	return o.services
}
