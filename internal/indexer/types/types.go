package types

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Processor define la interfaz base para todos los procesadores
type Processor interface {
	Name() string
	ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error
	ProcessTransaction(ctx context.Context, tx ingest.LedgerTransaction) error
}

// LedgerRange define un rango de ledgers a procesar
type LedgerRange struct {
	Start uint32
	End   *uint32 // nil para UnboundedRange
}

// CheckpointStore maneja el estado de sincronización
type CheckpointStore interface {
	GetLastProcessedLedger() (uint32, error)
	SaveCheckpoint(ledger uint32) error
}

// Event representa un evento genérico procesado
type Event struct {
	LedgerSequence uint32
	TxHash         string
	Type           string
	ContractID     string
	Data           map[string]interface{}
}

// USDCTransferEvent representa específicamente una transferencia USDC
type USDCTransferEvent struct {
	Event
	From   string
	To     string
	Amount string // Como string para evitar problemas de precisión
}
