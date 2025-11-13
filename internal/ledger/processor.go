package ledger

import (
	"io"
	"log/slog"

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

	slog.Debug("Processing ledger",
		"sequence", sequence,
		"tx_count", txCount,
		"factory", p.factoryContractID,
	)

	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		p.networkPassphrase,
		ledger,
	)
	if err != nil {
		slog.Error("Failed to create transaction reader",
			"sequence", sequence,
			"error", err,
		)
		return err
	}
	defer reader.Close()

	txIndex := 0
	sorobanCount := 0
	factoryDeployments := 0
	trackedActivities := 0

	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break
		}
		txIndex++

		successful := tx.Successful()
		isSoroban := tx.IsSorobanTx()

		slog.Debug("Transaction found",
			"tx_index", txIndex,
			"success", successful,
			"soroban", isSoroban,
			"hash", tx.Hash.HexString()[:16],
		)

		if !successful {
			continue
		}

		if !isSoroban {
			continue
		}

		sorobanCount++

		// Extraer TODOS los contract IDs del footprint
		contractIDs := p.extractAllContractIDs(tx)

		slog.Debug("Soroban transaction processed",
			"tx_index", txIndex,
			"contract_ids", contractIDs,
		)

		// Verificar si el factory está en alguno de los contract IDs
		isFactory := false
		for _, contractID := range contractIDs {
			if contractID == p.factoryContractID {
				isFactory = true
				break
			}
		}

		if isFactory {
			slog.Info("✅ New contract deployment detected",
				"ledger", sequence,
				"tx_hash", tx.Hash.HexString(),
			)
			p.handleFactoryDeployment(tx, sequence)
			factoryDeployments++
			continue
		}

		// Check tracked contracts
		foundTracked := false
		for _, contractID := range contractIDs {
			if p.trackedContracts[contractID] {
				p.handleTrackedContractTx(tx, contractID, sequence)
				foundTracked = true
				trackedActivities++
				break
			}
		}

		if !foundTracked {
			slog.Debug("Contracts not tracked", "contract_ids", contractIDs)
		}
	}

	if factoryDeployments > 0 || trackedActivities > 0 {
		slog.Info("Ledger summary",
			"sequence", sequence,
			"total_txs", txIndex,
			"soroban_txs", sorobanCount,
			"deployments", factoryDeployments,
			"tracked_activities", trackedActivities,
		)
	}

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
	slog.Info("Processing factory deployment",
		"ledger", ledgerSeq,
		"tx_hash", tx.Hash.HexString(),
	)

	// Extraer nuevo contract ID del ReturnValue
	// El factory devuelve el nuevo contract ID en SorobanMeta.ReturnValue
	if metaV3, ok := tx.UnsafeMeta.GetV3(); ok {
		if metaV3.SorobanMeta != nil {
			// TODO: parsear returnValue para obtener nuevo contract ID
			slog.Debug("Return value found",
				"value", metaV3.SorobanMeta.ReturnValue,
			)

			// Por ahora, agregar factory a trackeados (placeholder)
			// En siguiente paso extraeremos el contract ID real
		}
	}
}

func (p *Processor) handleTrackedContractTx(tx ingest.LedgerTransaction, contractID string, ledgerSeq uint32) {
	slog.Info("Tracked contract activity",
		"contract_id", contractID,
		"ledger", ledgerSeq,
		"tx_hash", tx.Hash.HexString(),
	)

	// Extraer eventos
	events, err := tx.GetContractEvents()
	if err == nil && len(events) > 0 {
		slog.Info("Contract events found",
			"contract_id", contractID,
			"event_count", len(events),
		)
		for i, event := range events {
			slog.Debug("Contract event",
				"event_index", i+1,
				"topics_count", len(event.Body.V0.Topics),
			)
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
			slog.Info("Storage changes detected",
				"contract_id", contractID,
				"changes_count", contractDataChanges,
			)
		}
	}
}
