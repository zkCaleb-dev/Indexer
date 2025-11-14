package services

import (
	"context"
	"log/slog"

	"indexer/internal/storage"
)

// FactoryService detects and processes contract deployments from factory contracts
type FactoryService struct {
	factoryContractID string
	repository        storage.Repository
}

// NewFactoryService creates a new FactoryService instance
func NewFactoryService(factoryContractID string, repository storage.Repository) *FactoryService {
	return &FactoryService{
		factoryContractID: factoryContractID,
		repository:        repository,
	}
}

// Process handles factory deployment detection
func (s *FactoryService) Process(ctx context.Context, tx ProcessedTx) error {
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

	// TODO: Extract and save deployment data
	// For now, just log
	slog.Debug("FactoryService: Deployment detected (stub mode)",
		"tx_hash", tx.Hash,
		"ledger", tx.LedgerSeq,
	)

	return nil
}

// Name returns the service name
func (s *FactoryService) Name() string {
	return "FactoryService"
}
