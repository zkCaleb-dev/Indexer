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

	// Save activity events
	if len(activity.Events) > 0 {
		if err := s.repository.SaveContractEvents(ctx, activity.Events); err != nil {
			slog.Error("ActivityService: Failed to save activity events to database",
				"error", err,
				"contract_id", trackedContractID,
			)
		}
	}

	// Save activity storage changes
	if len(activity.StorageChanges) > 0 {
		if err := s.repository.SaveStorageEntries(ctx, activity.StorageChanges); err != nil {
			slog.Error("ActivityService: Failed to save activity storage changes to database",
				"error", err,
				"contract_id", trackedContractID,
			)
		}
	}

	slog.Info("✅ ActivityService: Contract activity tracked and saved",
		"contract_id", trackedContractID,
		"events_count", len(activity.Events),
		"storage_changes", len(activity.StorageChanges),
		"success", activity.Success,
	)

	return nil
}

// Name returns the service name
func (s *ActivityService) Name() string {
	return "ActivityService"
}

// AddTrackedContract adds a contract ID to the tracking list
func (s *ActivityService) AddTrackedContract(contractID string) {
	s.mu.Lock()
	s.trackedContracts[contractID] = true
	s.mu.Unlock()
	slog.Debug("ActivityService: Added contract to tracking", "contract_id", contractID)
}

// RemoveTrackedContract removes a contract ID from the tracking list
func (s *ActivityService) RemoveTrackedContract(contractID string) {
	s.mu.Lock()
	delete(s.trackedContracts, contractID)
	s.mu.Unlock()
	slog.Debug("ActivityService: Removed contract from tracking", "contract_id", contractID)
}

// GetTrackedCount returns the number of contracts being tracked
func (s *ActivityService) GetTrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trackedContracts)
}
