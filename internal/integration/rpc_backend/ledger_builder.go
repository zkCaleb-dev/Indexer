package rpc_backend

import (
	"fmt"
	"net/http"

	"github.com/stellar/go/ingest/ledgerbackend"
)

type LedgerBuilder struct {
	ClientConfig ClientConfig
}

// Build will create a new ledgerbackend.RPCLedgerBackend rpc_backend from ClientConfig
func (lw *LedgerBuilder) Build() (*ledgerbackend.RPCLedgerBackend, error) {
	return lw.newBackendFromOptions()
}

// newBackendOptions will create a new rpc_backend options object from the client config
func (lw *LedgerBuilder) newBackendOptions() (*ledgerbackend.RPCLedgerBackendOptions, error) {

	// Check if Endpoint is empty or nil
	if lw.ClientConfig.Endpoint == "" {
		return nil, fmt.Errorf("ClientConfig.Endpoint valuie is empty, please provide a valid endpoint")
	}

	return &ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: lw.ClientConfig.Endpoint,
		BufferSize:   uint32(lw.ClientConfig.BufferSize),
		HttpClient:   &http.Client{},
	}, nil
}

func (lw *LedgerBuilder) newBackendFromOptions() (*ledgerbackend.RPCLedgerBackend, error) {
	backendOptions, err := lw.newBackendOptions()

	if err != nil {
		return nil, err
	}

	backend := ledgerbackend.NewRPCLedgerBackend(*backendOptions)

	return backend, nil
}
