package services

import (
	"context"
	"log/slog"

	"indexer/internal/extraction"
	"indexer/internal/storage"
)

// FactoryService detects and processes contract deployments from factory contracts
type FactoryService struct {
	factoryContractID string
	networkPassphrase string
	repository        storage.Repository
	extractor         *extraction.DataExtractor
	activityService   *ActivityService // Optional: to notify when new contracts are deployed
}

// NewFactoryService creates a new FactoryService instance
func NewFactoryService(factoryContractID string, networkPassphrase string, repository storage.Repository) *FactoryService {
	return &FactoryService{
		factoryContractID: factoryContractID,
		networkPassphrase: networkPassphrase,
		repository:        repository,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
		activityService:   nil, // Will be set via SetActivityService if needed
	}
}

// SetActivityService sets the activity service to notify on new deployments
func (s *FactoryService) SetActivityService(activityService *ActivityService) {
	s.activityService = activityService
}

// Process handles factory deployment detection
func (s *FactoryService) Process(ctx context.Context, tx *ProcessedTx) error {
	// Check if factory contract is in the transaction footprint
	isFactory := false
	for _, contractID := range tx.ContractIDs {
		if contractID == s.factoryContractID {
			isFactory = true
			break
		}
	}

	if !isFactory {
		return nil // Not a factory deployment, skip
	}

	// Extract complete deployment information
	contract, err := s.extractor.ExtractDeployedContract(tx.Tx, s.factoryContractID, tx.LedgerSeq)
	if err != nil {
		slog.Error("FactoryService: Failed to extract deployed contract",
			"error", err,
			"tx_hash", tx.Hash,
		)
		return err
	}

	// Save deployed contract to database
	if err := s.repository.SaveDeployedContract(ctx, contract); err != nil {
		slog.Error("FactoryService: Failed to save deployed contract to database",
			"error", err,
			"contract_id", contract.ContractID,
		)
		// Don't return error - continue processing even if DB save fails
	}

	// Save initialization events
	if len(contract.InitEvents) > 0 {
		if err := s.repository.SaveContractEvents(ctx, contract.InitEvents); err != nil {
			slog.Error("FactoryService: Failed to save contract events to database",
				"error", err,
				"contract_id", contract.ContractID,
			)
		}
	}

	// Save initialization storage
	if len(contract.InitStorage) > 0 {
		if err := s.repository.SaveStorageEntries(ctx, contract.InitStorage); err != nil {
			slog.Error("FactoryService: Failed to save storage entries to database",
				"error", err,
				"contract_id", contract.ContractID,
			)
		}
	}

	slog.Info("✅ FactoryService: New contract deployed and saved",
		"contract_id", contract.ContractID,
		"deployer", contract.Deployer,
		"fee", contract.FeeCharged,
		"events_count", len(contract.InitEvents),
		"storage_entries", len(contract.InitStorage),
	)

	// Notify ActivityService to start tracking this contract
	if s.activityService != nil {
		s.activityService.AddTrackedContract(contract.ContractID)
	}

	return nil
}

// Name returns the service name
func (s *FactoryService) Name() string {
	return "FactoryService"
}
