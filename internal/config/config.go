package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// RPC Server URL
	RPCServerURL string

	// Network passphrase ( mainnet or testnet )
	NetworkPassphrase string

	// Starting ledger sequence ( 0 means start from latest )
	StartLedger uint32

	// Buffer size for RPC requests
	BufferSize uint32

	FactoryContractID string
}

// Load returns the configuration for the indexer
// TODO: In the future, load this from env vars or config file
func Load() *Config {
	return &Config{

		// Use a public Stellar RPC endpoint
		// You can also use: https://soroban-mainnet.stellar.org for mainnet
		RPCServerURL: getEnv("RPC_SERVER_URL", "https://soroban-testnet.stellar.org"),

		// Mainnet passphrase use: Public Global Stellar Network ; September 2015
		NetworkPassphrase: getEnv("NETWORK_PASSPHRASE", "Test SDF Network ; September 2015"),

		// Start from latest ( 0 means we'll get it from GetHealth )
		StartLedger: getEnvAsUint32("START_LEDGER", 0),

		// Buffer size for ledger requests
		BufferSize: getEnvAsUint32("BUFFER_SIZE", 10),

		FactoryContractID: getEnv("FACTORY_CONTRACT_SINGLE_RELEASE_ID", "CDQPREX7KCYB4KBGSVYOUUMQ5FXT6R4NO6R3LLXUUK3FODVBY2FKNTMZ"),
	}
}

// Validate chacks if the configuration is valid
func (c *Config) Validate() error {
	if c.RPCServerURL == "" {
		return fmt.Errorf("RPCServerURL is required")
	}
	if c.NetworkPassphrase == "" {
		return fmt.Errorf("NetworkPassphrase is required")
	}
	return nil
}

// Helper function: get string env var with default
func getEnv(key string, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

// Helper function: get uint32 env var with default
func getEnvAsUint32(key string, defaultVal uint32) uint32 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}

	val, err := strconv.ParseUint(valStr, 10, 32)
	if err != nil {
		return defaultVal
	}
	return uint32(val)
}
