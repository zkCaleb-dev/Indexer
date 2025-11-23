package indexer

import (
	"fmt"
	"indexer/internal/service/ingest"
	"log"
	"os"
	"os/signal"
	"syscall"

	"indexer/internal/indexer/processors"
	"indexer/internal/service"
)

// Config contiene la configuración del indexador
type Config struct {
	RPCEndpoint string
	StartLedger uint32
	NetworkPass string
}

// Indexer es el coordinador principal
type Indexer struct {
	config        Config
	rpcService    *service.RPCService
	ingestService *ingest.OrchestratorService
	processors    []service.Processor
}

// New crea una nueva instancia del indexador
func New(config Config) (*Indexer, error) {
	// Crear servicio RPC
	rpcConfig := service.RPCConfig{
		Endpoint:    config.RPCEndpoint,
		NetworkPass: config.NetworkPass,
		BufferSize:  25,
	}

	rpcService, err := service.NewRPCService(rpcConfig)
	if err != nil {
		return nil, fmt.Errorf("error creando servicio RPC: %w", err)
	}

	// Crear procesadores
	usdcProcessor := processors.NewUSDCTransferProcessor()
	processorList := []service.Processor{usdcProcessor}

	// Crear servicio de ingesta
	ingestService := ingest.NewIngestService(rpcService, processorList)

	// Iniciar consumidor de eventos en background
	go consumeEvents(usdcProcessor)

	return &Indexer{
		config:        config,
		rpcService:    rpcService,
		ingestService: ingestService,
		processors:    processorList,
	}, nil
}

// Start inicia el indexador
func (idx *Indexer) Start() error {
	log.Printf("🚀 Iniciando indexador con RPC: %s", idx.config.RPCEndpoint)

	// Iniciar ingesta
	if err := idx.ingestService.Start(idx.config.StartLedger); err != nil {
		return fmt.Errorf("error iniciando ingesta: %w", err)
	}

	// Configurar manejo de señales
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Esperar señal de terminación
	sig := <-sigChan
	log.Printf("📡 Señal recibida: %v", sig)

	// Detener servicios
	idx.Stop()

	return nil
}

// Stop detiene el indexador
func (idx *Indexer) Stop() {
	log.Println("🛑 Deteniendo indexador...")

	// Detener ingesta
	idx.ingestService.Stop()

	// Cerrar RPC
	if err := idx.rpcService.Close(); err != nil {
		log.Printf("Error cerrando RPC: %v", err)
	}

	log.Println("✅ Indexador detenido")
}

// consumeEvents consume eventos del buffer del procesador
func consumeEvents(processor *processors.USDCTransferProcessor) {
	for event := range processor.GetBuffer() {
		// Por ahora solo loguear, después persistir
		log.Printf("📊 Evento USDC procesado: %+v", event)
		// TODO: Aquí iría la lógica de persistencia a MongoDB
	}
}
