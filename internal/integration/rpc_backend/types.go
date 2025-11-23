package rpc_backend

// BackendBuilder is a generic interface for building rpc_backends in a modular way.
type BackendBuilder[T any] interface {
	Build() (*T, error)
}

// ClientTimeoutConfig The timeout integration for the RPC client is meant to be used with a time unit of seconds
type ClientTimeoutConfig struct {
	Timeout  int
	Retries  int
	Interval int
}

type ClientConfig struct {
	Endpoint          string
	BufferSize        int
	NetworkPassphrase string
	TimeoutConfig     ClientTimeoutConfig
}
