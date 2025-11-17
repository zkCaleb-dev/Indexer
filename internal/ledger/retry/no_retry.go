package retry

import (
	"context"
)

// NoRetryStrategy executes operations without any retry logic
// This is the default behavior when retry is disabled
type NoRetryStrategy struct{}

// NewNoRetryStrategy creates a new NoRetryStrategy
func NewNoRetryStrategy() *NoRetryStrategy {
	return &NoRetryStrategy{}
}

// Execute runs the operation once without retrying
func (s *NoRetryStrategy) Execute(ctx context.Context, operation Operation) error {
	return operation()
}

// Name returns the strategy name
func (s *NoRetryStrategy) Name() string {
	return "NoRetry"
}
