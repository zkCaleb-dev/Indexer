package models

import "time"

// ContractEvent represents an event emitted by a smart contract
type ContractEvent struct {
	// Identification
	ContractID string `json:"contract_id"`
	EventType  string `json:"event_type"` // Parsed event type from topics
	EventIndex int    `json:"event_index"` // Index within transaction

	// Event data
	Topics    []string               `json:"topics"`          // Raw topic data (base64 or hex)
	Data      map[string]interface{} `json:"data,omitempty"`  // Parsed event data
	RawTopics [][]byte               `json:"-"`               // Raw topic bytes (not serialized)
	RawData   []byte                 `json:"-"`               // Raw data bytes (not serialized)

	// Transaction context
	TxHash    string    `json:"tx_hash"`
	LedgerSeq uint32    `json:"ledger_seq"`
	Timestamp time.Time `json:"timestamp"`

	// Diagnostic info
	InSuccessfulContractCall bool `json:"in_successful_contract_call"`
}

// EventFilter provides criteria for filtering events
type EventFilter struct {
	ContractID   string
	EventType    string
	FromLedger   uint32
	ToLedger     uint32
	FromTime     *time.Time
	ToTime       *time.Time
	Limit        int
	Offset       int
}
