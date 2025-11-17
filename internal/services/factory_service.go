package services

import (
	"context"
	"log/slog"

	"indexer/internal/extraction"
	"indexer/internal/storage"
)

// FactoryService detects and processes contract deployments from factory contracts
type FactoryService struct {
	factoryContracts  map[string]string // factory_id -> contract_type
	networkPassphrase string
	repository        storage.Repository
	extractor         *extraction.DataExtractor
	activityService   *ActivityService // Optional: to notify when new contracts are deployed
}

// NewFactoryService creates a new FactoryService instance
func NewFactoryService(factoryContracts map[string]string, networkPassphrase string, repository storage.Repository) *FactoryService {
	return &FactoryService{
		factoryContracts:  factoryContracts,
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
	// Check if any factory contract is in the transaction footprint
	factoryID, contractType, isFactory := s.detectFactory(tx.ContractIDs)

	if !isFactory {
		return nil // Not a factory deployment, skip
	}

	// Extract complete deployment information
	contract, err := s.extractor.ExtractDeployedContract(tx.Tx, factoryID, tx.LedgerSeq, tx.LedgerCloseTime)
	if err != nil {
		slog.Error("FactoryService: Failed to extract deployed contract",
			"error", err,
			"tx_hash", tx.Hash,
		)
		return err
	}

	// Set the contract type (single-release or multi-release)
	contract.ContractType = contractType

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

	slog.Info("✅ FactoryService: Contract deployment processed",
		"contract_id", contract.ContractID,
		"contract_type", contract.ContractType,
		"factory_id", contract.FactoryContractID,
	)

	return nil
}

// detectFactory checks if any contract ID matches a factory and returns factory ID, type, and match status
func (s *FactoryService) detectFactory(contractIDs []string) (string, string, bool) {
	for _, contractID := range contractIDs {
		if contractType, exists := s.factoryContracts[contractID]; exists {
			return contractID, contractType, true
		}
	}
	return "", "", false
}

// Name returns the service name
func (s *FactoryService) Name() string {
	return "FactoryService"
}
