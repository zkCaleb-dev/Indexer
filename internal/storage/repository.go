package storage

import (
	"context"
	"indexer/internal/models"
)

// Repository defines the interface for all storage operations
type Repository interface {
	// Deployed Contracts
	SaveDeployedContract(ctx context.Context, contract *models.DeployedContract) error
	GetDeployedContract(ctx context.Context, contractID string) (*models.DeployedContract, error)
	ListDeployedContracts(ctx context.Context, limit, offset int) ([]*models.DeployedContract, error)
	GetTrackedContractIDs(ctx context.Context) ([]string, error)

	// Contract Events
	SaveContractEvent(ctx context.Context, event *models.ContractEvent) error
	SaveContractEvents(ctx context.Context, events []models.ContractEvent) error
	ListContractEvents(ctx context.Context, contractID string, limit, offset int) ([]models.ContractEvent, error)

	// Storage Entries
	SaveStorageEntry(ctx context.Context, entry *models.StorageEntry) error
	SaveStorageEntries(ctx context.Context, entries []models.StorageEntry) error
	GetLatestStorageState(ctx context.Context, contractID string) ([]models.StorageEntry, error)

	// Contract Activities
	SaveContractActivity(ctx context.Context, activity *models.ContractActivity) error
	ListContractActivities(ctx context.Context, contractID string, limit, offset int) ([]*models.ContractActivity, error)

	// Ledger Info
	SaveLedgerInfo(ctx context.Context, info *models.LedgerInfo) error
	GetLastProcessedLedger(ctx context.Context) (uint32, error)

	// Health & Maintenance
	Ping(ctx context.Context) error
	Close() error
}
