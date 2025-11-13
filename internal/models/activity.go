package models

import "time"

// ContractActivity represents any activity/interaction with a contract
type ContractActivity struct {
	// Identification
	ActivityID string `json:"activity_id"` // Unique ID (could be txhash:index)
	ContractID string `json:"contract_id"`
	ActivityType string `json:"activity_type"` // "deployment", "invocation", "upgrade", etc.

	// Transaction context
	TxHash    string    `json:"tx_hash"`
	LedgerSeq uint32    `json:"ledger_seq"`
	Timestamp time.Time `json:"timestamp"`

	// Actor
	Invoker string `json:"invoker"` // Account that invoked the contract

	// Function invoked
	FunctionName string                 `json:"function_name,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`

	// Results
	Success       bool        `json:"success"`
	ReturnValue   interface{} `json:"return_value,omitempty"`
	FailureReason string      `json:"failure_reason,omitempty"`

	// Side effects
	Events         []ContractEvent `json:"events,omitempty"`
	StorageChanges []StorageEntry  `json:"storage_changes,omitempty"`

	// Resources used
	FeeCharged      int64  `json:"fee_charged"`
	CPUInstructions uint64 `json:"cpu_instructions,omitempty"`
	MemoryBytes     uint32 `json:"memory_bytes,omitempty"`
}

// ActivityType represents the type of contract activity
type ActivityType string

const (
	ActivityDeployment ActivityType = "deployment"
	ActivityInvocation ActivityType = "invocation"
	ActivityUpgrade    ActivityType = "upgrade"
)

// ActivityFilter provides criteria for filtering activities
type ActivityFilter struct {
	ContractID   string
	ActivityType string
	Invoker      string
	FromLedger   uint32
	ToLedger     uint32
	FromTime     *time.Time
	ToTime       *time.Time
	SuccessOnly  bool
	Limit        int
	Offset       int
}
