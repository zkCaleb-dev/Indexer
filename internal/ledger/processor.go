package ledger

import (
	"fmt"
	"io"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Processor handles the processing of ledger data
type Processor struct {
	networkPassphrase string
}

// NewProcessor creates a new Processor instance
func NewProcessor(networkPassphrase string) *Processor {
	return &Processor{
		networkPassphrase: networkPassphrase,
	}
}

// Process processes a single ledger and all its transactions
func (p *Processor) Process(ledger xdr.LedgerCloseMeta) error {
	// Get ledger info
	sequence := ledger.LedgerSequence()
	txCount := ledger.CountTransactions()

	fmt.Printf("\n=== Processing Ledger %d ===\n", sequence)
	fmt.Printf("Transactions: %d\n", txCount)

	// Create a transaction reader for this ledger
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		p.networkPassphrase,
		ledger,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction reader: %w", err)
	}
	defer reader.Close()

	// Iterate through all transactions
	txIndex := 0
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break // No more transactions
		}
		if err != nil {
			return fmt.Errorf("failed to read transaction: %w", err)
		}

		txIndex++
		p.processTransaction(tx, txIndex)
	}

	fmt.Printf("=== Finished Ledger %d ===\n\n", sequence)
	return nil
}

// processTransaction processes a single transaction
func (p *Processor) processTransaction(tx ingest.LedgerTransaction, index int) {
	fmt.Printf("  [Tx #%d] Hash: %x\n", index, tx.Hash)

	// Check if transaction was successful
	successful := tx.Result.Result.Result.Code == xdr.TransactionResultCodeTxSuccess ||
		tx.Result.Result.Result.Code == xdr.TransactionResultCodeTxFeeBumpInnerSuccess

	fmt.Printf("  [Tx #%d] Success: %v\n", index, successful)

	// Get operation count
	var opCount int
	switch tx.Envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		opCount = len(tx.Envelope.V1.Tx.Operations)
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		opCount = len(tx.Envelope.V0.Tx.Operations)
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		innerTx := tx.Envelope.FeeBump.Tx.InnerTx.V1.Tx
		opCount = len(innerTx.Operations)
	}

	fmt.Printf("  [Tx #%d] Operations: %d\n", index, opCount)

	// Check for contract events (Soroban)
	if successful {
		p.checkForContractEvents(tx, index)
	}
}

// checkForContractEvents checks if the transaction has contract events
func (p *Processor) checkForContractEvents(tx ingest.LedgerTransaction, index int) {
	// Contract events are in TransactionMeta V3
	if metaV3, ok := tx.UnsafeMeta.GetV3(); ok {
		// Verificar que SorobanMeta no sea nil
		if metaV3.SorobanMeta != nil {
			// Acceder directamente a Events (que SÍ es un slice)
			totalEvents := len(metaV3.SorobanMeta.Events)

			if totalEvents > 0 {
				fmt.Printf("  [Tx #%d] 🎉 Contract Events Found: %d\n", index, totalEvents)
				// TODO: Extract and process contract events here
			}
		}
	}
}
