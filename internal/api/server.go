package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/handlers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/middleware"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/routes"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/mappers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	domainServices "github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/cache"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database/postgresql"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/external"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// Server represents the HTTP server with basic configuration
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     logger.Logger

	// Database
	db *database.DB

	// Cache
	cacheManager *cache.CacheManager

	// External service clients
	portfolioClient external.PortfolioClient
	securityClient  external.SecurityClient

	// Repositories
	transactionRepo repositories.TransactionRepository
	balanceRepo     repositories.BalanceRepository

	// Domain services
	transactionValidator *domainServices.TransactionValidator
	transactionProcessor *domainServices.TransactionProcessor
	balanceCalculator    *domainServices.BalanceCalculator

	// Application services
	transactionService services.TransactionService
	balanceService     services.BalanceService

	// Handler dependencies
	transactionHandler *handlers.TransactionHandler
	balanceHandler     *handlers.BalanceHandler
	healthHandler      *handlers.HealthHandler
	swaggerHandler     *handlers.SwaggerHandler
}

// NewServer creates a new server instance with external service clients
func NewServer(cfg *config.Config, lg logger.Logger) (*Server, error) {
	server := &Server{
		config: cfg,
		logger: lg,
	}

	if err := server.initializeDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := server.initializeCache(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	if err := server.initializeExternalClients(); err != nil {
		return nil, fmt.Errorf("failed to initialize external clients: %w", err)
	}

	if err := server.initializeRepositories(); err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}

	if err := server.initializeDomainServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize domain services: %w", err)
	}

	if err := server.initializeApplicationServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize application services: %w", err)
	}

	if err := server.initializeHandlers(); err != nil {
		return nil, fmt.Errorf("failed to initialize handlers: %w", err)
	}

	if err := server.setupHTTPServer(); err != nil {
		return nil, fmt.Errorf("failed to setup HTTP server: %w", err)
	}

	return server, nil
}

// initializeDatabase initializes database connection
func (s *Server) initializeDatabase() error {
	s.logger.Info("Initializing database connection")

	db, err := database.NewConnection(s.config.Database, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}

	// Test database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	s.db = db
	s.logger.Info("Database connection initialized successfully")
	return nil
}

// initializeCache initializes cache connection
func (s *Server) initializeCache() error {
	s.logger.Info("Initializing cache")

	// Create cache configuration from main config
	cacheConfig := cache.Config{
		Type:          cache.CacheTypeRedis,
		Enabled:       s.config.Cache.Enabled,
		KeyPrefix:     "portfolio-accounting",
		DefaultTTL:    s.config.Cache.TTL,
		EnableMetrics: true,
		EnableLogging: true,
		Redis: cache.RedisConfig{
			Address:      s.config.Cache.Address,
			Password:     s.config.Cache.Password,
			Database:     s.config.Cache.Database,
			DialTimeout:  s.config.Cache.Timeout,
			ReadTimeout:  s.config.Cache.Timeout,
			WriteTimeout: s.config.Cache.Timeout,
		},
	}

	// Create cache manager
	cacheManager, err := cache.NewCacheManager(cacheConfig, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create cache manager: %w", err)
	}

	// Test cache connectivity
	if cacheConfig.Enabled {
		if err := cacheManager.Health(); err != nil {
			s.logger.Warn("Cache health check failed, continuing without cache", 
				zap.Error(err))
			// Don't fail startup if cache is unavailable, just log warning
		} else {
			s.logger.Info("Cache connection established successfully")
		}
	}

	s.cacheManager = cacheManager
	return nil
}

