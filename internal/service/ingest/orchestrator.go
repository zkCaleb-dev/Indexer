package ingest

import (
	"context"
	"fmt"
	"indexer/internal/service/rpc"
	"log"
	"sync"
	"time"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/network"
)

// OrchestratorService coordina la ingesta de ledgers
type OrchestratorService struct {
	ledgerBackend rpc.LedgerBackendHandlerService
	processors    []Processor
	checkpointMgr CheckpointStore

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewIngestService crea un nuevo servicio de ingesta
func NewIngestService(ledgerBackend rpc.LedgerBackendHandlerService, processors []Processor) *OrchestratorService {
	ctx, cancel := context.WithCancel(context.Background())

	return &OrchestratorService{
		ledgerBackend: ledgerBackend,
		processors:    processors,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start inicia el proceso de ingesta
func (s *OrchestratorService) Start(startLedger uint32) error {
	log.Printf("🚀 Iniciando ingesta desde ledger %d", startLedger)

	// Preparar rango unbounded
	if err := s.ledgerBackend.PrepareRange(s.ctx, &startLedger, nil); err != nil {
		return fmt.Errorf("error preparando rango: %w", err)
	}

	s.wg.Add(1)
	go s.ingestLoop(startLedger)

	return nil
}

// ingestLoop es el bucle principal de ingesta
func (s *OrchestratorService) ingestLoop(startLedger uint32) {
	defer s.wg.Done()

	currentLedger := startLedger
	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	ticker := time.NewTicker(2 * time.Second) // Polling cada 2 segundos
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("⏹️  Deteniendo ingesta...")
			return

		case <-ticker.C:
			// Intentar procesar el siguiente ledger
			if err := s.processLedger(currentLedger); err != nil {
				consecutiveErrors++
				log.Printf("❌ Error procesando ledger %d (intento %d/%d): %v",
					currentLedger, consecutiveErrors, maxConsecutiveErrors, err)

				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("🔴 Demasiados errores consecutivos, deteniendo...")
					return
				}

				// Backoff exponencial
				time.Sleep(time.Duration(consecutiveErrors) * time.Second)
				continue
			}

			// Éxito - resetear contador y avanzar
			consecutiveErrors = 0
			log.Printf("✅ Ledger %d procesado exitosamente", currentLedger)
			currentLedger++
		}
	}
}

// processLedger procesa un ledger individual
func (s *OrchestratorService) processLedger(sequence uint32) error {
	// Obtener el backend
	backend, err := s.ledgerBackend.HandleBackend()
	if err != nil {
		return fmt.Errorf("error obteniendo backend: %w", err)
	}

	// Obtener ledger del backend
	ledger, err := backend.GetLedger(s.ctx, sequence)
	if err != nil {
		return fmt.Errorf("error obteniendo ledger: %w", err)
	}

	// Crear transaction reader
	txReader, err := ingest.NewLedgerTransactionReader(
		s.ctx,
		backend,
		network.TestNetworkPassphrase,
		sequence,
	)
	if err != nil {
		return fmt.Errorf("error creando tx reader: %w", err)
	}
	defer txReader.Close()

	// Procesar el ledger con cada procesador
	for _, processor := range s.processors {
		if err := processor.ProcessLedger(s.ctx, ledger); err != nil {
			log.Printf("⚠️  Procesador %s falló en ledger: %v", processor.Name(), err)
			// Continuar con otros procesadores
		}
	}

	// Iterar transacciones
	for {
		tx, err := txReader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break // Fin de transacciones
			}
			return fmt.Errorf("error leyendo transacción: %w", err)
		}

		// Procesar transacción con cada procesador
		for _, processor := range s.processors {
			if err := processor.ProcessTransaction(s.ctx, tx); err != nil {
				log.Printf("⚠️  Procesador %s falló en tx: %v", processor.Name(), err)
				// Continuar con otros procesadores
			}
		}
	}

	return nil
}

// Stop detiene el servicio de ingesta
func (s *OrchestratorService) Stop() {
	log.Println("🛑 Solicitando detención de ingesta...")
	s.cancel()
	s.wg.Wait()
	log.Println("✅ Ingesta detenida")
}
