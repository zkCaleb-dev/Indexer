package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExponentialBackoffStrategy_Success(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(3, 10*time.Millisecond, 100*time.Millisecond)

	err := strategy.Execute(context.Background(), func() error {
		return nil // Success on first try
	})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestExponentialBackoffStrategy_SuccessAfterRetries(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(5, 10*time.Millisecond, 100*time.Millisecond)

	attempts := 0
	err := strategy.Execute(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("connection reset by peer") // Recoverable error
		}
		return nil // Success on 3rd attempt
	})

	if err != nil {
		t.Errorf("Expected no error after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
}

func TestExponentialBackoffStrategy_NonRecoverableError(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(5, 10*time.Millisecond, 100*time.Millisecond)

	attempts := 0
	err := strategy.Execute(context.Background(), func() error {
		attempts++
		return errors.New("invalid data") // Non-recoverable error
	})

	if err == nil {
		t.Error("Expected error for non-recoverable failure")
	}

	if attempts != 1 {
		t.Errorf("Expected only 1 attempt for non-recoverable error, got: %d", attempts)
	}
}

func TestExponentialBackoffStrategy_MaxRetriesExceeded(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(3, 10*time.Millisecond, 100*time.Millisecond)

	attempts := 0
	err := strategy.Execute(context.Background(), func() error {
		attempts++
		return errors.New("connection refused") // Always fail with recoverable error
	})

	if err == nil {
		t.Error("Expected error after max retries exceeded")
	}

	expectedAttempts := 4 // 1 initial + 3 retries
	if attempts != expectedAttempts {
		t.Errorf("Expected %d attempts, got: %d", expectedAttempts, attempts)
	}
}

func TestExponentialBackoffStrategy_ContextCancellation(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(10, 100*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := strategy.Execute(ctx, func() error {
		attempts++
		return errors.New("timeout") // Recoverable error
	})

	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should have attempted at least once
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got: %d", attempts)
	}
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"timeout", errors.New("i/o timeout"), true},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"invalid data", errors.New("invalid data format"), false},
		{"permission denied", errors.New("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRecoverableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRecoverableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}
