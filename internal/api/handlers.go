package api

import (
	"encoding/json"
	"net/http"
	"time"

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
			"GET /":        "This page - Service information",
			"GET /health":  "Health check endpoint",
			"GET /metrics": "Prometheus metrics for monitoring",
		},
		"note": "Additional REST API endpoints can be added in internal/api/handlers.go",
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
// CUSTOM ENDPOINTS - Template examples for user to implement
// =============================================================================

// Example: List all deployed contracts
// func (s *Server) handleListContracts(w http.ResponseWriter, r *http.Request) {
//     ctx := r.Context()
//
//     // Parse query parameters
//     limitStr := r.URL.Query().Get("limit")
//     limit := 100 // default
//     if limitStr != "" {
//         if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
//             limit = parsed
//         }
//     }
//
//     // Query from database
//     contracts, err := s.repository.ListDeployedContracts(ctx, limit)
//     if err != nil {
//         slog.Error("Failed to list contracts", "error", err)
//         http.Error(w, "Internal server error", http.StatusInternalServerError)
//         return
//     }
//
//     // Return JSON response
//     w.Header().Set("Content-Type", "application/json")
//     json.NewEncoder(w).Encode(map[string]interface{}{
//         "contracts": contracts,
//         "count":     len(contracts),
//     })
// }

// Example: Get contract by ID
// func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
//     // Extract contract ID from URL path
//     // Parse: /api/contracts/CDQPREX7...
//     path := strings.TrimPrefix(r.URL.Path, "/api/contracts/")
//     contractID := strings.TrimSpace(path)
//
//     if contractID == "" {
//         http.Error(w, "Contract ID required", http.StatusBadRequest)
//         return
//     }
//
//     ctx := r.Context()
//     contract, err := s.repository.GetContractByID(ctx, contractID)
//     if err != nil {
//         http.Error(w, "Contract not found", http.StatusNotFound)
//         return
//     }
//
//     w.Header().Set("Content-Type", "application/json")
//     json.NewEncoder(w).Encode(contract)
// }

// Example: List events with filtering
// func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
//     ctx := r.Context()
//
//     // Parse filters from query parameters
//     contractID := r.URL.Query().Get("contract_id")
//     eventType := r.URL.Query().Get("event_type")
//
//     events, err := s.repository.GetContractEvents(ctx, contractID, eventType, 100)
//     if err != nil {
//         http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
//         return
//     }
//
//     w.Header().Set("Content-Type", "application/json")
//     json.NewEncoder(w).Encode(map[string]interface{}{
//         "events": events,
//         "count":  len(events),
//     })
// }
