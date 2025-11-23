package rpc

import (
	"context"
	"indexer/internal/integration/rpc_backend"

	"github.com/stellar/go/ingest/ledgerbackend"
)

// LedgerBackendHandlerService defines the interface for managing ledger backend lifecycle and range preparation
type LedgerBackendHandlerService interface {
	PrepareRange(ctx context.Context, start, end *uint32) error
	BackendHandlerService[ledgerbackend.LedgerBackend]
}

// LedgerBackend implements the RPC-based ledger backend handler
type LedgerBackend struct {
	ClientConfig rpc_backend.ClientConfig
	backend      ledgerbackend.LedgerBackend
	buildErr     error
	isAvailable  bool
}

// Start initializes the ledger backend by building and configuring the RPC client
func (l *LedgerBackend) Start() error {

	// Build the new backend instance
	backendBuilder := rpc_backend.LedgerBuilder{
		ClientConfig: l.ClientConfig,
	}

	backend, err := backendBuilder.Build()

	if err != nil {
		l.buildErr = err
		l.isAvailable = false
		return err
	}

	// Set the backend and mark it as available
	l.backend = backend
	l.isAvailable = true

	return nil
}

// Close gracefully shuts down the ledger backend
func (l *LedgerBackend) Close() error {
	l.isAvailable = false
	if l.backend != nil {
		return l.backend.Close()
	}
	return nil
}

// IsAvailable returns whether the backend is ready for use
func (l *LedgerBackend) IsAvailable() bool {
	return l.isAvailable
}

// HandleBackend returns the underlying ledger backend instance
func (l *LedgerBackend) HandleBackend() (ledgerbackend.LedgerBackend, error) {
	return l.backend, l.buildErr
}

// PrepareRange configures the backend to stream ledgers within the specified range
func (l *LedgerBackend) PrepareRange(ctx context.Context, start, end *uint32) error {
	var ledgerRange ledgerbackend.Range

	if end == nil {
		// Unbounded range for continuous streaming
		ledgerRange = ledgerbackend.UnboundedRange(*start)
	} else {
		// Bounded range for a specific ledger range
		ledgerRange = ledgerbackend.BoundedRange(*start, *end)
	}

	return l.backend.PrepareRange(ctx, ledgerRange)
}
