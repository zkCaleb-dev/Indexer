package models

import "time"

// DeployedContract represents a contract deployed through a factory
type DeployedContract struct {
	// Identification
	ContractID        string `json:"contract_id"`
	FactoryContractID string `json:"factory_contract_id"`

	// Deployment metadata
	DeployedAtLedger uint32    `json:"deployed_at_ledger"`
	DeployedAtTime   time.Time `json:"deployed_at_time"`
	TxHash           string    `json:"tx_hash"`
	Deployer         string    `json:"deployer"` // Account that deployed

	// Code and resources
	WasmHash string `json:"wasm_hash,omitempty"` // Hash of WASM code
	WasmSize int    `json:"wasm_size,omitempty"` // Size of WASM code in bytes

	// Costs and resources
	FeeCharged      int64  `json:"fee_charged"`
	CPUInstructions uint64 `json:"cpu_instructions"`
	MemoryBytes     uint32 `json:"memory_bytes"`

	// Initialization data
	InitParams  map[string]interface{} `json:"init_params,omitempty"`  // Parsed initialization parameters
	InitEvents  []ContractEvent        `json:"init_events,omitempty"`  // Events emitted during deployment
	InitStorage []StorageEntry         `json:"init_storage,omitempty"` // Initial storage state

	// Metadata
	Memo     string `json:"memo,omitempty"`
	MemoType string `json:"memo_type,omitempty"`
}
