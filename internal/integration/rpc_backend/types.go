package rpc_backend

// BackendBuilder is a generic interface for building backend instances in a modular way
type BackendBuilder[T any] interface {
	Build() (*T, error)
}

// ClientTimeoutConfig holds timeout settings for the RPC client (all values in seconds)
type ClientTimeoutConfig struct {
	Timeout  int // Request timeout in seconds
	Retries  int // Number of retry attempts
	Interval int // Interval between retries in seconds
}

// ClientConfig contains the configuration for connecting to an RPC endpoint
type ClientConfig struct {
	Endpoint          string              // RPC server endpoint URL
	BufferSize        int                 // Number of ledgers to buffer
	NetworkPassphrase string              // Stellar network passphrase
	TimeoutConfig     ClientTimeoutConfig // Timeout and retry configuration
}
