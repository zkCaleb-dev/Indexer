package services

import (
	"context"
	"log/slog"

	"indexer/internal/storage"
)

// EventService filters and processes events with specific prefixes (e.g., "tw_*")
type EventService struct {
	eventPrefix        string
	trackedContracts   map[string]bool
	repository         storage.Repository
}

// NewEventService creates a new EventService instance
func NewEventService(eventPrefix string, repository storage.Repository) *EventService {
	return &EventService{
		eventPrefix:      eventPrefix,
		trackedContracts: make(map[string]bool),
		repository:       repository,
	}
}

// Process handles event filtering and extraction
func (s *EventService) Process(ctx context.Context, tx *ProcessedTx) error {
	// TODO: Extract events from transaction
	// TODO: Filter by prefix (e.g., "tw_*")
	// TODO: Verify event belongs to tracked contract
	// TODO: Save to database

	// For now, just log
	slog.Debug("EventService: Processing transaction (stub mode)",
		"tx_hash", tx.Hash,
		"ledger", tx.LedgerSeq,
		"prefix_filter", s.eventPrefix,
	)

	return nil
}

// Name returns the service name
func (s *EventService) Name() string {
	return "EventService"
}

// AddTrackedContract adds a contract ID to track
func (s *EventService) AddTrackedContract(contractID string) {
	s.trackedContracts[contractID] = true
}
