package ledger

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"

	"indexer/internal/models"
	"indexer/internal/storage"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Processor handles the processing of ledger data
type Processor struct {
	networkPassphrase string
	factoryContractID string
	trackedContracts  map[string]bool
	extractor         *DataExtractor
	repository        storage.Repository
}

// NewProcessor creates a new Processor instance
func NewProcessor(networkPassphrase string, factoryContractID string, repository storage.Repository) *Processor {
	return &Processor{
		networkPassphrase: networkPassphrase,
		factoryContractID: factoryContractID,
		trackedContracts:  make(map[string]bool),
		extractor:         NewDataExtractor(networkPassphrase),
		repository:        repository,
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

	// Extract complete deployment information
	contract, err := p.extractor.ExtractDeployedContract(tx, p.factoryContractID, ledgerSeq)
	if err != nil {
		slog.Error("Failed to extract deployed contract",
			"error", err,
			"tx_hash", tx.Hash.HexString(),
		)
		return
	}

	// Add new contract to tracked contracts
	p.trackedContracts[contract.ContractID] = true

	// Save deployed contract to database
	ctx := context.Background()
	if err := p.repository.SaveDeployedContract(ctx, contract); err != nil {
		slog.Error("Failed to save deployed contract to database",
			"error", err,
			"contract_id", contract.ContractID,
		)
		// Don't return - continue processing even if DB save fails
	}

	// Save initialization events
	if len(contract.InitEvents) > 0 {
		if err := p.repository.SaveContractEvents(ctx, contract.InitEvents); err != nil {
			slog.Error("Failed to save contract events to database",
				"error", err,
				"contract_id", contract.ContractID,
			)
		}
	}

	// Save initialization storage
	if len(contract.InitStorage) > 0 {
		if err := p.repository.SaveStorageEntries(ctx, contract.InitStorage); err != nil {
			slog.Error("Failed to save storage entries to database",
				"error", err,
				"contract_id", contract.ContractID,
			)
		}
	}

	slog.Info("New contract deployed",
		"contract_id", contract.ContractID,
		"deployer", contract.Deployer,
		"fee", contract.FeeCharged,
		"events_count", len(contract.InitEvents),
		"storage_entries", len(contract.InitStorage),
	)

	// Print full contract details in DEBUG mode
	p.printDeployedContract(contract)
}

func (p *Processor) handleTrackedContractTx(tx ingest.LedgerTransaction, contractID string, ledgerSeq uint32) {
	slog.Info("Tracked contract activity",
		"contract_id", contractID,
		"ledger", ledgerSeq,
		"tx_hash", tx.Hash.HexString(),
	)

	// Extract complete activity information
	activity, err := p.extractor.ExtractContractActivity(tx, contractID, ledgerSeq)
	if err != nil {
		slog.Error("Failed to extract contract activity",
			"error", err,
			"contract_id", contractID,
		)
		return
	}

	// Save contract activity to database
	ctx := context.Background()
	if err := p.repository.SaveContractActivity(ctx, activity); err != nil {
		slog.Error("Failed to save contract activity to database",
			"error", err,
			"contract_id", contractID,
		)
		// Don't return - continue processing even if DB save fails
	}

	// Save activity events
	if len(activity.Events) > 0 {
		if err := p.repository.SaveContractEvents(ctx, activity.Events); err != nil {
			slog.Error("Failed to save activity events to database",
				"error", err,
				"contract_id", contractID,
			)
		}
	}

	// Save activity storage changes
	if len(activity.StorageChanges) > 0 {
		if err := p.repository.SaveStorageEntries(ctx, activity.StorageChanges); err != nil {
			slog.Error("Failed to save activity storage changes to database",
				"error", err,
				"contract_id", contractID,
			)
		}
	}

	slog.Info("Contract activity extracted",
		"contract_id", contractID,
		"events_count", len(activity.Events),
		"storage_changes", len(activity.StorageChanges),
		"success", activity.Success,
	)

	// Print full activity details in DEBUG mode
	p.printContractActivity(activity)
}

// printDeployedContract prints the deployed contract in JSON format
func (p *Processor) printDeployedContract(contract *models.DeployedContract) {
	jsonData, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal contract to JSON", "error", err)
		return
	}

	slog.Debug("Deployed contract details", "json", string(jsonData))
}

// printContractActivity prints the contract activity in JSON format
func (p *Processor) printContractActivity(activity *models.ContractActivity) {
	jsonData, err := json.MarshalIndent(activity, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal activity to JSON", "error", err)
		return
	}

	slog.Debug("Contract activity details", "json", string(jsonData))
}
