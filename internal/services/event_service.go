package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"indexer/internal/extraction"
	"indexer/internal/models"
	"indexer/internal/storage"
)

// EventService filters and processes events with specific prefixes (e.g., "tw_*")
type EventService struct {
	networkPassphrase string
	trackedContracts  map[string]bool
	mu                sync.RWMutex // Protects trackedContracts
	repository        storage.Repository
	extractor         *extraction.DataExtractor
	eventPrefix       string // Filter prefix (e.g., "tw_" for TrustlessWork events)
}

// NewEventService creates a new EventService instance
func NewEventService(networkPassphrase string, repository storage.Repository) *EventService {
	return &EventService{
		networkPassphrase: networkPassphrase,
		trackedContracts:  make(map[string]bool),
		repository:        repository,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
		eventPrefix:       "tw_", // TrustlessWork event prefix
	}
}

// Process handles event filtering and extraction for tracked contracts
func (s *EventService) Process(ctx context.Context, tx *ProcessedTx) error {
	// Check if any contract in the transaction is tracked
	// Uses cache (memory) with fallback to DB for robustness
	var trackedContractID string

	for _, contractID := range tx.ContractIDs {
		// 1. First check cache in memory (fast path)
		s.mu.RLock()
		inCache := s.trackedContracts[contractID]
		s.mu.RUnlock()

		if inCache {
			trackedContractID = contractID
			break
		}

		// 2. If not in cache, check database (fallback for robustness)
		exists, err := s.repository.ContractExists(ctx, contractID)
		if err != nil {
			slog.Error("EventService: Failed to check contract existence",
				"error", err,
				"contract_id", contractID,
			)
			continue
		}

		if exists {
			// Add to cache for future transactions (auto-healing)
			s.mu.Lock()
			s.trackedContracts[contractID] = true
			s.mu.Unlock()

			slog.Info("EventService: Contract found in DB, added to cache (auto-healing)",
				"contract_id", contractID,
			)

			trackedContractID = contractID
			break
		}
	}

	if trackedContractID == "" {
		return nil // No deployed contracts in this transaction
	}

	// Extract ALL events from the transaction
	allEvents, err := s.extractor.ExtractEvents(tx.Tx, tx.LedgerSeq, tx.LedgerCloseTime)
	if err != nil {
		slog.Error("EventService: Failed to extract events",
			"error", err,
			"tx_hash", tx.Hash,
		)
		return err
	}

	// Filter events:
	// 1. Must be from the tracked contract
	// 2. Must have event_type starting with the prefix (e.g., "tw_")
	var filteredEvents []models.ContractEvent
	for _, event := range allEvents {
		// Verify event is from the tracked contract
		if event.ContractID != trackedContractID {
			continue
		}

		// Verify event type has the correct prefix
		if !strings.HasPrefix(event.EventType, s.eventPrefix) {
			slog.Debug("EventService: Skipping non-TrustlessWork event",
				"event_type", event.EventType,
				"contract_id", event.ContractID,
				"tx_hash", tx.Hash,
			)
			continue
		}

		filteredEvents = append(filteredEvents, event)
	}

	if len(filteredEvents) == 0 {
		return nil // No TrustlessWork events in this transaction
	}

	// Save events to database
	if err := s.repository.SaveContractEvents(ctx, filteredEvents); err != nil {
		slog.Error("EventService: Failed to save events to database",
			"error", err,
			"contract_id", trackedContractID,
		)
		// Don't return error - continue processing even if DB save fails
	}

	// Log success with event types
	var eventTypes []string
	for _, event := range filteredEvents {
		eventTypes = append(eventTypes, event.EventType)
	}

	slog.Info("✅ EventService: Events saved",
		"contract_id", trackedContractID,
		"events_count", len(filteredEvents),
		"event_types", eventTypes,
		"tx_hash", tx.Hash,
	)

	return nil
}

// Name returns the service name
func (s *EventService) Name() string {
	return "EventService"
}

// AddTrackedContract adds a contract ID to the tracking list
func (s *EventService) AddTrackedContract(contractID string) {
	s.mu.Lock()
	s.trackedContracts[contractID] = true
	s.mu.Unlock()
	slog.Debug("EventService: Added contract to tracking", "contract_id", contractID)
}

// RemoveTrackedContract removes a contract ID from the tracking list
func (s *EventService) RemoveTrackedContract(contractID string) {
	s.mu.Lock()
	delete(s.trackedContracts, contractID)
	s.mu.Unlock()
	slog.Debug("EventService: Removed contract from tracking", "contract_id", contractID)
}

// GetTrackedCount returns the number of contracts being tracked
func (s *EventService) GetTrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trackedContracts)
}

// SetEventPrefix allows changing the event prefix filter (useful for testing)
func (s *EventService) SetEventPrefix(prefix string) {
	s.eventPrefix = prefix
}
