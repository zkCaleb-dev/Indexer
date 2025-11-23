package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
)

// RPCConfig encapsula la configuración RPC
type RPCConfig struct {
	Endpoint    string
	NetworkPass string
	BufferSize  int
}

// RPCService abstrae el acceso al rpc_backend
type RPCService struct {
	Backend ledgerbackend.LedgerBackend
	config  RPCConfig
}

// NewRPCService crea una nueva instancia del servicio RPC
func NewRPCService(config RPCConfig) (*RPCService, error) {
	// Validar configuración
	if config.Endpoint == "" {
		return nil, fmt.Errorf("RPC endpoint es requerido")
	}

	if config.NetworkPass == "" {
		config.NetworkPass = network.TestNetworkPassphrase
	}

	if config.BufferSize == 0 {
		config.BufferSize = 10
	}

	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: config.Endpoint,
		BufferSize:   uint32(config.BufferSize),
		HttpClient:   &http.Client{},
	})

	return &RPCService{
		Backend: backend,
		config:  config,
	}, nil
}

// PrepareRange prepara un rango de ledgers para procesamiento
func (s *RPCService) PrepareRange(ctx context.Context, start, end *uint32) error {
	var ledgerRange ledgerbackend.Range

	if end == nil {
		// UnboundedRange para streaming continuo
		ledgerRange = ledgerbackend.UnboundedRange(*start)
	} else {
		// BoundedRange para rango específico
		ledgerRange = ledgerbackend.BoundedRange(*start, *end)
	}

	return s.Backend.PrepareRange(ctx, ledgerRange)
}

// GetLedger obtiene un ledger específico
func (s *RPCService) GetLedger(ctx context.Context, sequence uint32) (*xdr.LedgerCloseMeta, error) {
	ledger, err := s.Backend.GetLedger(ctx, sequence)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo ledger %d: %w", sequence, err)
	}
	return &ledger, nil
}

// Close cierra el rpc_backend
func (s *RPCService) Close() error {
	if s.Backend != nil {
		return s.Backend.Close()
	}
	return nil
}
