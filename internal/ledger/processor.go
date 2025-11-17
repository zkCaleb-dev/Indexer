package ledger

import (
	"context"
	"io"
	"log/slog"
	"time"

	"indexer/internal/extraction"
	"indexer/internal/orchestrator"
	"indexer/internal/services"
	"indexer/internal/storage"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// Processor handles the processing of ledger data
type Processor struct {
	networkPassphrase string
	factoryContractID string
	extractor         *extraction.DataExtractor
	repository        storage.Repository
	orchestrator      *orchestrator.Orchestrator // Optional: for new service-based architecture
}

// NewProcessor creates a new Processor instance
func NewProcessor(networkPassphrase string, factoryContractID string, repository storage.Repository) *Processor {
	return &Processor{
		networkPassphrase: networkPassphrase,
		factoryContractID: factoryContractID,
		extractor:         extraction.NewDataExtractor(networkPassphrase),
		repository:        repository,
		orchestrator:      nil, // Will be set later via SetOrchestrator if needed
	}
}

// SetOrchestrator sets the orchestrator for service-based processing (optional)
func (p *Processor) SetOrchestrator(orch *orchestrator.Orchestrator) {
	p.orchestrator = orch
}

// toProcessedTx converts an ingest.LedgerTransaction to *services.ProcessedTx
// Returns a pointer to avoid copying large structs when passing to services
func (p *Processor) toProcessedTx(tx ingest.LedgerTransaction, ledgerSeq uint32, ledgerCloseTime time.Time) *services.ProcessedTx {
	return &services.ProcessedTx{
		Tx:              tx,
		Hash:            tx.Hash.HexString(),
		LedgerSeq:       ledgerSeq,
		LedgerCloseTime: ledgerCloseTime,
		Success:         tx.Successful(),
		IsSoroban:       tx.IsSorobanTx(),
		ContractIDs:     extraction.ExtractAllContractIDs(tx),
	}
}

// Process processes a single ledger and all its transactions
func (p *Processor) Process(ledger xdr.LedgerCloseMeta) error {
	sequence := ledger.LedgerSequence()
	txCount := ledger.CountTransactions()
	ledgerCloseTime := ledger.ClosedAt() // Get actual ledger close timestamp

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
		contractIDs := extraction.ExtractAllContractIDs(tx)

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

			// Process via orchestrator services
			processedTx := p.toProcessedTx(tx, sequence, ledgerCloseTime)
			ctx := context.Background()
			if err := p.orchestrator.ProcessTx(ctx, processedTx); err != nil {
				slog.Error("Orchestrator processing failed", "error", err)
			}

			factoryDeployments++
			continue
		}

		// Process all other Soroban transactions through orchestrator (for ActivityService)
		processedTx := p.toProcessedTx(tx, sequence, ledgerCloseTime)
		ctx := context.Background()
		if err := p.orchestrator.ProcessTx(ctx, processedTx); err != nil {
			slog.Error("Orchestrator processing failed", "error", err)
		}
	}

	if factoryDeployments > 0 {
		slog.Info("Ledger summary",
			"sequence", sequence,
			"total_txs", txIndex,
			"soroban_txs", sorobanCount,
			"deployments", factoryDeployments,
		)
	}

	return nil
}

