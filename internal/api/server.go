package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/handlers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/middleware"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/routes"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// Server represents the HTTP server with basic configuration
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     logger.Logger

	// Handler dependencies (simplified for now)
	transactionHandler *handlers.TransactionHandler
	balanceHandler     *handlers.BalanceHandler
	healthHandler      *handlers.HealthHandler
}

// NewServer creates a new server instance with simplified dependencies
func NewServer(cfg *config.Config, lg logger.Logger) (*Server, error) {
	server := &Server{
		config: cfg,
		logger: lg,
	}

	if err := server.initializeHandlers(); err != nil {
		return nil, fmt.Errorf("failed to initialize handlers: %w", err)
	}

	if err := server.setupHTTPServer(); err != nil {
		return nil, fmt.Errorf("failed to setup HTTP server: %w", err)
	}

	return server, nil
}

// initializeHandlers sets up HTTP handlers with minimal dependencies
func (s *Server) initializeHandlers() error {
	s.logger.Info("Initializing HTTP handlers")

	// For now, create handlers with nil services - they will handle gracefully
	// TODO: Initialize proper application services in future iterations
	s.transactionHandler = handlers.NewTransactionHandler(nil, s.logger)
	s.balanceHandler = handlers.NewBalanceHandler(nil, s.logger)
	s.healthHandler = handlers.NewHealthHandler(
		nil, // transaction service
		nil, // balance service
		s.logger,
		"1.0.0",       // version
		"development", // environment
	)

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

	// TODO: Add resource cleanup when other services are properly integrated

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
