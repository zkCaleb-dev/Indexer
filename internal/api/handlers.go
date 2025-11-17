package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"indexer/internal/models"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// handleIndex returns basic indexer information
// GET / - Returns service info and available endpoints
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	info := map[string]interface{}{
		"service":     "Stellar Indexer",
		"version":     "1.0.0",
		"description": "TrustlessWork Smart Contract Indexer for Stellar",
		"endpoints": map[string]string{
			"GET /":                         "This page - Service information",
			"GET /health":                   "Health check endpoint",
			"GET /metrics":                  "Prometheus metrics for monitoring",
			"GET /contracts":                "List all deployed contracts (supports ?type=, ?deployer=, ?limit=, ?offset=)",
			"GET /contracts/{id}":           "Get contract details with current state",
			"GET /contracts/{id}/events":    "Get event timeline for a contract",
			"GET /contracts/{id}/milestones": "Get milestone status for a contract",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleHealth returns health status
// GET /health - Health check for monitoring systems
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Could add database ping here to verify database connectivity
	// ctx := r.Context()
	// if err := s.repository.Ping(ctx); err != nil {
	//     http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
	//     return
	// }

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "stellar-indexer",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleMetrics returns Prometheus metrics
// GET /metrics - Prometheus scraping endpoint
func (s *Server) handleMetrics() http.Handler {
	return promhttp.Handler()
}

// =============================================================================
// CONTRACT ENDPOINTS
// =============================================================================

// handleListContracts lists all deployed contracts with optional filtering
// GET /contracts?type=single-release&deployer=GXXX...&limit=50&offset=0
func (s *Server) handleListContracts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := r.URL.Query()

	// Pagination
	limit := 50 // default
	if limitStr := query.Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Filters
	var contractType *string
	if typeStr := query.Get("type"); typeStr != "" {
		contractType = &typeStr
	}

	var deployer *string
	if deployerStr := query.Get("deployer"); deployerStr != "" {
		deployer = &deployerStr
	}

	// Get total count
	total, err := s.repository.CountDeployedContracts(ctx, contractType)
	if err != nil {
		slog.Error("Failed to count contracts", "error", err)
		s.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get contracts
	contracts, err := s.repository.ListDeployedContractsFiltered(ctx, contractType, deployer, limit, offset)
	if err != nil {
		slog.Error("Failed to list contracts", "error", err)
		s.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build summaries
	summaries := make([]models.ContractSummary, len(contracts))
	for i, contract := range contracts {
		summaries[i] = BuildContractSummary(contract)
	}

	// Calculate pagination
	page := (offset / limit) + 1
	if offset == 0 {
		page = 1
	}

	response := models.ContractListResponse{
		Contracts: summaries,
		Total:     total,
		Page:      page,
		PageSize:  limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetContract returns detailed contract information with current state
// GET /contracts/{id}
func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	// Extract contract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/contracts/")
	contractID := strings.Split(path, "/")[0]

	if contractID == "" {
		s.sendError(w, "Contract ID required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get contract
	contract, err := s.repository.GetDeployedContract(ctx, contractID)
	if err != nil {
		slog.Error("Failed to get contract", "contract_id", contractID, "error", err)
		s.sendError(w, "Contract not found", http.StatusNotFound)
		return
	}

	// Get events
	events, err := s.repository.ListContractEvents(ctx, contractID, 1000, 0)
	if err != nil {
		slog.Error("Failed to get events", "contract_id", contractID, "error", err)
		events = []models.ContractEvent{} // Continue without events
	}

	// Get latest storage
	storage, err := s.repository.GetLatestStorageChanges(ctx, contractID)
	if err != nil {
		slog.Error("Failed to get storage", "contract_id", contractID, "error", err)
		storage = []*models.StorageChange{} // Continue without storage
	}

	// Build response
	response, err := BuildContractResponse(contract, events, storage)
	if err != nil {
		slog.Error("Failed to build response", "error", err)
		s.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetContractEvents returns the event timeline for a contract
// GET /contracts/{id}/events
func (s *Server) handleGetContractEvents(w http.ResponseWriter, r *http.Request) {
	// Extract contract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/contracts/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		s.sendError(w, "Contract ID required", http.StatusBadRequest)
		return
	}
	contractID := parts[0]

	ctx := r.Context()

	// Get events
	events, err := s.repository.ListContractEvents(ctx, contractID, 1000, 0)
	if err != nil {
		slog.Error("Failed to get events", "contract_id", contractID, "error", err)
		s.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	eventResponses := make([]models.EventResponse, len(events))
	for i, event := range events {
		eventResponses[i] = models.EventResponse{
			EventType:  event.EventType,
			Timestamp:  event.Timestamp,
			LedgerSeq:  event.LedgerSeq,
			TxHash:     event.TxHash,
			EventIndex: event.EventIndex,
			Data:       event.Data,
		}
	}

	response := models.EventsResponse{
		ContractID: contractID,
		Events:     eventResponses,
		Total:      len(eventResponses),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetMilestones returns milestone status for a contract
// GET /contracts/{id}/milestones
func (s *Server) handleGetMilestones(w http.ResponseWriter, r *http.Request) {
	// Extract contract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/contracts/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		s.sendError(w, "Contract ID required", http.StatusBadRequest)
		return
	}
	contractID := parts[0]

	ctx := r.Context()

	// Get contract
	contract, err := s.repository.GetDeployedContract(ctx, contractID)
	if err != nil {
		slog.Error("Failed to get contract", "contract_id", contractID, "error", err)
		s.sendError(w, "Contract not found", http.StatusNotFound)
		return
	}

	// Get events
	events, err := s.repository.ListContractEvents(ctx, contractID, 1000, 0)
	if err != nil {
		slog.Error("Failed to get events", "contract_id", contractID, "error", err)
		events = []models.ContractEvent{} // Continue without events
	}

	// Build milestone responses
	milestones, err := BuildMilestoneResponses(contract, events)
	if err != nil {
		slog.Error("Failed to build milestones", "error", err)
		s.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"contract_id": contractID,
		"milestones":  milestones,
		"total":       len(milestones),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendError sends a JSON error response
func (s *Server) sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   http.StatusText(code),
		Message: message,
		Code:    code,
	})
}
