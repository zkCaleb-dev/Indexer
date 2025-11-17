package models

import (
	"time"
)

// ContractResponse represents a contract with current state for API responses
type ContractResponse struct {
	ContractID   string    `json:"contract_id"`
	EngagementID string    `json:"engagement_id,omitempty"`
	Type         string    `json:"type"` // "single-release" or "multi-release"
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty"`

	// Financials (formatted for UI)
	AmountStroops  string `json:"amount_stroops,omitempty"`
	AmountXLM      string `json:"amount_xlm,omitempty"` // Divided by 10^7
	BalanceStroops string `json:"balance_stroops,omitempty"`
	BalanceXLM     string `json:"balance_xlm,omitempty"`
	PlatformFee    int    `json:"platform_fee,omitempty"`

	// Status
	Status string `json:"status"` // active, completed, disputed, pending_funding
	Funded bool   `json:"funded"`

	// Participants
	Roles RolesResponse `json:"roles"`

	// Milestones
	Milestones []MilestoneResponse `json:"milestones,omitempty"`

	// Metadata
	DeployedAt time.Time  `json:"deployed_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	Deployer   string     `json:"deployer,omitempty"`
	TxHash     string     `json:"tx_hash,omitempty"`

	// Factory info
	FactoryContractID string `json:"factory_contract_id,omitempty"`
}

// RolesResponse represents contract participant roles
type RolesResponse struct {
	Approver        string `json:"approver,omitempty"`
	ServiceProvider string `json:"service_provider,omitempty"`
	PlatformAddress string `json:"platform_address,omitempty"`
	ReleaseSigner   string `json:"release_signer,omitempty"`
	DisputeResolver string `json:"dispute_resolver,omitempty"`
	Receiver        string `json:"receiver,omitempty"` // Only for single-release
}

// MilestoneResponse represents a milestone with enriched status
type MilestoneResponse struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`

	// Financials (multi-release only)
	AmountStroops string `json:"amount_stroops,omitempty"`
	AmountXLM     string `json:"amount_xlm,omitempty"`
	Receiver      string `json:"receiver,omitempty"`

	// Status
	Status   string `json:"status"` // pending, approved, released, disputed, resolved
	Approved bool   `json:"approved"`
	Released bool   `json:"released"`
	Disputed bool   `json:"disputed"`
	Resolved bool   `json:"resolved"`

	// Timestamps (from event analysis)
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	DisputedAt *time.Time `json:"disputed_at,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// EventResponse represents a simplified event for timeline
type EventResponse struct {
	EventType  string                 `json:"event_type"`
	Timestamp  time.Time              `json:"timestamp"`
	LedgerSeq  uint32                 `json:"ledger_seq"`
	TxHash     string                 `json:"tx_hash"`
	EventIndex int                    `json:"event_index"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// ContractListResponse represents a paginated list of contracts
type ContractListResponse struct {
	Contracts []ContractSummary `json:"contracts"`
	Total     int               `json:"total"`
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
}

// ContractSummary represents a contract summary for list views
type ContractSummary struct {
	ContractID   string    `json:"contract_id"`
	EngagementID string    `json:"engagement_id,omitempty"`
	Type         string    `json:"type"`
	Title        string    `json:"title,omitempty"`
	AmountXLM    string    `json:"amount_xlm,omitempty"`
	Status       string    `json:"status"`
	DeployedAt   time.Time `json:"deployed_at"`
	Deployer     string    `json:"deployer,omitempty"`
}

// EventsResponse represents a list of events
type EventsResponse struct {
	ContractID string          `json:"contract_id"`
	Events     []EventResponse `json:"events"`
	Total      int             `json:"total"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}
