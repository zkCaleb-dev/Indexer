package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"indexer/internal/storage"
)

// Server represents the HTTP API server
// Provides endpoints for Prometheus metrics, health checks, and custom REST APIs
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	repository storage.Repository
	port       int
}

// NewServer creates a new API server instance
// The repository is made available to all handlers for database access
func NewServer(port int, repository storage.Repository) *Server {
	mux := http.NewServeMux()

	s := &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		mux:        mux,
		repository: repository,
		port:       port,
	}

	// Register all HTTP routes
	s.registerRoutes()

	return s
}

// registerRoutes sets up all HTTP routes
// Add your custom endpoints here by calling s.mux.HandleFunc(...)
func (s *Server) registerRoutes() {
	// Core endpoints (implemented)
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.Handle("/metrics", s.handleMetrics())

	// Future custom endpoints (for user to implement):
	// Example: List deployed contracts
	// s.mux.HandleFunc("/api/contracts", s.handleListContracts)
	//
	// Example: Get contract details by ID
	// s.mux.HandleFunc("/api/contracts/", s.handleGetContract)
	//
	// Example: List contract events
	// s.mux.HandleFunc("/api/events", s.handleListEvents)
	//
	// Example: Get storage changes
	// s.mux.HandleFunc("/api/storage-changes", s.handleListStorageChanges)
}

// Start starts the HTTP server in a goroutine
// Returns immediately after starting the server
func (s *Server) Start() error {
	go func() {
		slog.Info("API server starting",
			"port", s.port,
			"endpoints", []string{"/", "/health", "/metrics"},
		)

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("API server error", "error", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Shutdown gracefully shuts down the HTTP server
// Waits for active connections to close or context to timeout
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("API server shutting down...")
	return s.httpServer.Shutdown(ctx)
}