// initializeExternalClients initializes external service clients
func (s *Server) initializeExternalClients() error {
	s.logger.Info("Initializing external service clients")

	// Create portfolio service client configuration
	portfolioConfig := external.PortfolioServiceConfig{
		ClientConfig: external.ClientConfig{
			BaseURL:             fmt.Sprintf("http://%s:%d", s.config.External.PortfolioService.Host, s.config.External.PortfolioService.Port),
			Timeout:             s.config.External.PortfolioService.Timeout,
			MaxIdleConnections:  100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			HealthEndpoint:      s.config.External.PortfolioService.HealthEndpoint,
			Retry: external.RetryConfig{
				MaxAttempts:     s.config.External.PortfolioService.MaxRetries,
				InitialInterval: s.config.External.PortfolioService.RetryBackoff,
				MaxInterval:     5 * time.Second,
				BackoffFactor:   2.0,
				EnableJitter:    true,
			},
			CircuitBreaker: external.CircuitBreakerConfig{
				FailureThreshold: uint32(s.config.External.PortfolioService.CircuitBreakerThreshold),
				SuccessThreshold: 3,
				MaxRequests:      3,
				Interval:         60 * time.Second,
				Timeout:          60 * time.Second,
			},
			EnableLogging: true,
		},
		ServiceName: "portfolio-service",
	}

	// Create security service client configuration
	securityConfig := external.SecurityServiceConfig{
		ClientConfig: external.ClientConfig{
			BaseURL:             fmt.Sprintf("http://%s:%d", s.config.External.SecurityService.Host, s.config.External.SecurityService.Port),
			Timeout:             s.config.External.SecurityService.Timeout,
			MaxIdleConnections:  100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			HealthEndpoint:      s.config.External.SecurityService.HealthEndpoint,
			Retry: external.RetryConfig{
				MaxAttempts:     s.config.External.SecurityService.MaxRetries,
				InitialInterval: s.config.External.SecurityService.RetryBackoff,
				MaxInterval:     5 * time.Second,
				BackoffFactor:   2.0,
				EnableJitter:    true,
			},
			CircuitBreaker: external.CircuitBreakerConfig{
				FailureThreshold: uint32(s.config.External.SecurityService.CircuitBreakerThreshold),
				SuccessThreshold: 3,
				MaxRequests:      3,
				Interval:         60 * time.Second,
				Timeout:          60 * time.Second,
			},
			EnableLogging: true,
		},
		ServiceName: "security-service",
	}

	// Initialize portfolio client with instrumented http.Client
	s.portfolioClient = external.NewPortfolioClient(portfolioConfig, nil, s.logger)

	// Initialize security client with instrumented http.Client
	s.securityClient = external.NewSecurityClient(securityConfig, nil, s.logger)

	s.logger.Info("External service clients initialized",
		zap.String("portfolio_service_url", portfolioConfig.BaseURL),
		zap.String("security_service_url", securityConfig.BaseURL))

	return nil
}

// initializeRepositories initializes repositories
func (s *Server) initializeRepositories() error {
	s.logger.Info("Initializing repositories")

	// Initialize transaction repository
	s.transactionRepo = postgresql.NewTransactionRepository(s.db, s.logger)

	// Initialize balance repository
	s.balanceRepo = postgresql.NewBalanceRepository(s.db, s.logger)

	s.logger.Info("Repositories initialized")
	return nil
}

// initializeDomainServices initializes domain services
func (s *Server) initializeDomainServices() error {
	s.logger.Info("Initializing domain services")

	// Initialize transaction validator
	s.transactionValidator = domainServices.NewTransactionValidator(s.transactionRepo, s.balanceRepo, s.logger)

	// Initialize balance calculator
	s.balanceCalculator = domainServices.NewBalanceCalculator(s.balanceRepo, s.logger)

	// Initialize transaction processor
	s.transactionProcessor = domainServices.NewTransactionProcessor(
		s.transactionRepo,
		s.balanceRepo,
		s.transactionValidator,
		s.balanceCalculator,
		s.logger,
	)

	s.logger.Info("Domain services initialized")
	return nil
}

// initializeApplicationServices initializes application services
func (s *Server) initializeApplicationServices() error {
	s.logger.Info("Initializing application services")

	// Initialize mappers
	transactionMapper := mappers.NewTransactionMapper()
	balanceMapper := mappers.NewBalanceMapper()

	// Initialize transaction service
	transactionServiceConfig := services.TransactionServiceConfig{
		MaxBatchSize:          1000,
		ProcessingTimeout:     30 * time.Second,
		EnableAsyncProcessing: false,
	}

	s.transactionService = services.NewTransactionService(
		s.transactionRepo,
		s.balanceRepo,
		*s.transactionProcessor,
		*s.transactionValidator,
		transactionMapper,
		transactionServiceConfig,
		s.logger,
	)

	// Initialize balance service
	balanceServiceConfig := services.BalanceServiceConfig{
		MaxBulkUpdateSize:    1000,
		CacheTimeout:         15 * time.Minute,
		HistoryRetentionDays: 90,
	}

	s.balanceService = services.NewBalanceService(
		s.balanceRepo,
		s.transactionRepo,
		*s.balanceCalculator,
		balanceMapper,
		balanceServiceConfig,
		s.logger,
	)

	s.logger.Info("Application services initialized")
	return nil
}

