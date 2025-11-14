package services

import (
	"context"
	"log/slog"

	"indexer/internal/storage"
)

// DepositService tracks direct deposits to tracked contracts (e.g., USDC transfers)
type DepositService struct {
	assetCode        string // e.g., "USDC"
	trackedContracts map[string]bool
	repository       storage.Repository
}

// NewDepositService creates a new DepositService instance
func NewDepositService(assetCode string, repository storage.Repository) *DepositService {
	return &DepositService{
		assetCode:        assetCode,
		trackedContracts: make(map[string]bool),
		repository:       repository,
	}
}

// Process handles deposit detection from transfer events
func (s *DepositService) Process(ctx context.Context, tx *ProcessedTx) error {
	// TODO: Extract events from transaction
	// TODO: Filter "transfer" events for specific asset (e.g., USDC)
	// TODO: Check if "to" address is a tracked contract
	// TODO: Save deposit information to database

	// For now, just log
	slog.Debug("DepositService: Processing transaction (stub mode)",
		"tx_hash", tx.Hash,
		"ledger", tx.LedgerSeq,
		"asset_filter", s.assetCode,
	)

	return nil
}

// Name returns the service name
func (s *DepositService) Name() string {
	return "DepositService"
}

// AddTrackedContract adds a contract ID to track for deposits
func (s *DepositService) AddTrackedContract(contractID string) {
	s.trackedContracts[contractID] = true
}
