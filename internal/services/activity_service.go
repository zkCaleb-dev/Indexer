package services

import (
	"context"
	"log/slog"
	"sync"

	"indexer/internal/extraction"
	"indexer/internal/storage"
)

// ActivityService tracks and processes activity on deployed contracts
type ActivityService struct {
	networkPassphrase string
	trackedContracts  map[string]bool
	mu                sync.RWMutex // Protects trackedContracts
	repository        storage.Repository
	extractor         *extraction.DataExtractor

	// Optional services for notifications
	eventService         *EventService
	storageChangeService *StorageChangeService
}

// NewActivityService creates a new ActivityService instance
func NewActivityService(networkPassphrase string, repository storage.Repository) *ActivityService {
	return &ActivityService{
		networkPassphrase: networkPassphrase,
		trackedContracts:  make(map[string]bool),
		repository:        repository,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
	}
}

// SetEventService sets the event service for notifications
func (s *ActivityService) SetEventService(eventService *EventService) {
	s.eventService = eventService
}

// SetStorageChangeService sets the storage change service for notifications
func (s *ActivityService) SetStorageChangeService(storageChangeService *StorageChangeService) {
	s.storageChangeService = storageChangeService
}

// Process handles tracked contract activity detection and extraction
func (s *ActivityService) Process(ctx context.Context, tx *ProcessedTx) error {
	// Check if any contract in the transaction is tracked
	s.mu.RLock()
	var trackedContractID string
	for _, contractID := range tx.ContractIDs {
		if s.trackedContracts[contractID] {
			trackedContractID = contractID
			break
		}
	}
	s.mu.RUnlock()

	if trackedContractID == "" {
		return nil // No tracked contracts in this transaction
	}

	slog.Info("Tracked contract activity",
		"contract_id", trackedContractID,
		"ledger", tx.LedgerSeq,
		"tx_hash", tx.Hash,
	)

	// Extract complete activity information
	activity, err := s.extractor.ExtractContractActivity(tx.Tx, trackedContractID, tx.LedgerSeq)
	if err != nil {
		slog.Error("ActivityService: Failed to extract contract activity",
			"error", err,
			"contract_id", trackedContractID,
		)
		return err
	}

	// Save contract activity to database
	if err := s.repository.SaveContractActivity(ctx, activity); err != nil {
		slog.Error("ActivityService: Failed to save contract activity to database",
			"error", err,
			"contract_id", trackedContractID,
		)
		// Don't return - continue processing even if DB save fails
	}

	// NOTE: Events and storage changes are now handled by EventService and StorageChangeService
	// We no longer save them here to avoid duplication

	slog.Info("✅ ActivityService: Contract activity tracked and saved",
		"contract_id", trackedContractID,
		"success", activity.Success,
	)

	return nil
}

// Name returns the service name
func (s *ActivityService) Name() string {
	return "ActivityService"
}

// AddTrackedContract adds a contract ID to the tracking list
// Also notifies EventService and StorageChangeService if they are connected
func (s *ActivityService) AddTrackedContract(contractID string) {
	s.mu.Lock()
	s.trackedContracts[contractID] = true
	s.mu.Unlock()

	slog.Debug("ActivityService: Added contract to tracking", "contract_id", contractID)

	// Notify other services to start tracking this contract
	if s.eventService != nil {
		s.eventService.AddTrackedContract(contractID)
	}

	if s.storageChangeService != nil {
		s.storageChangeService.AddTrackedContract(contractID)
	}
}

// RemoveTrackedContract removes a contract ID from the tracking list
func (s *ActivityService) RemoveTrackedContract(contractID string) {
	s.mu.Lock()
	delete(s.trackedContracts, contractID)
	s.mu.Unlock()

	slog.Debug("ActivityService: Removed contract from tracking", "contract_id", contractID)

	// Notify other services to stop tracking this contract
	if s.eventService != nil {
		s.eventService.RemoveTrackedContract(contractID)
	}

	if s.storageChangeService != nil {
		s.storageChangeService.RemoveTrackedContract(contractID)
	}
}

// GetTrackedCount returns the number of contracts being tracked
func (s *ActivityService) GetTrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trackedContracts)
}
