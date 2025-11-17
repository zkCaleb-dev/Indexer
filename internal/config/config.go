package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// FactoryConfig represents a factory contract configuration
type FactoryConfig struct {
	ID   string // Contract ID (STRKEY format)
	Type string // Contract type: "single-release" or "multi-release"
}

type Config struct {
	// RPC Server URL
	RPCServerURL string

	// Network passphrase ( mainnet or testnet )
	NetworkPassphrase string

	// Starting ledger sequence ( 0 means start from latest )
	StartLedger uint32

	// Buffer size for RPC ledger retrieval
	// Controls how many ledgers are pre-fetched and kept in memory before processing.
	// Higher values = more memory usage but better throughput and resilience to network latency.
	// Recommended: 150 (balance between memory and performance for optimized processing)
	BufferSize uint32

	// Factory contracts to monitor (supports multiple factories)
	FactoryContracts []FactoryConfig

	// Logging level (debug, info, warn, error)
	LogLevel string

	// Database connection string
	DatabaseURL string

	// HTTP Client configuration
	HTTPTimeout          int // Request timeout in seconds
	HTTPMaxIdleConns     int // Max idle connections across all hosts
	HTTPMaxConnsPerHost  int // Max connections per host
	HTTPIdleConnTimeout  int // Idle connection timeout in seconds

	// Checkpointing configuration
	CheckpointInterval uint32 // Save progress every N ledgers (0 = disable)

	// API Server configuration
	APIServerPort int // HTTP API server port for metrics and REST endpoints
}

// Load returns the configuration for the indexer
func Load() *Config {
	return &Config{

		// Use a public Stellar RPC endpoint
		// You can also use: https://soroban-mainnet.stellar.org for mainnet
		RPCServerURL: getEnv("RPC_SERVER_URL", "https://soroban-testnet.stellar.org"),

		// Mainnet passphrase use: Public Global Stellar Network ; September 2015
		NetworkPassphrase: getEnv("NETWORK_PASSPHRASE", "Test SDF Network ; September 2015"),

		// Start from latest ( 0 means we'll get it from GetHealth )
		StartLedger: getEnvAsUint32("START_LEDGER", 0),

		// Buffer size for ledger requests (optimized for batch processing + compaction)
		BufferSize: getEnvAsUint32("BUFFER_SIZE", 150),

		// Factory contracts to monitor
		FactoryContracts: []FactoryConfig{
			{
				ID:   getEnv("FACTORY_CONTRACT_SINGLE_RELEASE_ID", "CDQPREX7KCYB4KBGSVYOUUMQ5FXT6R4NO6R3LLXUUK3FODVBY2FKNTMZ"),
				Type: "single-release",
			},
			{
				ID:   getEnv("FACTORY_CONTRACT_MULTI_RELEASE_ID", "CCAJPWPKSR6FY5Q5RYT5E3EIZQNDMDFYVVKJ656C5SUOIXQOQ4JQVWGV"),
				Type: "multi-release",
			},
		},

		// Logging level
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Database connection string
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://indexer:indexer_dev_password@localhost:5433/stellar_indexer?sslmode=disable"),

		// HTTP Client configuration (optimized for RPC streaming)
		HTTPTimeout:         getEnvAsInt("HTTP_TIMEOUT_SEC", 60),
		HTTPMaxIdleConns:    getEnvAsInt("HTTP_MAX_IDLE_CONNS", 100),
		HTTPMaxConnsPerHost: getEnvAsInt("HTTP_MAX_CONNS_PER_HOST", 100),
		HTTPIdleConnTimeout: getEnvAsInt("HTTP_IDLE_CONN_TIMEOUT_SEC", 90),

		// Checkpointing configuration
		CheckpointInterval: getEnvAsUint32("CHECKPOINT_INTERVAL", 100), // Save progress every 100 ledgers

		// API Server configuration
		APIServerPort: getEnvAsInt("API_SERVER_PORT", 2112), // Port for metrics and REST API
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

// Helper function: get int env var with default
func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// NewHTTPClient creates an optimized HTTP client with connection pooling
// configured from the Config settings
func (c *Config) NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        c.HTTPMaxIdleConns,
		MaxIdleConnsPerHost: c.HTTPMaxConnsPerHost,
		IdleConnTimeout:     time.Duration(c.HTTPIdleConnTimeout) * time.Second,
		DisableKeepAlives:   false, // Enable keep-alive
		DisableCompression:  false, // Enable compression
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(c.HTTPTimeout) * time.Second,
	}
}
