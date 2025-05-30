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
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/external"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// Server represents the HTTP server with basic configuration
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     logger.Logger

	// External service clients
	portfolioClient external.PortfolioClient
	securityClient  external.SecurityClient

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

	if err := server.initializeExternalClients(); err != nil {
		return nil, fmt.Errorf("failed to initialize external clients: %w", err)
	}

	if err := server.initializeHandlers(); err != nil {
		return nil, fmt.Errorf("failed to initialize handlers: %w", err)
	}

	if err := server.setupHTTPServer(); err != nil {
		return nil, fmt.Errorf("failed to setup HTTP server: %w", err)
	}

	return server, nil
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

	// Initialize portfolio client
	s.portfolioClient = external.NewPortfolioClient(portfolioConfig, nil, s.logger)

	// Initialize security client
	s.securityClient = external.NewSecurityClient(securityConfig, nil, s.logger)

	s.logger.Info("External service clients initialized",
		zap.String("portfolio_service_url", portfolioConfig.BaseURL),
		zap.String("security_service_url", securityConfig.BaseURL))

	return nil
}

// initializeHandlers sets up HTTP handlers with minimal dependencies
func (s *Server) initializeHandlers() error {
	s.logger.Info("Initializing HTTP handlers")

	// For now, create handlers with nil services - they will handle gracefully
	// TODO: Initialize proper application services in future iterations
	s.transactionHandler = handlers.NewTransactionHandler(nil, s.logger)
	s.balanceHandler = handlers.NewBalanceHandler(nil, s.logger)
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
		ServiceName:   "globeco-portfolio-accounting-service",
		Version:       "1.0.0",
		Environment:   "development",
		EnableMetrics: true,
		EnableCORS:    true,
		CORSConfig:    corsConfig,
	}

	// Setup router dependencies
	routerDeps := routes.RouterDependencies{
		TransactionHandler: s.transactionHandler,
		BalanceHandler:     s.balanceHandler,
		HealthHandler:      s.healthHandler,
		SwaggerHandler:     s.swaggerHandler,
		Logger:             s.logger,
	}

	// Create router
	router := routes.SetupRouter(routerConfig, routerDeps)

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}

	s.logger.Info("HTTP server configured",
		zap.String("address", addr))

	return nil
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting HTTP server",
		zap.String("address", s.httpServer.Addr))

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-stop:
		s.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		return s.Shutdown(ctx)
	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down")
		return s.Shutdown(ctx)
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

	// TODO: Add comprehensive health checks when dependencies are integrated
	// - Database connection check
	// - Cache connection check
	// - External services check
	// - Application services check

	return nil
}
