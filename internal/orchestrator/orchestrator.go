package orchestrator

import (
	"context"
	"log/slog"

	"indexer/internal/services"
)

// Orchestrator coordinates multiple services to process transactions
type Orchestrator struct {
	services      []services.Service
	currentLedger uint32
}

// New creates a new Orchestrator with the given services
func New(services []services.Service) *Orchestrator {
	return &Orchestrator{
		services:      services,
		currentLedger: 0,
	}
}

// ProcessTx runs a transaction through all registered services
func (o *Orchestrator) ProcessTx(ctx context.Context, tx *services.ProcessedTx) error {
	// Check if we moved to a new ledger - flush services if so
	if o.currentLedger != 0 && o.currentLedger != tx.LedgerSeq {
		if err := o.flushLedger(ctx); err != nil {
			slog.Error("Orchestrator: Failed to flush ledger",
				"error", err,
				"ledger", o.currentLedger,
			)
			// Continue processing even if flush fails
		}
	}

	o.currentLedger = tx.LedgerSeq

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

// flushLedger calls FlushLedger on all services that implement Flushable
func (o *Orchestrator) flushLedger(ctx context.Context) error {
	for _, service := range o.services {
		// Check if service implements Flushable interface
		if flushable, ok := service.(services.Flushable); ok {
			if err := flushable.FlushLedger(ctx); err != nil {
				slog.Error("Service flush failed",
					"service", service.Name(),
					"error", err,
				)
				// Continue flushing other services even if one fails
			} else {
				slog.Debug("Service flushed successfully",
					"service", service.Name(),
					"ledger", o.currentLedger,
				)
			}
		}
	}
	return nil
}

// Services returns the list of registered services (for inspection/testing)
func (o *Orchestrator) Services() []services.Service {
	return o.services
}
