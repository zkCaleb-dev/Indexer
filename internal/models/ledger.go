package models

import "time"

// LedgerInfo represents metadata about a ledger
type LedgerInfo struct {
	Sequence      uint32    `json:"sequence"`
	Hash          string    `json:"hash"`
	PreviousHash  string    `json:"previous_hash"`
	CloseTime     time.Time `json:"close_time"`
	TxCount       int       `json:"tx_count"`
	SorobanTxCount int      `json:"soroban_tx_count"`

	// Processing metadata
	ProcessedAt time.Time `json:"processed_at"`
	ProcessingDuration int64 `json:"processing_duration_ms"`
}

// ProcessingCheckpoint represents the current indexing progress
type ProcessingCheckpoint struct {
	LastProcessedLedger uint32    `json:"last_processed_ledger"`
	LastProcessedAt     time.Time `json:"last_processed_at"`
	ContractsTracked    int       `json:"contracts_tracked"`
	TotalEvents         int       `json:"total_events"`
	TotalActivities     int       `json:"total_activities"`
}
