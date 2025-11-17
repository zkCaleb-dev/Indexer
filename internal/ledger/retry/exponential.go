package retry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ExponentialBackoffStrategy implements retry with exponential backoff
type ExponentialBackoffStrategy struct {
	maxRetries   int
	initialDelay time.Duration
	maxDelay     time.Duration
}

// NewExponentialBackoffStrategy creates a new ExponentialBackoffStrategy
func NewExponentialBackoffStrategy(maxRetries int, initialDelay, maxDelay time.Duration) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		maxRetries:   maxRetries,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
	}
}

// Execute runs the operation with exponential backoff retry logic
func (s *ExponentialBackoffStrategy) Execute(ctx context.Context, operation Operation) error {
	var lastErr error
	delay := s.initialDelay

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Execute the operation
		err := operation()

		// Success case
		if err == nil {
			if attempt > 0 {
				slog.Info("Operation succeeded after retry",
					"attempt", attempt+1,
					"total_attempts", s.maxRetries+1)
			}
			return nil
		}

		lastErr = err

		// Check if error is recoverable
		if !isRecoverableError(err) {
			slog.Error("Non-recoverable error, failing immediately",
				"error", err,
				"attempt", attempt+1)
			return err
		}

		// If this was the last attempt, return error
		if attempt >= s.maxRetries {
			break
		}

		// Log retry attempt
		slog.Warn("Operation failed, retrying with exponential backoff",
			"attempt", attempt+1,
			"max_attempts", s.maxRetries+1,
			"retry_in_seconds", delay.Seconds(),
			"error", err)

		// Wait with exponential backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Double the delay for next attempt (exponential backoff)
			delay *= 2
			if delay > s.maxDelay {
				delay = s.maxDelay
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", s.maxRetries+1, lastErr)
}

// Name returns the strategy name
func (s *ExponentialBackoffStrategy) Name() string {
	return "ExponentialBackoff"
}

// isRecoverableError determines if an error is recoverable and worth retrying
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors that are typically recoverable
	recoverablePatterns := []string{
		"connection reset by peer",
		"connection refused",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"broken pipe",
		"i/o timeout",
		"eof",
		"tls handshake timeout",
		"no such host",
		"connection timed out",
		"dial tcp",
		"read: connection reset",
		"write: broken pipe",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
