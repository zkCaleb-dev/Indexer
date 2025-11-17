package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"indexer/internal/extraction"
	"indexer/internal/metrics"
	"indexer/internal/models"
	"indexer/internal/storage"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// changeMetadata stores additional information needed to reconstruct models.StorageChange
type changeMetadata struct {
	txHash          string
	ledgerSeq       uint32
	ledgerCloseTime time.Time
}

// StorageChangeService detects and processes storage state changes for tracked contracts
type StorageChangeService struct {
	networkPassphrase string
	trackedContracts  map[string]bool
	mu                sync.RWMutex // Protects trackedContracts
	repository        storage.Repository
	extractor         *extraction.DataExtractor

	// Accumulation for ChangeCompactor
	currentLedger uint32
	compactor     *ingest.ChangeCompactor
	changes       []ingest.Change
	metadata      []changeMetadata // Parallel array to changes for metadata
}

// NewStorageChangeService creates a new StorageChangeService instance
func NewStorageChangeService(networkPassphrase string, repository storage.Repository) *StorageChangeService {
	return &StorageChangeService{
		networkPassphrase: networkPassphrase,
		trackedContracts:  make(map[string]bool),
		repository:        repository,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
		compactor:         ingest.NewChangeCompactor(ingest.ChangeCompactorConfig{
			SuppressRemoveAfterRestoreChange: false,
		}),
		changes:  make([]ingest.Change, 0, 100),
		metadata: make([]changeMetadata, 0, 100),
	}
}

// Process handles storage change detection and extraction for tracked contracts
// Now accumulates changes instead of saving immediately, to enable compaction
func (s *StorageChangeService) Process(ctx context.Context, tx *ProcessedTx) error {
	// Check if we moved to a new ledger - flush previous ledger if so
	if s.currentLedger != 0 && s.currentLedger != tx.LedgerSeq {
		if err := s.FlushLedger(ctx); err != nil {
			slog.Error("StorageChangeService: Failed to flush ledger",
				"error", err,
				"ledger", s.currentLedger,
			)
			// Continue processing even if flush fails
		}
	}

	s.currentLedger = tx.LedgerSeq

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

	// Extract raw changes (for compaction)
	rawChanges, err := s.extractor.ExtractContractChangesRaw(tx.Tx, trackedContractID)
	if err != nil {
		slog.Error("StorageChangeService: Failed to extract storage changes",
			"error", err,
			"contract_id", trackedContractID,
			"tx_hash", tx.Hash,
		)
		return err
	}

	if len(rawChanges) == 0 {
		return nil // No storage changes in this transaction
	}

	// Accumulate changes for compaction
	for range rawChanges {
		s.metadata = append(s.metadata, changeMetadata{
			txHash:          tx.Hash,
			ledgerSeq:       tx.LedgerSeq,
			ledgerCloseTime: tx.LedgerCloseTime,
		})
	}
	s.changes = append(s.changes, rawChanges...)

	slog.Debug("StorageChangeService: Accumulated changes",
		"contract_id", trackedContractID,
		"changes_count", len(rawChanges),
		"total_accumulated", len(s.changes),
	)

	return nil
}

// FlushLedger compacts accumulated changes and saves them to the database
func (s *StorageChangeService) FlushLedger(ctx context.Context) error {
	if len(s.changes) == 0 {
		return nil
	}

	originalCount := len(s.changes)

	// Add all changes to compactor
	for _, change := range s.changes {
		if err := s.compactor.AddChange(change); err != nil {
			slog.Warn("Failed to add change to compactor", "error", err)
			// Continue with other changes
		}
	}

	// Get compacted changes
	compacted := s.compactor.GetChanges()
	compactedCount := len(compacted)

	// Convert compacted changes back to models.StorageChange
	storageChanges, err := s.convertToStorageChanges(compacted)
	if err != nil {
		return fmt.Errorf("failed to convert compacted changes: %w", err)
	}

	// Save to database (using batch INSERT that we implemented earlier)
	if err := s.repository.SaveStorageChanges(ctx, storageChanges); err != nil {
		return fmt.Errorf("failed to save storage changes: %w", err)
	}

	reduction := 100.0 * (1 - float64(compactedCount)/float64(originalCount))

	// Record metrics
	metrics.CompactorReductionPercent.Set(reduction)
	metrics.StorageChangesSaved.Add(float64(compactedCount))

	slog.Info("✅ StorageChangeService: Ledger flushed with compaction",
		"ledger", s.currentLedger,
		"original_changes", originalCount,
		"compacted_changes", compactedCount,
		"reduction_percent", fmt.Sprintf("%.1f%%", reduction),
	)

	// Reset accumulator
	s.changes = s.changes[:0]
	s.metadata = s.metadata[:0]
	s.compactor = ingest.NewChangeCompactor(ingest.ChangeCompactorConfig{
		SuppressRemoveAfterRestoreChange: false,
	})

	return nil
}

