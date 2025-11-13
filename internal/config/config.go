package config

import "fmt"

type Config struct {
	// RPC Server URL
	RPCServerURL string

	// Network passphrase ( mainnet or testnet )
	NetworkPassphrase string

	// Starting ledger sequence ( 0 means start from latest )
	StartLedger uint32

	// Buffer size for RPC requests
	BufferSize uint32
}

// Load returns the configuration for the indexer
// TODO: In the future, load this from env vars or config file
func Load() *Config {
	return &Config{
		// Use a public Stellar RPC endpoint
		// You can also use: https://soroban-mainnet.stellar.org for mainnet
		RPCServerURL: "https://soroban-testnet.stellar.org",

		// Mainnet passphrase use: Public Global Stellar Network ; September 2015
		NetworkPassphrase: "Test SDF Network ; September 2015",

		// Start from latest ( 0 means we'll get it from GetHealth )
		StartLedger: 0,

		// Buffer size for ledger requests
		BufferSize: 10,
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
