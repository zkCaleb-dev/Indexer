package services

import (
	"context"
	"log/slog"
	"sync"

	"indexer/internal/extraction"
	"indexer/internal/storage"
)

// StorageChangeService detects and processes storage state changes for tracked contracts
type StorageChangeService struct {
	networkPassphrase string
	trackedContracts  map[string]bool
	mu                sync.RWMutex // Protects trackedContracts
	repository        storage.Repository
	extractor         *extraction.DataExtractor
}

// NewStorageChangeService creates a new StorageChangeService instance
func NewStorageChangeService(networkPassphrase string, repository storage.Repository) *StorageChangeService {
	return &StorageChangeService{
		networkPassphrase: networkPassphrase,
		trackedContracts:  make(map[string]bool),
		repository:        repository,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
	}
}

// Process handles storage change detection and extraction for tracked contracts
func (s *StorageChangeService) Process(ctx context.Context, tx *ProcessedTx) error {
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

	// Extract storage changes
	changes, err := s.extractor.ExtractContractStorageChanges(tx.Tx, trackedContractID, tx.LedgerSeq, tx.LedgerCloseTime)
	if err != nil {
		slog.Error("StorageChangeService: Failed to extract storage changes",
			"error", err,
			"contract_id", trackedContractID,
			"tx_hash", tx.Hash,
		)
		return err
	}

	if len(changes) == 0 {
		return nil // No storage changes in this transaction
	}

	// Save storage changes to database
	if err := s.repository.SaveStorageChanges(ctx, changes); err != nil {
		slog.Error("StorageChangeService: Failed to save storage changes to database",
			"error", err,
			"contract_id", trackedContractID,
		)
		// Don't return error - continue processing even if DB save fails
	}

	slog.Info("✅ StorageChangeService: Storage changes saved",
		"contract_id", trackedContractID,
		"changes_count", len(changes),
		"tx_hash", tx.Hash,
	)

	return nil
}

// Name returns the service name
func (s *StorageChangeService) Name() string {
	return "StorageChangeService"
}

// AddTrackedContract adds a contract ID to the tracking list
func (s *StorageChangeService) AddTrackedContract(contractID string) {
	s.mu.Lock()
	s.trackedContracts[contractID] = true
	s.mu.Unlock()
	slog.Debug("StorageChangeService: Added contract to tracking", "contract_id", contractID)
}

// RemoveTrackedContract removes a contract ID from the tracking list
func (s *StorageChangeService) RemoveTrackedContract(contractID string) {
	s.mu.Lock()
	delete(s.trackedContracts, contractID)
	s.mu.Unlock()
	slog.Debug("StorageChangeService: Removed contract from tracking", "contract_id", contractID)
}

// GetTrackedCount returns the number of contracts being tracked
func (s *StorageChangeService) GetTrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trackedContracts)
}
