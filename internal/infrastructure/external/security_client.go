package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/cache"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// SecurityClient represents the interface for security service operations
type SecurityClient interface {
	// GetSecurity retrieves a security by ID
	GetSecurity(ctx context.Context, securityID string) (*SecurityResponse, error)

	// GetSecurities retrieves all securities
	GetSecurities(ctx context.Context) (SecurityListResponse, error)

	// GetSecurityType retrieves a security type by ID
	GetSecurityType(ctx context.Context, securityTypeID string) (*SecurityTypeResponse, error)

	// GetSecurityTypes retrieves all security types
	GetSecurityTypes(ctx context.Context) (SecurityTypeListResponse, error)

	// Health checks security service health
	Health(ctx context.Context) error

	// Close closes the client connections
	Close() error
}

// securityClient implements SecurityClient interface
type securityClient struct {
	httpClient     *http.Client
	config         SecurityServiceConfig
	circuitBreaker *CircuitBreaker
	retrier        *Retrier
	cacheAside     *cache.ExternalServiceCacheAside
	logger         logger.Logger
}

// NewSecurityClient creates a new security service client
func NewSecurityClient(config SecurityServiceConfig, cacheAside *cache.ExternalServiceCacheAside, lg logger.Logger) SecurityClient {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		lg.Error("Invalid security client configuration", logger.Err(err))
		config.SetDefaults()
	}

	// Create HTTP client with timeouts
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConnections,
			MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
			IdleConnTimeout:     config.IdleConnTimeout,
		},
	}

	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(config.CircuitBreaker, lg)

	// Create retrier
	retrier := NewRetrier(config.Retry, lg)

	client := &securityClient{
		httpClient:     httpClient,
		config:         config,
		circuitBreaker: circuitBreaker,
		retrier:        retrier,
		cacheAside:     cacheAside,
		logger:         lg,
	}

	lg.Info("Security client initialized",
		logger.String("baseURL", config.BaseURL),
		logger.Duration("timeout", config.Timeout))

	return client
}

// GetSecurity retrieves a security by ID
func (c *securityClient) GetSecurity(ctx context.Context, securityID string) (*SecurityResponse, error) {
	// Try cache first if cache aside is available
	if c.cacheAside != nil {
		securityData, err := c.cacheAside.GetSecurity(ctx, securityID, func() (interface{}, error) {
			return c.getSecurityFromService(ctx, securityID)
		})
		if err != nil {
			return nil, err
		}

		if security, ok := securityData.(*SecurityResponse); ok {
			return security, nil
		}
	}

	// Fallback to direct service call
	return c.getSecurityFromService(ctx, securityID)
}

// getSecurityFromService retrieves security directly from the service
func (c *securityClient) getSecurityFromService(ctx context.Context, securityID string) (*SecurityResponse, error) {
	url := fmt.Sprintf("%s/api/v1/security/%s", c.config.BaseURL, securityID)

	var security *SecurityResponse
	err := c.executeRequest(ctx, "GET", url, nil, &security, "GetSecurity")
	if err != nil {
		return nil, err
	}

	return security, nil
}

// GetSecurities retrieves all securities
func (c *securityClient) GetSecurities(ctx context.Context) (SecurityListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/securities", c.config.BaseURL)

	var securities SecurityListResponse
	err := c.executeRequest(ctx, "GET", url, nil, &securities, "GetSecurities")
	if err != nil {
		return nil, err
	}

	return securities, nil
}

// GetSecurityType retrieves a security type by ID
func (c *securityClient) GetSecurityType(ctx context.Context, securityTypeID string) (*SecurityTypeResponse, error) {
	url := fmt.Sprintf("%s/api/v1/securityType/%s", c.config.BaseURL, securityTypeID)

	var securityType *SecurityTypeResponse
	err := c.executeRequest(ctx, "GET", url, nil, &securityType, "GetSecurityType")
	if err != nil {
		return nil, err
	}

	return securityType, nil
}

// GetSecurityTypes retrieves all security types
func (c *securityClient) GetSecurityTypes(ctx context.Context) (SecurityTypeListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/securityTypes", c.config.BaseURL)

	var securityTypes SecurityTypeListResponse
	err := c.executeRequest(ctx, "GET", url, nil, &securityTypes, "GetSecurityTypes")
	if err != nil {
		return nil, err
	}

	return securityTypes, nil
}

// Health checks security service health
func (c *securityClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s%s", c.config.BaseURL, c.config.HealthEndpoint)

	return c.executeRequest(ctx, "GET", url, nil, nil, "Health")
}

// executeRequest executes HTTP request with circuit breaker and retry logic
func (c *securityClient) executeRequest(ctx context.Context, method, url string, body io.Reader, result interface{}, operation string) error {
	// Execute with circuit breaker
	return c.circuitBreaker.Execute(ctx, func() error {
		// Execute with retry
		return c.retrier.ExecuteWithRetry(ctx, func() error {
			return c.doHTTPRequest(ctx, method, url, body, result, operation)
		})
	})
}

// doHTTPRequest performs the actual HTTP request
func (c *securityClient) doHTTPRequest(ctx context.Context, method, url string, body io.Reader, result interface{}, operation string) error {
	startTime := time.Now()

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "globeco-portfolio-accounting-service/1.0")

	// Add authentication headers if configured
	if c.config.APIKey != "" {
		req.Header.Set("X-API-Key", c.config.APIKey)
	}
	if c.config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	}

	// Log request if enabled
	if c.config.EnableLogging {
		c.logger.Info("Making HTTP request",
			logger.String("method", method),
			logger.String("url", url),
			logger.String("operation", operation))
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		c.logger.Error("HTTP request failed",
			logger.String("method", method),
			logger.String("url", url),
			logger.String("operation", operation),
			logger.Duration("duration", duration),
			logger.Err(err))
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Log response if enabled
	if c.config.EnableLogging {
		c.logger.Info("HTTP response received",
			logger.String("method", method),
			logger.String("url", url),
			logger.String("operation", operation),
			logger.Int("statusCode", resp.StatusCode),
			logger.Duration("duration", duration))
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)
		if errorMsg == "" {
			errorMsg = http.StatusText(resp.StatusCode)
		}

		serviceErr := NewServiceError(c.config.ServiceName, operation, resp.StatusCode, errorMsg)

		c.logger.Error("HTTP request returned error",
			logger.String("method", method),
			logger.String("url", url),
			logger.String("operation", operation),
			logger.Int("statusCode", resp.StatusCode),
			logger.String("error", errorMsg),
			logger.Duration("duration", duration))

		return serviceErr
	}

	// Parse response if result is provided
	if result != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("failed to parse response JSON: %w", err)
		}
	}

	return nil
}

// Close closes the client connections
func (c *securityClient) Close() error {
	// Close HTTP client connections
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	c.logger.Info("Security client closed")
	return nil
}

// GetStats returns circuit breaker statistics
func (c *securityClient) GetStats() CircuitBreakerStats {
	return c.circuitBreaker.GetStats()
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (c *securityClient) ResetCircuitBreaker() {
	c.circuitBreaker.Reset()
}
