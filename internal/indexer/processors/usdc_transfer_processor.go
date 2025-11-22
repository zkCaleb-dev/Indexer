package processors

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"indexer/internal/indexer/types"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
)

// USDCTransferProcessor procesa transferencias USDC SAC
type USDCTransferProcessor struct {
	contractAddress string
	assetString     string
	buffer          chan types.USDCTransferEvent
}

// NewUSDCTransferProcessor crea un nuevo procesador USDC
func NewUSDCTransferProcessor() *USDCTransferProcessor {
	return &USDCTransferProcessor{
		// USDC mainnet - ajustar para testnet si es necesario
		assetString: "USDC:GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN",
		buffer:      make(chan types.USDCTransferEvent, 1000), // Buffer de eventos
	}
}

func (p *USDCTransferProcessor) Name() string {
	return "USDCTransferProcessor"
}

// ProcessLedger procesa todos los eventos de un ledger
func (p *USDCTransferProcessor) ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error {
	// Por ahora solo logueamos, después implementaremos el procesamiento completo
	log.Printf("[%s] Procesando ledger %d", p.Name(), ledger.LedgerSequence())
	return nil
}

// ProcessTransaction procesa una transacción individual
func (p *USDCTransferProcessor) ProcessTransaction(ctx context.Context, tx ingest.LedgerTransaction) error {
	// Verificar si la transacción tiene metadata Soroban
	if tx.UnsafeMeta.V3 == nil || tx.UnsafeMeta.V3.SorobanMeta == nil {
		return nil // No es una transacción Soroban
	}

	// Obtener hash de la transacción
	txHash := hex.EncodeToString(tx.Result.TransactionHash[:])

	// Obtener ledger sequence
	ledgerSeq := tx.Ledger.LedgerSequence()

	// Iterar sobre eventos Soroban
	for _, event := range tx.UnsafeMeta.V3.SorobanMeta.Events {
		if err := p.processEvent(ctx, event, ledgerSeq, txHash); err != nil {
			log.Printf("Error procesando evento: %v", err)
			// Continuar con otros eventos
		}
	}

	return nil
}

// processEvent procesa un evento individual
func (p *USDCTransferProcessor) processEvent(ctx context.Context, event xdr.ContractEvent, ledgerSeq uint32, txHash string) error {
	// Solo procesar eventos de contrato
	if event.Type != xdr.ContractEventTypeContract {
		return nil
	}

	body := event.Body.MustV0()
	topics := body.Topics

	// Verificar si es un evento transfer
	if len(topics) < 3 {
		return nil
	}

	eventType, ok := topics[0].GetSym()
	if !ok || eventType != xdr.ScSymbol("transfer") {
		return nil
	}

	// Verificar si es USDC (topic[3])
	if len(topics) >= 4 {
		// Convertir ScVal a string para comparar con assetString
		// Por ahora, saltamos esta verificación ya que GetString() no existe
		// TODO: Implementar lógica correcta de verificación del asset
	}

	// Extraer from y to
	from, err := p.addressFromScVal(topics[1])
	if err != nil {
		return fmt.Errorf("error parseando from: %w", err)
	}

	to, err := p.addressFromScVal(topics[2])
	if err != nil {
		return fmt.Errorf("error parseando to: %w", err)
	}

	// Extraer cantidad
	amount, err := p.extractAmount(body.Data)
	if err != nil {
		return fmt.Errorf("error extrayendo cantidad: %w", err)
	}

	// Crear evento
	transferEvent := types.USDCTransferEvent{
		Event: types.Event{
			LedgerSequence: ledgerSeq,
			TxHash:         txHash,
			Type:           "transfer",
			ContractID:     p.contractAddress,
		},
		From:   from,
		To:     to,
		Amount: amount,
	}

	// Enviar al buffer (non-blocking)
	select {
	case p.buffer <- transferEvent:
		log.Printf("🔄 USDC Transfer: %s -> %s: %s USDC (Ledger: %d, Tx: %s)",
			from, to, p.formatUSDC(amount), ledgerSeq, txHash[:8])
	default:
		log.Printf("⚠️  Buffer lleno, descartando evento")
	}

	return nil
}

// addressFromScVal convierte ScVal a dirección string
func (p *USDCTransferProcessor) addressFromScVal(val xdr.ScVal) (string, error) {
	addr, ok := val.GetAddress()
	if !ok {
		return "", fmt.Errorf("no es una dirección válida")
	}

	switch addr.Type {
	case xdr.ScAddressTypeScAddressTypeAccount:
		encoded, err := strkey.Encode(strkey.VersionByteAccountID, addr.AccountId.Ed25519[:])
		if err != nil {
			return "", fmt.Errorf("error encoding account ID: %w", err)
		}
		return encoded, nil
	case xdr.ScAddressTypeScAddressTypeContract:
		encoded, err := strkey.Encode(strkey.VersionByteContract, addr.ContractId[:])
		if err != nil {
			return "", fmt.Errorf("error encoding contract ID: %w", err)
		}
		return encoded, nil
	default:
		return "", fmt.Errorf("tipo de dirección no soportado")
	}
}

// extractAmount extrae la cantidad del campo data
func (p *USDCTransferProcessor) extractAmount(data xdr.ScVal) (string, error) {
	i128, ok := data.GetI128()
	if !ok {
		return "", fmt.Errorf("cantidad no es i128")
	}

	// Convertir a big.Int
	amount := big.NewInt(0)
	hi := big.NewInt(int64(i128.Hi))
	lo := big.NewInt(int64(i128.Lo))

	amount.Lsh(hi, 64)
	amount.Add(amount, lo)

	return amount.String(), nil
}

// formatUSDC formatea la cantidad para display (7 decimales)
func (p *USDCTransferProcessor) formatUSDC(amount string) string {
	val, ok := new(big.Float).SetString(amount)
	if !ok {
		return "0"
	}

	divisor := new(big.Float).SetFloat64(10000000) // 10^7
	result := new(big.Float).Quo(val, divisor)

	return result.Text('f', 2) // 2 decimales para display
}

// GetBuffer retorna el canal de buffer para consumir eventos
func (p *USDCTransferProcessor) GetBuffer() <-chan types.USDCTransferEvent {
	return p.buffer
}
