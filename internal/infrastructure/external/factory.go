package external

import (
	"context"
	"fmt"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/cache"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// ExternalServiceFactory creates and manages external service clients
type ExternalServiceFactory struct {
	config     ExternalServicesConfig
	cacheAside *cache.ExternalServiceCacheAside
	logger     logger.Logger
}

// NewExternalServiceFactory creates a new external service factory
func NewExternalServiceFactory(config ExternalServicesConfig, cacheAside *cache.ExternalServiceCacheAside, lg logger.Logger) *ExternalServiceFactory {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		lg.Error("Invalid external services configuration", logger.Err(err))
		config.SetDefaults()
	}

	return &ExternalServiceFactory{
		config:     config,
		cacheAside: cacheAside,
		logger:     lg,
	}
}

// CreatePortfolioClient creates a new portfolio service client
func (f *ExternalServiceFactory) CreatePortfolioClient() PortfolioClient {
	return NewPortfolioClient(f.config.PortfolioService, f.cacheAside, f.logger)
}

// CreateSecurityClient creates a new security service client
func (f *ExternalServiceFactory) CreateSecurityClient() SecurityClient {
	return NewSecurityClient(f.config.SecurityService, f.cacheAside, f.logger)
}

// ExternalServiceManager manages all external service clients
type ExternalServiceManager struct {
	portfolioClient PortfolioClient
	securityClient  SecurityClient
	factory         *ExternalServiceFactory
	logger          logger.Logger
}

// NewExternalServiceManager creates a new external service manager
func NewExternalServiceManager(config ExternalServicesConfig, cacheAside *cache.ExternalServiceCacheAside, lg logger.Logger) *ExternalServiceManager {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	factory := NewExternalServiceFactory(config, cacheAside, lg)

	manager := &ExternalServiceManager{
		portfolioClient: factory.CreatePortfolioClient(),
		securityClient:  factory.CreateSecurityClient(),
		factory:         factory,
		logger:          lg,
	}

	lg.Info("External service manager initialized")
	return manager
}

// PortfolioClient returns the portfolio service client
func (m *ExternalServiceManager) PortfolioClient() PortfolioClient {
	return m.portfolioClient
}

// SecurityClient returns the security service client
func (m *ExternalServiceManager) SecurityClient() SecurityClient {
	return m.securityClient
}

// Health checks the health of all external services
func (m *ExternalServiceManager) Health(ctx context.Context) error {
	// Check portfolio service health
	if err := m.portfolioClient.Health(ctx); err != nil {
		m.logger.Error("Portfolio service health check failed", logger.Err(err))
		return fmt.Errorf("portfolio service health check failed: %w", err)
	}

	// Check security service health
	if err := m.securityClient.Health(ctx); err != nil {
		m.logger.Error("Security service health check failed", logger.Err(err))
		return fmt.Errorf("security service health check failed: %w", err)
	}

	m.logger.Debug("All external services are healthy")
	return nil
}

// Close closes all external service clients
func (m *ExternalServiceManager) Close() error {
	var closeErrors []error

	// Close portfolio client
	if err := m.portfolioClient.Close(); err != nil {
		closeErrors = append(closeErrors, fmt.Errorf("failed to close portfolio client: %w", err))
	}

	// Close security client
	if err := m.securityClient.Close(); err != nil {
		closeErrors = append(closeErrors, fmt.Errorf("failed to close security client: %w", err))
	}

	if len(closeErrors) > 0 {
		// Return the first error (in production, you might want to aggregate all errors)
		return closeErrors[0]
	}

	m.logger.Info("External service manager closed")
	return nil
}

// GetStats returns statistics for all external service clients
func (m *ExternalServiceManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Get portfolio client stats
	if portfolioClient, ok := m.portfolioClient.(*portfolioClient); ok {
		stats["portfolio_service"] = portfolioClient.GetStats()
	}

	// Get security client stats
	if securityClient, ok := m.securityClient.(*securityClient); ok {
		stats["security_service"] = securityClient.GetStats()
	}

	return stats
}

// ResetCircuitBreakers resets all circuit breakers to closed state
func (m *ExternalServiceManager) ResetCircuitBreakers() {
	// Reset portfolio client circuit breaker
	if portfolioClient, ok := m.portfolioClient.(*portfolioClient); ok {
		portfolioClient.ResetCircuitBreaker()
	}

	// Reset security client circuit breaker
	if securityClient, ok := m.securityClient.(*securityClient); ok {
		securityClient.ResetCircuitBreaker()
	}

	m.logger.Info("All circuit breakers have been reset")
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// GetServiceStatuses returns the status of all external services
func (m *ExternalServiceManager) GetServiceStatuses(ctx context.Context) []ServiceStatus {
	var statuses []ServiceStatus

	// Check portfolio service
	portfolioStatus := ServiceStatus{Name: "portfolio-service", Available: true}
	if err := m.portfolioClient.Health(ctx); err != nil {
		portfolioStatus.Available = false
		portfolioStatus.Error = err.Error()
	}
	statuses = append(statuses, portfolioStatus)

	// Check security service
	securityStatus := ServiceStatus{Name: "security-service", Available: true}
	if err := m.securityClient.Health(ctx); err != nil {
		securityStatus.Available = false
		securityStatus.Error = err.Error()
	}
	statuses = append(statuses, securityStatus)

	return statuses
}
