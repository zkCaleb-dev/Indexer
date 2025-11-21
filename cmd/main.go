package main

import (
	"flag"
	"log"
	"os"

	"indexer/internal/indexer"

	"github.com/stellar/go/network"
)

func main() {
	// Parsear flags
	var (
		rpcEndpoint = flag.String("rpc", "https://soroban-testnet.stellar.org", "RPC endpoint")
		startLedger = flag.Uint("start", 0, "Ledger inicial (0 = último)")
		networkPass = flag.String("network", network.TestNetworkPassphrase, "Network passphrase")
	)
	flag.Parse()

	// Configurar logger
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Obtener ledger actual si start = 0
	if *startLedger == 0 {
		// TODO: Implementar obtención del último ledger
		*startLedger = 1696100 // Por ahora hardcodeado
	}

	// Crear configuración
	config := indexer.Config{
		RPCEndpoint: *rpcEndpoint,
		StartLedger: uint32(*startLedger),
		NetworkPass: *networkPass,
	}

	// Crear y ejecutar indexador
	idx, err := indexer.New(config)
	if err != nil {
		log.Fatalf("Error creando indexador: %v", err)
	}

	if err := idx.Start(); err != nil {
		log.Fatalf("Error ejecutando indexador: %v", err)
	}

	os.Exit(0)
}
