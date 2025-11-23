package rpc_backend

import (
	"fmt"
	"net/http"

	"github.com/stellar/go/ingest/ledgerbackend"
)

// LedgerBuilder is responsible for constructing RPC ledger backend instances
type LedgerBuilder struct {
	ClientConfig ClientConfig
}

// Build creates a new RPC ledger backend instance from the client configuration
func (lw *LedgerBuilder) Build() (*ledgerbackend.RPCLedgerBackend, error) {
	return lw.newBackendFromOptions()
}

// newBackendOptions creates RPC backend options from the client configuration
func (lw *LedgerBuilder) newBackendOptions() (*ledgerbackend.RPCLedgerBackendOptions, error) {

	// Validate that endpoint is provided
	if lw.ClientConfig.Endpoint == "" {
		return nil, fmt.Errorf("ClientConfig.Endpoint value is empty, please provide a valid endpoint")
	}

	return &ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: lw.ClientConfig.Endpoint,
		BufferSize:   uint32(lw.ClientConfig.BufferSize),
		HttpClient:   &http.Client{},
	}, nil
}

// newBackendFromOptions constructs the RPC ledger backend using the configured options
func (lw *LedgerBuilder) newBackendFromOptions() (*ledgerbackend.RPCLedgerBackend, error) {
	backendOptions, err := lw.newBackendOptions()

	if err != nil {
		return nil, err
	}

	backend := ledgerbackend.NewRPCLedgerBackend(*backendOptions)

	return backend, nil
}
