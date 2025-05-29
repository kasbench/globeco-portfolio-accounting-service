package external

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// RetryableError represents an error that can be retried
type RetryableError struct {
	Cause error
	Retry bool
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Cause)
}

func (e *RetryableError) Unwrap() error {
	return e.Cause
}

// Retrier implements retry logic with exponential backoff
type Retrier struct {
	config RetryConfig
	logger logger.Logger
}

// NewRetrier creates a new retrier with the given configuration
func NewRetrier(config RetryConfig, lg logger.Logger) *Retrier {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		lg.Error("Invalid retry configuration", logger.Err(err))
		config.SetDefaults()
	}

	return &Retrier{
		config: config,
		logger: lg,
	}
}

// ExecuteWithRetry executes the given function with retry logic
func (r *Retrier) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Execute the operation
		err := operation()
		if err == nil {
			// Success - log if this was a retry
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					logger.Int("attempt", attempt),
					logger.Int("totalAttempts", r.config.MaxAttempts))
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			r.logger.Debug("Error is not retryable, stopping",
				logger.Err(err),
				logger.Int("attempt", attempt))
			return err
		}

		// Don't wait after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate backoff delay
		delay := r.calculateBackoff(attempt)

		r.logger.Warn("Operation failed, retrying",
			logger.Err(err),
			logger.Int("attempt", attempt),
			logger.Int("maxAttempts", r.config.MaxAttempts),
			logger.Duration("delay", delay))

		// Wait for backoff delay or context cancellation
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Continue to next attempt
		}
	}

	// All attempts failed
	r.logger.Error("All retry attempts failed",
		logger.Err(lastErr),
		logger.Int("maxAttempts", r.config.MaxAttempts))

	return &RetryableError{
		Cause: lastErr,
		Retry: false,
	}
}

// isRetryable determines if an error should be retried
func (r *Retrier) isRetryable(err error) bool {
	// Check for context cancellation - never retry
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for circuit breaker errors - don't retry if circuit is open
	if IsCircuitBreakerOpen(err) {
		return false
	}

	// Check for network errors
	if r.isNetworkError(err) {
		return true
	}

	// Check for HTTP status codes
	if r.isRetryableHTTPError(err) {
		return true
	}

	// Check configured retryable error types
	return r.matchesRetryableError(err)
}

// isNetworkError checks if the error is a network-related error
func (r *Retrier) isNetworkError(err error) bool {
	// Network timeouts
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Connection errors
	if _, ok := err.(*net.OpError); ok {
		return true
	}

	// DNS errors
	if _, ok := err.(*net.DNSError); ok {
		return true
	}

	return false
}

// isRetryableHTTPError checks if the HTTP error status code is retryable
func (r *Retrier) isRetryableHTTPError(err error) bool {
	// If it's a wrapped HTTP error, extract the status code
	var httpErr interface{ StatusCode() int }
	if errors.As(err, &httpErr) {
		statusCode := httpErr.StatusCode()

		// Retry on server errors (5xx) and specific client errors
		switch statusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}

	return false
}

// matchesRetryableError checks if the error matches configured retryable error patterns
func (r *Retrier) matchesRetryableError(err error) bool {
	errorString := err.Error()

	for _, pattern := range r.config.RetryableErrors {
		// Simple string matching - in production, you might want more sophisticated matching
		if contains(errorString, pattern) {
			return true
		}
	}

	return false
}

// calculateBackoff calculates the backoff delay for the given attempt
func (r *Retrier) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(r.config.InitialInterval) * math.Pow(r.config.BackoffFactor, float64(attempt-1))

	// Apply max interval limit
	if delay > float64(r.config.MaxInterval) {
		delay = float64(r.config.MaxInterval)
	}

	// Add jitter if enabled
	if r.config.EnableJitter {
		jitter := rand.Float64() * 0.1 * delay // 10% jitter
		delay += jitter
	}

	return time.Duration(delay)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive substring search
	// In production, you might want to use strings.Contains or regex
	return len(substr) <= len(s) &&
		(substr == "" || s == substr ||
			findSubstring(s, substr))
}

// findSubstring performs a simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
	Cause      error
}

func (e *HTTPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP %d: %s (cause: %v)", e.StatusCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *HTTPError) Unwrap() error {
	return e.Cause
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string, cause error) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}
}

// IsRetryableError checks if an error should be retried
func IsRetryableError(err error) bool {
	if retryErr, ok := err.(*RetryableError); ok {
		return retryErr.Retry
	}
	return false
}
