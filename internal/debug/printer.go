package debug

import (
	"encoding/json"
	"log/slog"

	"indexer/internal/models"
)

// PrintDeployedContract prints the deployed contract in JSON format
func PrintDeployedContract(contract *models.DeployedContract) {
	jsonData, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal contract to JSON", "error", err)
		return
	}

	slog.Debug("Deployed contract details", "json", string(jsonData))
}

// PrintContractActivity prints the contract activity in JSON format
func PrintContractActivity(activity *models.ContractActivity) {
	jsonData, err := json.MarshalIndent(activity, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal activity to JSON", "error", err)
		return
	}

	slog.Debug("Contract activity details", "json", string(jsonData))
}
