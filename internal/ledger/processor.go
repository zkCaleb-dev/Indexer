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
	factoryContractID string
	trackedContracts  map[string]bool
}

// NewProcessor creates a new Processor instance
func NewProcessor(networkPassphrase string, factoryContractID string) *Processor {
	return &Processor{
		networkPassphrase: networkPassphrase,
		factoryContractID: factoryContractID,
		trackedContracts:  make(map[string]bool),
	}
}

// Process processes a single ledger and all its transactions
func (p *Processor) Process(ledger xdr.LedgerCloseMeta) error {
	sequence := ledger.LedgerSequence()
	txCount := ledger.CountTransactions()

	fmt.Printf("\n=== Processing Ledger %d (Txs: %d) ===\n", sequence, txCount)
	fmt.Printf("🏭 Factory looking for: %s\n", p.factoryContractID)

	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		p.networkPassphrase,
		ledger,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction reader: %w", err)
	}
	defer reader.Close()

	txIndex := 0
	sorobanCount := 0

	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break
		}
		txIndex++

		// Debug cada tx
		successful := tx.Successful()
		isSoroban := tx.IsSorobanTx()

		fmt.Printf("  [Tx #%d] Success=%v Soroban=%v Hash=%x\n",
			txIndex, successful, isSoroban, tx.Hash[:8])

		if !successful {
			fmt.Printf("    ❌ Skipped: Failed\n")
			continue
		}

		if !isSoroban {
			fmt.Printf("    ⚪ Skipped: Not Soroban\n")
			continue
		}

		sorobanCount++
		fmt.Printf("    ⭐ SOROBAN TX #%d\n", sorobanCount)

		// Extraer TODOS los contract IDs del footprint
		contractIDs := p.extractAllContractIDs(tx)
		fmt.Printf("    📍 Contract IDs found: %v\n", contractIDs)

		// Verificar si el factory está en alguno de los contract IDs
		isFactory := false
		for _, contractID := range contractIDs {
			if contractID == p.factoryContractID {
				isFactory = true
				break
			}
		}

		fmt.Printf("    🔍 Factory match: %v\n", isFactory)

		if isFactory {
			fmt.Printf("\n🎯🎯🎯 FACTORY MATCH! 🎯🎯🎯\n")
			p.handleFactoryDeployment(tx, sequence)
			continue
		}

		// Check tracked contracts
		foundTracked := false
		for _, contractID := range contractIDs {
			if p.trackedContracts[contractID] {
				p.handleTrackedContractTx(tx, contractID, sequence)
				foundTracked = true
				break
			}
		}

		if !foundTracked {
			fmt.Printf("    ℹ️  Not tracked: %v\n", contractIDs)
		}
	}

	fmt.Printf("📊 Summary: %d total txs, %d soroban txs\n", txIndex, sorobanCount)
	return nil
}

// extractAllContractIDs extracts all contract IDs from the transaction footprint
func (p *Processor) extractAllContractIDs(tx ingest.LedgerTransaction) []string {
	var contractIDs []string
	seen := make(map[string]bool) // Para evitar duplicados

	v1Envelope, ok := tx.GetTransactionV1Envelope()
	if !ok {
		return contractIDs
	}

	// Helper para extraer contract ID de un ledger key
	extractFromKey := func(ledgerKey xdr.LedgerKey) {
		contractData, ok := ledgerKey.GetContractData()
		if !ok {
			return
		}

		// Convertir a formato strkey (C...)
		contractIdStr, err := contractData.Contract.String()
		if err != nil {
			return
		}

		if contractIdStr != "" && !seen[contractIdStr] {
			contractIDs = append(contractIDs, contractIdStr)
			seen[contractIdStr] = true
		}
	}

	// Iterar sobre ReadWrite footprint
	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadWrite {
		extractFromKey(ledgerKey)
	}

	// Iterar sobre ReadOnly footprint
	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadOnly {
		extractFromKey(ledgerKey)
	}

	return contractIDs
}

func (p *Processor) handleFactoryDeployment(tx ingest.LedgerTransaction, ledgerSeq uint32) {
	fmt.Printf("\n🏭 FACTORY DEPLOYMENT DETECTED!\n")
	fmt.Printf("  Ledger: %d\n", ledgerSeq)
	fmt.Printf("  Tx Hash: %x\n", tx.Hash)

	// Extraer nuevo contract ID del ReturnValue
	// El factory devuelve el nuevo contract ID en SorobanMeta.ReturnValue
	if metaV3, ok := tx.UnsafeMeta.GetV3(); ok {
		if metaV3.SorobanMeta != nil {
			// TODO: parsear returnValue para obtener nuevo contract ID
			fmt.Printf("  Return Value: %v\n", metaV3.SorobanMeta.ReturnValue)

			// Por ahora, agregar factory a trackeados (placeholder)
			// En siguiente paso extraeremos el contract ID real
		}
	}
}

func (p *Processor) handleTrackedContractTx(tx ingest.LedgerTransaction, contractID string, ledgerSeq uint32) {
	fmt.Printf("\n📝 TRACKED CONTRACT ACTIVITY\n")
	fmt.Printf("  Contract: %s\n", contractID)
	fmt.Printf("  Ledger: %d\n", ledgerSeq)
	fmt.Printf("  Tx Hash: %x\n", tx.Hash)

	// Extraer eventos
	events, err := tx.GetContractEvents()
	if err == nil && len(events) > 0 {
		fmt.Printf("  🎉 Events: %d\n", len(events))
		for i, event := range events {
			fmt.Printf("    Event #%d:\n", i+1)
			fmt.Printf("      Topics: %d\n", len(event.Body.V0.Topics))
			// TODO: parsear topics y data
		}
	}

	// Extraer cambios de estado
	changes, err := tx.GetChanges()
	if err == nil {
		contractDataChanges := 0
		for _, change := range changes {
			if change.Type == xdr.LedgerEntryTypeContractData {
				contractDataChanges++
			}
		}
		if contractDataChanges > 0 {
			fmt.Printf("  💾 Storage Changes: %d\n", contractDataChanges)
		}
	}
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
