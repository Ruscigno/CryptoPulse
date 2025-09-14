package retry

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"

	"go.uber.org/zap"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	Jitter          bool
	RetryableErrors []error
	Logger          *zap.Logger
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		Logger:        zap.NewNop(),
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryWithResult is a function that returns a result and can be retried
type RetryWithResult func() (interface{}, error)

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			if attempt > 1 {
				config.Logger.Info("Operation succeeded after retry",
					zap.Int("attempt", attempt),
					zap.Int("max_attempts", config.MaxAttempts))
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err, config.RetryableErrors) {
			config.Logger.Warn("Non-retryable error encountered",
				zap.Error(err),
				zap.Int("attempt", attempt))
			return err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		delay := calculateDelay(attempt, config)
		config.Logger.Warn("Operation failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Duration("delay", delay))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	config.Logger.Error("Operation failed after all retry attempts",
		zap.Error(lastErr),
		zap.Int("max_attempts", config.MaxAttempts))

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// RetryWithResultFunc executes a function that returns a result with retry logic
func RetryWithResultFunc(ctx context.Context, config RetryConfig, fn RetryWithResult) (interface{}, error) {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			if attempt > 1 {
				config.Logger.Info("Operation succeeded after retry",
					zap.Int("attempt", attempt),
					zap.Int("max_attempts", config.MaxAttempts))
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err, config.RetryableErrors) {
			config.Logger.Warn("Non-retryable error encountered",
				zap.Error(err),
				zap.Int("attempt", attempt))
			return nil, err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		delay := calculateDelay(attempt, config)
		config.Logger.Warn("Operation failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Duration("delay", delay))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	config.Logger.Error("Operation failed after all retry attempts",
		zap.Error(lastErr),
		zap.Int("max_attempts", config.MaxAttempts))

	return nil, fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential backoff delay
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))

	// Apply maximum delay limit
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter if enabled
	if config.Jitter {
		// Use crypto/rand for secure random jitter
		randomBig, err := rand.Int(rand.Reader, big.NewInt(200))
		if err == nil {
			// Convert to float64 in range [-1, 1]
			randomFloat := (float64(randomBig.Int64()) / 100.0) - 1.0
			jitter := delay * 0.1 * randomFloat // ±10% jitter
			delay += jitter
		}
	}

	return time.Duration(delay)
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		// If no specific retryable errors are defined, consider all errors retryable
		return true
	}

	for _, retryableErr := range retryableErrors {
		if err.Error() == retryableErr.Error() {
			return true
		}
	}

	return false
}

// Common retryable error types
var (
	ErrTimeout        = fmt.Errorf("timeout")
	ErrNetworkFailure = fmt.Errorf("network failure")
	ErrServiceBusy    = fmt.Errorf("service busy")
	ErrRateLimited    = fmt.Errorf("rate limited")
)

// IsTemporaryError checks if an error is likely temporary and retryable
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	temporaryPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
		"server busy",
		"internal server error",
		"bad gateway",
		"service temporarily unavailable",
		"gateway timeout",
	}

	for _, pattern := range temporaryPatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}

	return false
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains check
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return string(result)
}

// contains checks if string contains substring
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RetryHTTPRequest creates a retry configuration suitable for HTTP requests
func RetryHTTPRequest() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []error{
			ErrTimeout,
			ErrNetworkFailure,
			ErrServiceBusy,
			ErrRateLimited,
		},
		Logger: zap.NewNop(),
	}
}

// RetryDatabaseOperation creates a retry configuration suitable for database operations
func RetryDatabaseOperation() RetryConfig {
	return RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 1.5,
		Jitter:        true,
		RetryableErrors: []error{
			ErrTimeout,
			ErrNetworkFailure,
		},
		Logger: zap.NewNop(),
	}
}