// initializeHandlers sets up HTTP handlers with proper dependencies
func (s *Server) initializeHandlers() error {
	s.logger.Info("Initializing HTTP handlers")

	// Initialize handlers with proper services
	s.transactionHandler = handlers.NewTransactionHandler(s.transactionService, s.logger)
	s.balanceHandler = handlers.NewBalanceHandler(s.balanceService, s.logger)
	s.healthHandler = handlers.NewHealthHandler(
		s.portfolioClient,
		s.securityClient,
		s.logger,
		"1.0.0",       // version
		"development", // environment
	)
	s.swaggerHandler = handlers.NewSwaggerHandler(s.logger)

	s.logger.Info("HTTP handlers initialized")
	return nil
}

// setupHTTPServer configures the HTTP server
func (s *Server) setupHTTPServer() error {
	s.logger.Info("Setting up HTTP server")

	// Configure CORS
	corsConfig := middleware.DefaultCORSConfig()

	// Setup router configuration
	routerConfig := routes.Config{
		ServiceName:           "globeco-portfolio-accounting-service",
		Version:               "1.0.0",
		CORSConfig:            corsConfig,
		EnableMetrics:         s.config.Metrics.Enabled,
		EnableEnhancedMetrics: s.config.Metrics.Enhanced.Enabled,
		EnableCORS:            true,
	}

	// Setup router dependencies
	routerDeps := routes.RouterDependencies{
		TransactionHandler: s.transactionHandler,
		BalanceHandler:     s.balanceHandler,
		HealthHandler:      s.healthHandler,
		SwaggerHandler:     s.swaggerHandler,
		Logger:             s.logger,
	}

	// Create router with handlers
	router := routes.SetupRouter(routerConfig, routerDeps)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      router,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}

	s.logger.Info("HTTP server configured",
		zap.String("address", s.httpServer.Addr))

	return nil
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting HTTP server",
		zap.String("address", s.httpServer.Addr))

	// Channel to listen for interrupt signal to gracefully shutdown the server
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	// Channel to listen for interrupt signal to gracefully shutdown the server
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive our signal or an error from the server
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed to start: %w", err)
		}
		return nil
	case sig := <-shutdown:
		s.logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()))

		// Give outstanding requests 30 seconds to complete
		shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Server.GracefulShutdownTimeout)
		defer cancel()

		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Initiating graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Server.GracefulShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	// Close external service clients
	if s.portfolioClient != nil {
		if err := s.portfolioClient.Close(); err != nil {
			s.logger.Error("Failed to close portfolio client", zap.Error(err))
		}
	}

	if s.securityClient != nil {
		if err := s.securityClient.Close(); err != nil {
			s.logger.Error("Failed to close security client", zap.Error(err))
		}
	}

	// Close cache connection
	if s.cacheManager != nil {
		if err := s.cacheManager.Close(); err != nil {
			s.logger.Error("Failed to close cache connection", zap.Error(err))
		}
	}

	// Close database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("Failed to close database connection", zap.Error(err))
		}
	}

	s.logger.Info("Graceful shutdown completed")
	return nil
}

// GetServer returns the underlying HTTP server (for testing)
func (s *Server) GetServer() *http.Server {
	return s.httpServer
}

// HealthCheck performs a basic health check
func (s *Server) HealthCheck(ctx context.Context) error {
	// Basic health check - server is running
	if s.httpServer == nil {
		return fmt.Errorf("HTTP server not initialized")
	}

	// Database health check
	if s.db != nil {
		if err := s.db.HealthCheck(ctx); err != nil {
			return fmt.Errorf("database health check failed: %w", err)
		}
	}

	// Cache health check (if enabled)
	if s.cacheManager != nil && s.cacheManager.IsEnabled() {
		if err := s.cacheManager.Health(); err != nil {
			return fmt.Errorf("cache health check failed: %w", err)
		}
	}

	return nil
}
