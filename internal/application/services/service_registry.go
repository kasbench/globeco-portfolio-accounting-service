package services

import (
	"context"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/mappers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// ServiceRegistry manages all application services
type ServiceRegistry struct {
	// Services
	TransactionService   TransactionService
	BalanceService       BalanceService
	FileProcessorService FileProcessorService

	// Configuration
	Config ServiceRegistryConfig
	Logger logger.Logger
}

// ServiceRegistryConfig holds configuration for all services
type ServiceRegistryConfig struct {
	// Transaction service configuration
	Transaction TransactionServiceConfig

	// Balance service configuration
	Balance BalanceServiceConfig

	// File processor configuration
	FileProcessor FileProcessorConfig

	// General configuration
	DefaultTimeout time.Duration
	BatchSize      int
}

// ServiceRegistryDependencies holds all dependencies needed to create services
type ServiceRegistryDependencies struct {
	// Repositories
	TransactionRepo repositories.TransactionRepository
	BalanceRepo     repositories.BalanceRepository

	// Domain services
	TransactionProcessor *services.TransactionProcessor
	TransactionValidator *services.TransactionValidator
	BalanceCalculator    *services.BalanceCalculator

	// Mappers
	TransactionMapper *mappers.TransactionMapper
	BalanceMapper     *mappers.BalanceMapper

	// Logger
	Logger logger.Logger
}

// NewServiceRegistry creates a new service registry with all services
func NewServiceRegistry(deps ServiceRegistryDependencies, config ServiceRegistryConfig) *ServiceRegistry {
	if deps.Logger == nil {
		deps.Logger = logger.NewDevelopment()
	}

	// Set default configuration
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}

	// Set default transaction service configuration
	if config.Transaction.MaxBatchSize == 0 {
		config.Transaction.MaxBatchSize = config.BatchSize
	}
	if config.Transaction.ProcessingTimeout == 0 {
		config.Transaction.ProcessingTimeout = config.DefaultTimeout
	}

	// Set default balance service configuration
	if config.Balance.MaxBulkUpdateSize == 0 {
		config.Balance.MaxBulkUpdateSize = config.BatchSize
	}
	if config.Balance.CacheTimeout == 0 {
		config.Balance.CacheTimeout = 15 * time.Minute
	}
	if config.Balance.HistoryRetentionDays == 0 {
		config.Balance.HistoryRetentionDays = 90
	}

	// Set default file processor configuration
	if config.FileProcessor.MaxRecordsPerBatch == 0 {
		config.FileProcessor.MaxRecordsPerBatch = config.BatchSize
	}
	if config.FileProcessor.TimeoutPerBatch == 0 {
		config.FileProcessor.TimeoutPerBatch = config.DefaultTimeout
	}
	if config.FileProcessor.MaxFileSize == 0 {
		config.FileProcessor.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}

	// Create transaction service
	transactionService := NewTransactionService(
		deps.TransactionRepo,
		deps.BalanceRepo,
		*deps.TransactionProcessor,
		*deps.TransactionValidator,
		deps.TransactionMapper,
		config.Transaction,
		deps.Logger,
	)

	// Create balance service
	balanceService := NewBalanceService(
		deps.BalanceRepo,
		deps.TransactionRepo,
		*deps.BalanceCalculator,
		deps.BalanceMapper,
		config.Balance,
		deps.Logger,
	)

	// Create file processor service
	fileProcessorService := NewFileProcessorService(
		transactionService,
		config.FileProcessor,
		deps.Logger,
	)

	return &ServiceRegistry{
		TransactionService:   transactionService,
		BalanceService:       balanceService,
		FileProcessorService: fileProcessorService,
		Config:               config,
		Logger:               deps.Logger,
	}
}

// HealthCheck performs health checks on all services
func (sr *ServiceRegistry) HealthCheck(ctx context.Context) error {
	sr.Logger.Debug("Performing health check on all services")

	// Check transaction service
	if err := sr.TransactionService.GetServiceHealth(ctx); err != nil {
		sr.Logger.Error("Transaction service health check failed", logger.Err(err))
		return err
	}

	// Check balance service
	if err := sr.BalanceService.GetServiceHealth(ctx); err != nil {
		sr.Logger.Error("Balance service health check failed", logger.Err(err))
		return err
	}

	// Check file processor service
	if err := sr.FileProcessorService.GetServiceHealth(ctx); err != nil {
		sr.Logger.Error("File processor service health check failed", logger.Err(err))
		return err
	}

	sr.Logger.Info("All services are healthy")
	return nil
}

// GetServiceStats returns statistics about all services
func (sr *ServiceRegistry) GetServiceStats(ctx context.Context) (*ServiceStats, error) {
	stats := &ServiceStats{
		Services: make(map[string]ServiceHealthStatus),
	}

	// Check each service individually
	services := map[string]func(context.Context) error{
		"transaction":   sr.TransactionService.GetServiceHealth,
		"balance":       sr.BalanceService.GetServiceHealth,
		"fileProcessor": sr.FileProcessorService.GetServiceHealth,
	}

	allHealthy := true
	for serviceName, healthCheck := range services {
		status := ServiceHealthStatus{
			Name:      serviceName,
			Healthy:   true,
			CheckedAt: time.Now(),
		}

		if err := healthCheck(ctx); err != nil {
			status.Healthy = false
			status.Error = err.Error()
			allHealthy = false
		}

		stats.Services[serviceName] = status
	}

	stats.OverallHealth = allHealthy
	stats.CheckedAt = time.Now()

	return stats, nil
}

// Shutdown gracefully shuts down all services
func (sr *ServiceRegistry) Shutdown(ctx context.Context) error {
	sr.Logger.Info("Shutting down service registry")

	// In a real implementation, this would:
	// 1. Stop accepting new requests
	// 2. Wait for current operations to complete
	// 3. Clean up resources
	// 4. Close database connections
	// 5. Shutdown external service clients

	sr.Logger.Info("Service registry shutdown completed")
	return nil
}

// ServiceStats represents health statistics for all services
type ServiceStats struct {
	OverallHealth bool                           `json:"overallHealth"`
	CheckedAt     time.Time                      `json:"checkedAt"`
	Services      map[string]ServiceHealthStatus `json:"services"`
}

// ServiceHealthStatus represents the health status of a single service
type ServiceHealthStatus struct {
	Name      string    `json:"name"`
	Healthy   bool      `json:"healthy"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}

// DefaultServiceRegistryConfig returns a default configuration for the service registry
func DefaultServiceRegistryConfig() ServiceRegistryConfig {
	return ServiceRegistryConfig{
		DefaultTimeout: 30 * time.Second,
		BatchSize:      1000,
		Transaction: TransactionServiceConfig{
			MaxBatchSize:          1000,
			ProcessingTimeout:     30 * time.Second,
			EnableAsyncProcessing: false,
		},
		Balance: BalanceServiceConfig{
			MaxBulkUpdateSize:    1000,
			HistoryRetentionDays: 90,
			CacheTimeout:         15 * time.Minute,
		},
		FileProcessor: FileProcessorConfig{
			WorkingDirectory:   "./data",
			ErrorFileDirectory: "./data/errors",
			MaxFileSize:        100 * 1024 * 1024, // 100MB
			MaxRecordsPerBatch: 1000,
			TimeoutPerBatch:    5 * time.Minute,
			RequiredHeaders: []string{
				"portfolio_id", "security_id", "source_id", "transaction_type",
				"quantity", "price", "transaction_date",
			},
		},
	}
}
