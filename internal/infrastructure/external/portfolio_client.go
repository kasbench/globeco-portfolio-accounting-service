package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/cache"
	logutil "github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	logger         logutil.Logger
}

// NewPortfolioClient creates a new portfolio service client
func NewPortfolioClient(cfg PortfolioServiceConfig, httpClient *http.Client, logger logutil.Logger) PortfolioClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   cfg.ClientConfig.Timeout,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
	} else {
		httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)
	}

	if logger == nil {
		logger = logutil.NewDevelopment()
	}

	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid portfolio client configuration", logutil.Err(err))
		cfg.SetDefaults()
	}

	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(cfg.CircuitBreaker, logger)

	// Create retrier
	retrier := NewRetrier(cfg.Retry, logger)

	client := &portfolioClient{
		httpClient:     httpClient,
		config:         cfg,
		circuitBreaker: circuitBreaker,
		retrier:        retrier,
		cacheAside:     nil, // CacheAside is not directly passed to NewPortfolioClient in this version
		logger:         logger,
	}

	logger.Info("Portfolio client initialized",
		logutil.String("baseURL", cfg.BaseURL),
		logutil.Duration("timeout", cfg.Timeout),
	)

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
	url := fmt.Sprintf("%s%s", c.config.BaseURL, c.config.HealthEndpoint)

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
			logutil.String("method", method),
			logutil.String("url", url),
			logutil.String("operation", operation))
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		c.logger.Error("HTTP request failed",
			logutil.String("method", method),
			logutil.String("url", url),
			logutil.String("operation", operation),
			logutil.Duration("duration", duration),
			logutil.Err(err))
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Log response if enabled
	if c.config.EnableLogging {
		c.logger.Info("HTTP response received",
			logutil.String("method", method),
			logutil.String("url", url),
			logutil.String("operation", operation),
			logutil.Int("statusCode", resp.StatusCode),
			logutil.Duration("duration", duration))
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
			logutil.String("method", method),
			logutil.String("url", url),
			logutil.String("operation", operation),
			logutil.Int("statusCode", resp.StatusCode),
			logutil.String("error", errorMsg),
			logutil.Duration("duration", duration))

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
