package rpc

import (
	"context"
	"indexer/internal/integration/rpc_backend"

	"github.com/stellar/go/ingest/ledgerbackend"
)

type LedgerBackendHandlerService interface {
	PrepareRange(ctx context.Context, start, end *uint32) error
	BackendHandlerService[ledgerbackend.LedgerBackend]
}

type LedgerBackend struct {
	ClientConfig rpc_backend.ClientConfig
	backend      ledgerbackend.LedgerBackend
	buildErr     error
	isAvailable  bool
}

func (l *LedgerBackend) Start() error {

	// Let's Build the new Backend Instance
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

func (l *LedgerBackend) Close() error {
	l.isAvailable = false
	if l.backend != nil {
		return l.backend.Close()
	}
	return nil
}

func (l *LedgerBackend) IsAvailable() bool {
	return l.isAvailable
}

func (l *LedgerBackend) HandleBackend() (ledgerbackend.LedgerBackend, error) {
	return l.backend, l.buildErr
}

func (l *LedgerBackend) PrepareRange(ctx context.Context, start, end *uint32) error {
	var ledgerRange ledgerbackend.Range

	if end == nil {
		// UnboundedRange for continuous streaming
		ledgerRange = ledgerbackend.UnboundedRange(*start)
	} else {
		// BoundedRange for specific range
		ledgerRange = ledgerbackend.BoundedRange(*start, *end)
	}

	return l.backend.PrepareRange(ctx, ledgerRange)
}
