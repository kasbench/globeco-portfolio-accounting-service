package external

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	// CircuitBreakerStateClosed means requests are allowed through
	CircuitBreakerStateClosed CircuitBreakerState = iota
	// CircuitBreakerStateOpen means requests are immediately rejected
	CircuitBreakerStateOpen
	// CircuitBreakerStateHalfOpen means limited requests are allowed to test recovery
	CircuitBreakerStateHalfOpen
)

// String returns string representation of circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerStateClosed:
		return "CLOSED"
	case CircuitBreakerStateOpen:
		return "OPEN"
	case CircuitBreakerStateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerStats holds statistics for the circuit breaker
type CircuitBreakerStats struct {
	State                CircuitBreakerState `json:"state"`
	SuccessCount         uint32              `json:"success_count"`
	FailureCount         uint32              `json:"failure_count"`
	RequestCount         uint32              `json:"request_count"`
	LastStateChange      time.Time           `json:"last_state_change"`
	LastSuccessTime      time.Time           `json:"last_success_time"`
	LastFailureTime      time.Time           `json:"last_failure_time"`
	NextAttemptAllowedAt time.Time           `json:"next_attempt_allowed_at"`
}

// CircuitBreakerError represents errors related to circuit breaker
type CircuitBreakerError struct {
	State   CircuitBreakerState
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("circuit breaker %s: %s", e.State.String(), e.Message)
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config               CircuitBreakerConfig
	state                CircuitBreakerState
	successCount         uint32
	failureCount         uint32
	requestCount         uint32
	lastStateChange      time.Time
	lastSuccessTime      time.Time
	lastFailureTime      time.Time
	nextAttemptAllowedAt time.Time
	mu                   sync.RWMutex
	logger               logger.Logger
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig, lg logger.Logger) *CircuitBreaker {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		lg.Error("Invalid circuit breaker configuration", logger.Err(err))
		config.SetDefaults()
	}

	cb := &CircuitBreaker{
		config:          config,
		state:           CircuitBreakerStateClosed,
		lastStateChange: time.Now(),
		logger:          lg,
	}

	lg.Info("Circuit breaker initialized",
		logger.String("state", cb.state.String()),
		logger.Int("failureThreshold", int(config.FailureThreshold)),
		logger.Int("successThreshold", int(config.SuccessThreshold)))

	return cb
}

// Execute executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if request is allowed
	if !cb.allowRequest() {
		return &CircuitBreakerError{
			State:   cb.getState(),
			Message: "circuit breaker is open, requests are not allowed",
		}
	}

	// Execute the function
	err := fn()

	// Record the result
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// allowRequest determines if a request should be allowed through
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		if now.After(cb.nextAttemptAllowedAt) {
			cb.setState(CircuitBreakerStateHalfOpen)
			return true
		}
		return false
	case CircuitBreakerStateHalfOpen:
		return cb.requestCount < cb.config.MaxRequests
	default:
		return false
	}
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.requestCount++
	cb.lastSuccessTime = time.Now()

	if cb.state == CircuitBreakerStateHalfOpen {
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.setState(CircuitBreakerStateClosed)
		}
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.requestCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerStateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.setState(CircuitBreakerStateOpen)
		}
	case CircuitBreakerStateHalfOpen:
		cb.setState(CircuitBreakerStateOpen)
	}
}

// setState changes the circuit breaker state and resets counters
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState
		cb.lastStateChange = time.Now()

		// Reset counters based on new state
		switch newState {
		case CircuitBreakerStateClosed:
			cb.successCount = 0
			cb.failureCount = 0
			cb.requestCount = 0
		case CircuitBreakerStateOpen:
			cb.nextAttemptAllowedAt = time.Now().Add(cb.config.Timeout)
			cb.successCount = 0
			cb.requestCount = 0
		case CircuitBreakerStateHalfOpen:
			cb.successCount = 0
			cb.failureCount = 0
			cb.requestCount = 0
		}

		if cb.config.OnStateChangeEvent {
			cb.logger.Info("Circuit breaker state changed",
				logger.String("from", oldState.String()),
				logger.String("to", newState.String()),
				logger.String("timestamp", cb.lastStateChange.Format(time.RFC3339)))
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) getState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current statistics of the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:                cb.state,
		SuccessCount:         cb.successCount,
		FailureCount:         cb.failureCount,
		RequestCount:         cb.requestCount,
		LastStateChange:      cb.lastStateChange,
		LastSuccessTime:      cb.lastSuccessTime,
		LastFailureTime:      cb.lastFailureTime,
		NextAttemptAllowedAt: cb.nextAttemptAllowedAt,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(CircuitBreakerStateClosed)
	cb.logger.Info("Circuit breaker has been reset")
}

// IsRequestAllowed checks if a request would be allowed without executing it
func (cb *CircuitBreaker) IsRequestAllowed() bool {
	return cb.allowRequest()
}

// Name returns a descriptive name for this circuit breaker
func (cb *CircuitBreaker) Name() string {
	return "http-client-circuit-breaker"
}

// IsCircuitBreakerError checks if an error is a circuit breaker error
func IsCircuitBreakerError(err error) bool {
	_, ok := err.(*CircuitBreakerError)
	return ok
}

// IsCircuitBreakerOpen checks if the circuit breaker error indicates an open state
func IsCircuitBreakerOpen(err error) bool {
	if cbErr, ok := err.(*CircuitBreakerError); ok {
		return cbErr.State == CircuitBreakerStateOpen
	}
	return false
}
