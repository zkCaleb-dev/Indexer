package models

// StorageEntry represents a contract storage key-value pair
type StorageEntry struct {
	// Identification
	ContractID string `json:"contract_id"`
	Key        string `json:"key"`      // Hex or base64 encoded key
	KeyType    string `json:"key_type"` // Type of the key (e.g., "symbol", "address", "u64")

	// Value
	Value     interface{} `json:"value"`           // Parsed value
	ValueType string      `json:"value_type"`      // Type of the value
	RawKey    []byte      `json:"-"`               // Raw key bytes (not serialized)
	RawValue  []byte      `json:"-"`               // Raw value bytes (not serialized)

	// Change metadata
	ChangeType string `json:"change_type"` // "created", "updated", "removed"

	// Transaction context
	LedgerSeq     uint32 `json:"ledger_seq"`
	TxHash        string `json:"tx_hash"`

	// Previous value (for updates)
	PreviousValue interface{} `json:"previous_value,omitempty"`
}

// StorageChangeType represents the type of storage change
type StorageChangeType string

const (
	StorageCreated StorageChangeType = "created"
	StorageUpdated StorageChangeType = "updated"
	StorageRemoved StorageChangeType = "removed"
)
