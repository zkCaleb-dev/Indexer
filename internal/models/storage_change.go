package models

import "time"

// StorageChange represents a change to smart contract storage
type StorageChange struct {
	ID              int64  `db:"id"`
	ContractID      string `db:"contract_id"`
	ChangeType      string `db:"change_type"` // CREATED, UPDATED, REMOVED, RESTORED

	// Parsed storage key/value (from SCVal)
	StorageKey      map[string]interface{} `db:"storage_key"`
	StorageValue    map[string]interface{} `db:"storage_value"`    // NULL if removed
	PreviousValue   map[string]interface{} `db:"previous_value"`   // NULL if created/restored

	// Raw XDR bytes for exact reconstruction
	RawKey          []byte `db:"raw_key"`
	RawValue        []byte `db:"raw_value"`
	RawPreviousValue []byte `db:"raw_previous_value"`

	// Storage durability type
	Durability      string `db:"durability"` // TEMPORARY or PERSISTENT

	// Transaction context
	TxHash          string    `db:"tx_hash"`
	LedgerSeq       uint32    `db:"ledger_seq"`
	OperationIndex  *uint32   `db:"operation_index"` // NULL if transaction-level change
	Timestamp       time.Time `db:"timestamp"`

	// Metadata
	CreatedAt       time.Time `db:"created_at"`
}

// StorageChangeFilter defines criteria for querying storage changes
type StorageChangeFilter struct {
	ContractID  string
	ChangeType  string // CREATED, UPDATED, REMOVED, RESTORED
	FromLedger  uint32
	ToLedger    uint32
	FromTime    time.Time
	ToTime      time.Time
	Limit       int
	Offset      int
}