// convertToStorageChanges converts compacted ingest.Change objects back to models.StorageChange
func (s *StorageChangeService) convertToStorageChanges(compacted []ingest.Change) ([]*models.StorageChange, error) {
	// Use the last metadata entry for each change (approximation since we compacted)
	// In practice, compacted changes represent the final state, so we use the most recent metadata
	meta := s.metadata[len(s.metadata)-1]

	var result []*models.StorageChange
	for _, change := range compacted {
		storageChange, err := s.convertSingleChange(change, meta)
		if err != nil {
			slog.Warn("Failed to convert change", "error", err)
			continue
		}
		result = append(result, storageChange)
	}

	return result, nil
}

// convertSingleChange converts a single ingest.Change to models.StorageChange
func (s *StorageChangeService) convertSingleChange(change ingest.Change, meta changeMetadata) (*models.StorageChange, error) {
	if change.Type != xdr.LedgerEntryTypeContractData {
		return nil, fmt.Errorf("not a contract data change")
	}

	var contractData *xdr.ContractDataEntry
	if change.Post != nil {
		contractData = change.Post.Data.ContractData
	} else if change.Pre != nil {
		contractData = change.Pre.Data.ContractData
	} else {
		return nil, fmt.Errorf("change has neither Pre nor Post")
	}

	contractID, err := contractData.Contract.String()
	if err != nil {
		return nil, err
	}

	storageChange := &models.StorageChange{
		ContractID: contractID,
		TxHash:     meta.txHash,
		LedgerSeq:  meta.ledgerSeq,
		Timestamp:  meta.ledgerCloseTime,
		Durability: contractData.Durability.String(),
	}

	// Determine change type
	switch change.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		storageChange.ChangeType = "CREATED"
		storageChange.StorageKey = s.extractor.ScValToMap(contractData.Key)
		storageChange.StorageValue = s.extractor.ScValToMap(contractData.Val)
		storageChange.RawKey, _ = contractData.Key.MarshalBinary()
		storageChange.RawValue, _ = contractData.Val.MarshalBinary()

	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		storageChange.ChangeType = "UPDATED"
		oldData := change.Pre.Data.ContractData
		newData := change.Post.Data.ContractData

		storageChange.StorageKey = s.extractor.ScValToMap(newData.Key)
		storageChange.StorageValue = s.extractor.ScValToMap(newData.Val)
		storageChange.PreviousValue = s.extractor.ScValToMap(oldData.Val)

		storageChange.RawKey, _ = newData.Key.MarshalBinary()
		storageChange.RawValue, _ = newData.Val.MarshalBinary()
		storageChange.RawPreviousValue, _ = oldData.Val.MarshalBinary()

	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		storageChange.ChangeType = "REMOVED"
		storageChange.StorageKey = s.extractor.ScValToMap(contractData.Key)
		storageChange.PreviousValue = s.extractor.ScValToMap(contractData.Val)
		storageChange.RawKey, _ = contractData.Key.MarshalBinary()
		storageChange.RawPreviousValue, _ = contractData.Val.MarshalBinary()

	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		storageChange.ChangeType = "RESTORED"
		storageChange.StorageKey = s.extractor.ScValToMap(contractData.Key)
		storageChange.StorageValue = s.extractor.ScValToMap(contractData.Val)
		storageChange.RawKey, _ = contractData.Key.MarshalBinary()
		storageChange.RawValue, _ = contractData.Val.MarshalBinary()

	default:
		return nil, fmt.Errorf("unknown change type: %v", change.ChangeType)
	}

	return storageChange, nil
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
