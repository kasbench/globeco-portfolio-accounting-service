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

// PortfolioClient represents the interface for portfolio service operations
type PortfolioClient interface {
	// GetPortfolio retrieves a portfolio by ID
	GetPortfolio(ctx context.Context, portfolioID string) (*PortfolioResponse, error)

	// GetPortfolios retrieves all portfolios
	GetPortfolios(ctx context.Context) (PortfolioListResponse, error)

	// Health checks portfolio service health
	Health(ctx context.Context) error

	// Close closes the client connections
	Close() error
}

// portfolioClient implements PortfolioClient interface
type portfolioClient struct {
	httpClient     *http.Client
	config         PortfolioServiceConfig
	circuitBreaker *CircuitBreaker
	retrier        *Retrier
	cacheAside     *cache.ExternalServiceCacheAside
	logger         logger.Logger
}

// NewPortfolioClient creates a new portfolio service client
func NewPortfolioClient(config PortfolioServiceConfig, cacheAside *cache.ExternalServiceCacheAside, lg logger.Logger) PortfolioClient {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		lg.Error("Invalid portfolio client configuration", logger.Err(err))
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

	client := &portfolioClient{
		httpClient:     httpClient,
		config:         config,
		circuitBreaker: circuitBreaker,
		retrier:        retrier,
		cacheAside:     cacheAside,
		logger:         lg,
	}

	lg.Info("Portfolio client initialized",
		logger.String("baseURL", config.BaseURL),
		logger.Duration("timeout", config.Timeout))

	return client
}

// GetPortfolio retrieves a portfolio by ID
func (c *portfolioClient) GetPortfolio(ctx context.Context, portfolioID string) (*PortfolioResponse, error) {
	// Try cache first if cache aside is available
	if c.cacheAside != nil {
		portfolioData, err := c.cacheAside.GetPortfolio(ctx, portfolioID, func() (interface{}, error) {
			return c.getPortfolioFromService(ctx, portfolioID)
		})
		if err != nil {
			return nil, err
		}

		if portfolio, ok := portfolioData.(*PortfolioResponse); ok {
			return portfolio, nil
		}
	}

	// Fallback to direct service call
	return c.getPortfolioFromService(ctx, portfolioID)
}

// getPortfolioFromService retrieves portfolio directly from the service
func (c *portfolioClient) getPortfolioFromService(ctx context.Context, portfolioID string) (*PortfolioResponse, error) {
	url := fmt.Sprintf("%s/api/v1/portfolio/%s", c.config.BaseURL, portfolioID)

	var portfolio *PortfolioResponse
	err := c.executeRequest(ctx, "GET", url, nil, &portfolio, "GetPortfolio")
	if err != nil {
		return nil, err
	}

	return portfolio, nil
}

// GetPortfolios retrieves all portfolios
func (c *portfolioClient) GetPortfolios(ctx context.Context) (PortfolioListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/portfolios", c.config.BaseURL)

	var portfolios PortfolioListResponse
	err := c.executeRequest(ctx, "GET", url, nil, &portfolios, "GetPortfolios")
	if err != nil {
		return nil, err
	}

	return portfolios, nil
}

// Health checks portfolio service health
func (c *portfolioClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/", c.config.BaseURL)

	return c.executeRequest(ctx, "GET", url, nil, nil, "Health")
}

// executeRequest executes HTTP request with circuit breaker and retry logic
func (c *portfolioClient) executeRequest(ctx context.Context, method, url string, body io.Reader, result interface{}, operation string) error {
	// Execute with circuit breaker
	return c.circuitBreaker.Execute(ctx, func() error {
		// Execute with retry
		return c.retrier.ExecuteWithRetry(ctx, func() error {
			return c.doHTTPRequest(ctx, method, url, body, result, operation)
		})
	})
}

// doHTTPRequest performs the actual HTTP request
func (c *portfolioClient) doHTTPRequest(ctx context.Context, method, url string, body io.Reader, result interface{}, operation string) error {
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
func (c *portfolioClient) Close() error {
	// Close HTTP client connections
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	c.logger.Info("Portfolio client closed")
	return nil
}

// GetStats returns circuit breaker statistics
func (c *portfolioClient) GetStats() CircuitBreakerStats {
	return c.circuitBreaker.GetStats()
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (c *portfolioClient) ResetCircuitBreaker() {
	c.circuitBreaker.Reset()
}
