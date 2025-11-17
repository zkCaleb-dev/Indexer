package api

import (
	"fmt"
	"strconv"
	"strings"

	"indexer/internal/models"
)

// StrtoopsToXLM converts stroops (smallest unit) to XLM
// 1 XLM = 10,000,000 stroops
func StrtoopsToXLM(stroops string) (string, error) {
	if stroops == "" {
		return "0.0000000", nil
	}

	amount, err := strconv.ParseInt(stroops, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid stroops value: %w", err)
	}

	xlm := float64(amount) / 10000000.0
	return fmt.Sprintf("%.7f", xlm), nil
}

// CalculateContractStatus determines the current status of a contract based on events and storage
func CalculateContractStatus(
	contract *models.DeployedContract,
	events []models.ContractEvent,
	storage []*models.StorageChange,
) string {
	// Check for disputes
	hasActiveDispute := false
	for _, event := range events {
		if event.EventType == "tw_dispute" {
			// Check if there's a resolution after this dispute
			resolved := false
			for _, e := range events {
				if e.EventType == "tw_disp_resolve" && e.LedgerSeq > event.LedgerSeq {
					resolved = true
					break
				}
			}
			if !resolved {
				hasActiveDispute = true
				break
			}
		}
	}

	if hasActiveDispute {
		return "disputed"
	}

	// Check if funded
	funded := false
	for _, event := range events {
		if event.EventType == "tw_fund" {
			funded = true
			break
		}
	}

	if !funded {
		return "pending_funding"
	}

	// Check if all milestones are released (completed)
	milestonesReleased := 0
	totalMilestones := 0

	// Get milestone count from init_params
	if milestones, ok := contract.InitParams["milestones"].([]interface{}); ok {
		totalMilestones = len(milestones)
	}

	// Count releases
	for _, event := range events {
		if event.EventType == "tw_release" {
			milestonesReleased++
		}
	}

	if totalMilestones > 0 && milestonesReleased >= totalMilestones {
		return "completed"
	}

	return "active"
}

// BuildMilestoneResponses creates milestone responses with enriched status from events
func BuildMilestoneResponses(
	contract *models.DeployedContract,
	events []models.ContractEvent,
) ([]models.MilestoneResponse, error) {
	// Extract milestones from init_params
	milestonesData, ok := contract.InitParams["milestones"].([]interface{})
	if !ok {
		return []models.MilestoneResponse{}, nil
	}

	result := make([]models.MilestoneResponse, len(milestonesData))

	for i, m := range milestonesData {
		milestone, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		response := models.MilestoneResponse{
			Index:       i,
			Description: getStringValue(milestone, "description"),
			Evidence:    getStringValue(milestone, "evidence"),
		}

		// For multi-release, extract amount and receiver
		if contract.ContractType == "multi-release" {
			if amount, ok := milestone["amount"].(string); ok {
				response.AmountStroops = amount
				if xlm, err := StrtoopsToXLM(amount); err == nil {
					response.AmountXLM = xlm
				}
			}
			response.Receiver = getStringValue(milestone, "receiver")
		}

		// Analyze events for this milestone
		for _, event := range events {
			milestoneIndex := getMilestoneIndexFromEvent(event)
			if milestoneIndex != i {
				continue
			}

			switch event.EventType {
			case "tw_ms_approve":
				response.Approved = true
				response.ApprovedAt = &event.Timestamp

			case "tw_release":
				response.Released = true
				response.ReleasedAt = &event.Timestamp

			case "tw_dispute":
				response.Disputed = true
				response.DisputedAt = &event.Timestamp

			case "tw_disp_resolve":
				response.Resolved = true
				response.ResolvedAt = &event.Timestamp
			}
		}

		// Determine overall status
		response.Status = calculateMilestoneStatus(response)

		result[i] = response
	}

	return result, nil
}

// calculateMilestoneStatus determines the status based on flags
func calculateMilestoneStatus(m models.MilestoneResponse) string {
	if m.Released {
		return "released"
	}
	if m.Disputed && !m.Resolved {
		return "disputed"
	}
	if m.Resolved {
		return "resolved"
	}
	if m.Approved {
		return "approved"
	}
	return "pending"
}

