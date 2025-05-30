package external

import (
	"fmt"
	"time"
)

// ClientConfig holds configuration for external service clients
type ClientConfig struct {
	// Basic HTTP configuration
	BaseURL             string        `mapstructure:"base_url" json:"base_url"`
	Timeout             time.Duration `mapstructure:"timeout" json:"timeout"`
	MaxIdleConnections  int           `mapstructure:"max_idle_connections" json:"max_idle_connections"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `mapstructure:"idle_conn_timeout" json:"idle_conn_timeout"`

	// Health check endpoint
	HealthEndpoint string `mapstructure:"health_endpoint" json:"health_endpoint"`

	// Authentication (if needed)
	APIKey      string `mapstructure:"api_key" json:"api_key,omitempty"`
	BearerToken string `mapstructure:"bearer_token" json:"bearer_token,omitempty"`

	// Retry configuration
	Retry RetryConfig `mapstructure:"retry" json:"retry"`

	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker" json:"circuit_breaker"`

	// Logging
	EnableLogging bool `mapstructure:"enable_logging" json:"enable_logging"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int           `mapstructure:"max_attempts" json:"max_attempts"`
	InitialInterval time.Duration `mapstructure:"initial_interval" json:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval" json:"max_interval"`
	BackoffFactor   float64       `mapstructure:"backoff_factor" json:"backoff_factor"`
	EnableJitter    bool          `mapstructure:"enable_jitter" json:"enable_jitter"`
	RetryableErrors []string      `mapstructure:"retryable_errors" json:"retryable_errors"`
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxRequests        uint32        `mapstructure:"max_requests" json:"max_requests"`
	Interval           time.Duration `mapstructure:"interval" json:"interval"`
	Timeout            time.Duration `mapstructure:"timeout" json:"timeout"`
	FailureThreshold   uint32        `mapstructure:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold   uint32        `mapstructure:"success_threshold" json:"success_threshold"`
	OnStateChangeEvent bool          `mapstructure:"on_state_change_event" json:"on_state_change_event"`
}

// PortfolioServiceConfig holds portfolio service specific configuration
type PortfolioServiceConfig struct {
	ClientConfig `mapstructure:",squash"`
	ServiceName  string `mapstructure:"service_name" json:"service_name"`
}

// SecurityServiceConfig holds security service specific configuration
type SecurityServiceConfig struct {
	ClientConfig `mapstructure:",squash"`
	ServiceName  string `mapstructure:"service_name" json:"service_name"`
}

// ExternalServicesConfig holds configuration for all external services
type ExternalServicesConfig struct {
	PortfolioService PortfolioServiceConfig `mapstructure:"portfolio_service" json:"portfolio_service"`
	SecurityService  SecurityServiceConfig  `mapstructure:"security_service" json:"security_service"`
}

// Validate validates the client configuration
func (c *ClientConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if err := c.Retry.Validate(); err != nil {
		return fmt.Errorf("retry config validation failed: %w", err)
	}

	if err := c.CircuitBreaker.Validate(); err != nil {
		return fmt.Errorf("circuit breaker config validation failed: %w", err)
	}

	return nil
}

// SetDefaults sets default values for client configuration
func (c *ClientConfig) SetDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}

	if c.MaxIdleConnections == 0 {
		c.MaxIdleConnections = 100
	}

	if c.MaxIdleConnsPerHost == 0 {
		c.MaxIdleConnsPerHost = 10
	}

	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = 90 * time.Second
	}

	c.Retry.SetDefaults()
	c.CircuitBreaker.SetDefaults()
}

// Validate validates the retry configuration
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts < 0 {
		return fmt.Errorf("max attempts cannot be negative")
	}

	if r.InitialInterval < 0 {
		return fmt.Errorf("initial interval cannot be negative")
	}

	if r.MaxInterval < 0 {
		return fmt.Errorf("max interval cannot be negative")
	}

	if r.BackoffFactor < 1.0 {
		return fmt.Errorf("backoff factor must be >= 1.0")
	}

	return nil
}

// SetDefaults sets default values for retry configuration
func (r *RetryConfig) SetDefaults() {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}

	if r.InitialInterval == 0 {
		r.InitialInterval = 100 * time.Millisecond
	}

	if r.MaxInterval == 0 {
		r.MaxInterval = 5 * time.Second
	}

	if r.BackoffFactor == 0 {
		r.BackoffFactor = 2.0
	}

	if len(r.RetryableErrors) == 0 {
		r.RetryableErrors = []string{
			"connection_error",
			"timeout",
			"service_unavailable",
			"internal_server_error",
			"bad_gateway",
			"gateway_timeout",
		}
	}
}

// Validate validates the circuit breaker configuration
func (cb *CircuitBreakerConfig) Validate() error {
	if cb.FailureThreshold == 0 {
		return fmt.Errorf("failure threshold must be positive")
	}

	if cb.SuccessThreshold == 0 {
		return fmt.Errorf("success threshold must be positive")
	}

	if cb.Interval < 0 {
		return fmt.Errorf("interval cannot be negative")
	}

	if cb.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	return nil
}

// SetDefaults sets default values for circuit breaker configuration
func (cb *CircuitBreakerConfig) SetDefaults() {
	if cb.MaxRequests == 0 {
		cb.MaxRequests = 3
	}

	if cb.Interval == 0 {
		cb.Interval = 60 * time.Second
	}

	if cb.Timeout == 0 {
		cb.Timeout = 60 * time.Second
	}

	if cb.FailureThreshold == 0 {
		cb.FailureThreshold = 5
	}

	if cb.SuccessThreshold == 0 {
		cb.SuccessThreshold = 3
	}
}

// Validate validates the external services configuration
func (e *ExternalServicesConfig) Validate() error {
	if err := e.PortfolioService.Validate(); err != nil {
		return fmt.Errorf("portfolio service config validation failed: %w", err)
	}

	if err := e.SecurityService.Validate(); err != nil {
		return fmt.Errorf("security service config validation failed: %w", err)
	}

	return nil
}

// SetDefaults sets default values for external services configuration
func (e *ExternalServicesConfig) SetDefaults() {
	// Portfolio service defaults
	if e.PortfolioService.ServiceName == "" {
		e.PortfolioService.ServiceName = "portfolio-service"
	}
	if e.PortfolioService.BaseURL == "" {
		e.PortfolioService.BaseURL = "http://globeco-portfolio-service-kafka:8001"
	}
	e.PortfolioService.ClientConfig.SetDefaults()

	// Security service defaults
	if e.SecurityService.ServiceName == "" {
		e.SecurityService.ServiceName = "security-service"
	}
	if e.SecurityService.BaseURL == "" {
		e.SecurityService.BaseURL = "http://globeco-security-service:8000"
	}
	e.SecurityService.ClientConfig.SetDefaults()
}
