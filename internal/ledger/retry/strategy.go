package retry

import (
	"context"
	"log/slog"
)

// Strategy defines the interface for retry strategies
type Strategy interface {
	// Execute runs the operation with the configured retry logic
	Execute(ctx context.Context, operation Operation) error

	// Name returns the name of the strategy for logging
	Name() string
}

// Operation is a function that can be retried
type Operation func() error

// OperationInfo contains metadata about the operation being retried
type OperationInfo struct {
	Name     string
	Sequence uint32
}

// NewStrategy creates a retry strategy based on configuration
func NewStrategy(config Config) Strategy {
	if !config.Enabled {
		slog.Info("Retry disabled, using NoRetryStrategy")
		return NewNoRetryStrategy()
	}

	slog.Info("Retry enabled, using ExponentialBackoffStrategy",
		"max_retries", config.MaxRetries,
		"initial_delay_sec", config.InitialDelay,
		"max_delay_sec", config.MaxDelay,
	)

	return NewExponentialBackoffStrategy(
		config.MaxRetries,
		config.InitialDelay,
		config.MaxDelay,
	)
}