// getMilestoneIndexFromEvent extracts milestone index from event data
func getMilestoneIndexFromEvent(event models.ContractEvent) int {
	if parsed, ok := event.Data["parsed"].(map[string]interface{}); ok {
		if idx, ok := parsed["milestone_index"].(float64); ok {
			return int(idx)
		}
		if idx, ok := parsed["milestone_index"].(int); ok {
			return idx
		}
	}
	return -1
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// ExtractRolesFromInitParams extracts roles from init_params
func ExtractRolesFromInitParams(initParams map[string]interface{}) models.RolesResponse {
	var roles models.RolesResponse

	if rolesData, ok := initParams["roles"].(map[string]interface{}); ok {
		roles.Approver = getStringValue(rolesData, "approver")
		roles.ServiceProvider = getStringValue(rolesData, "service_provider")
		roles.PlatformAddress = getStringValue(rolesData, "platform_address")
		roles.ReleaseSigner = getStringValue(rolesData, "release_signer")
		roles.DisputeResolver = getStringValue(rolesData, "dispute_resolver")
		roles.Receiver = getStringValue(rolesData, "receiver")
	}

	return roles
}

// BuildContractResponse creates a full contract response
func BuildContractResponse(
	contract *models.DeployedContract,
	events []models.ContractEvent,
	storage []*models.StorageChange,
) (*models.ContractResponse, error) {
	response := &models.ContractResponse{
		ContractID:        contract.ContractID,
		Type:              contract.ContractType,
		Deployer:          contract.Deployer,
		TxHash:            contract.TxHash,
		FactoryContractID: contract.FactoryContractID,
		DeployedAt:        contract.DeployedAtTime,
	}

	// Extract common fields from init_params
	if engagementID, ok := contract.InitParams["engagement_id"].(string); ok {
		response.EngagementID = engagementID
	}
	response.Title = getStringValue(contract.InitParams, "title")
	response.Description = getStringValue(contract.InitParams, "description")

	// Platform fee
	if fee, ok := contract.InitParams["platform_fee"].(float64); ok {
		response.PlatformFee = int(fee)
	}

	// Amount (single-release only)
	if contract.ContractType == "single-release" {
		if amount, ok := contract.InitParams["amount"].(string); ok {
			response.AmountStroops = amount
			if xlm, err := StrtoopsToXLM(amount); err == nil {
				response.AmountXLM = xlm
			}
		}
	}

	// Balance from storage (if available)
	for _, change := range storage {
		if keyStr, ok := change.StorageKey["value"].(string); ok {
			if strings.ToLower(keyStr) == "balance" {
				if balanceStr, ok := change.StorageValue["value"].(string); ok {
					response.BalanceStroops = balanceStr
					if xlm, err := StrtoopsToXLM(balanceStr); err == nil {
						response.BalanceXLM = xlm
					}
				}
			}
		}
	}

	// Roles
	response.Roles = ExtractRolesFromInitParams(contract.InitParams)

	// Milestones
	milestones, err := BuildMilestoneResponses(contract, events)
	if err != nil {
		return nil, err
	}
	response.Milestones = milestones

	// Status
	response.Status = CalculateContractStatus(contract, events, storage)

	// Check if funded
	response.Funded = response.Status != "pending_funding"

	// UpdatedAt - use latest event timestamp if available
	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		response.UpdatedAt = &lastEvent.Timestamp
	}

	return response, nil
}

// BuildContractSummary creates a summary for list views
func BuildContractSummary(contract *models.DeployedContract) models.ContractSummary {
	summary := models.ContractSummary{
		ContractID: contract.ContractID,
		Type:       contract.ContractType,
		DeployedAt: contract.DeployedAtTime,
		Deployer:   contract.Deployer,
	}

	// Extract fields from init_params
	if engagementID, ok := contract.InitParams["engagement_id"].(string); ok {
		summary.EngagementID = engagementID
	}
	summary.Title = getStringValue(contract.InitParams, "title")

	// Amount (single-release only)
	if contract.ContractType == "single-release" {
		if amount, ok := contract.InitParams["amount"].(string); ok {
			if xlm, err := StrtoopsToXLM(amount); err == nil {
				summary.AmountXLM = xlm
			}
		}
	}

	// Status would need events to calculate properly, so we'll set a default
	summary.Status = "unknown"

	return summary
}
