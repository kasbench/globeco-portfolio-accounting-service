package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"

	// Import for swagger docs generation
	_ "github.com/kasbench/globeco-portfolio-accounting-service/docs"
)

// @title GlobeCo Portfolio Accounting Service API
// @version 1.0
// @description Financial transaction processing and portfolio balance management microservice for GlobeCo benchmarking suite.
// @description
// @description This service processes financial transactions and maintains portfolio account balances with:
// @description - Transaction creation and processing with comprehensive validation
// @description - Balance calculation and portfolio summary generation
// @description - Batch transaction processing for file imports
// @description - Real-time balance updates with optimistic locking
// @description - Integration with portfolio and security services
//
// @contact.name GlobeCo Support
// @contact.email noah@kasbench.org
// @contact.url https://github.com/kasbench/globeco-portfolio-accounting-service
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8087
// @BasePath /api/v1
//
// @schemes http https
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key for service authentication
//
// @tag.name Transactions
// @tag.description Transaction processing and management endpoints
//
// @tag.name Balances
// @tag.description Portfolio balance and position management endpoints
//
// @tag.name Health
// @tag.description Service health and monitoring endpoints
//
// @externalDocs.description GlobeCo Portfolio Accounting Service Documentation
// @externalDocs.url https://github.com/kasbench/globeco-portfolio-accounting-service/blob/main/README.md

const (
	// serviceName is the name of the service
	serviceName = "globeco-portfolio-accounting-service"

	// serviceVersion is the version of the service
	serviceVersion = "1.0.0"

	// gracefulShutdownTimeout is the default timeout for graceful shutdown
	gracefulShutdownTimeout = 30 * time.Second
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger, err := initializeLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	appLogger.Info("Starting GlobeCo Portfolio Accounting Service",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
	)

	// Create main context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection and run migrations
	if err := initializeDatabase(ctx, cfg, appLogger); err != nil {
		appLogger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize and start server
	if err := runServer(ctx, cfg, appLogger); err != nil {
		appLogger.Fatal("Server failed to run", zap.Error(err))
	}

	appLogger.Info("GlobeCo Portfolio Accounting Service stopped gracefully")
}

// runServer initializes and runs the HTTP server
func runServer(ctx context.Context, cfg *config.Config, logger logger.Logger) error {
	// Create server instance
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Channel to capture server errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		)

		if err := server.Start(ctx); err != nil {
			serverErr <- fmt.Errorf("server start failed: %w", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		logger.Error("Server error occurred", zap.Error(err))
		return err
	case sig := <-signalChan:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer shutdownCancel()

		// Gracefully shutdown the server
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown server gracefully", zap.Error(err))
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		logger.Info("Server shutdown completed")
		return nil
	case <-ctx.Done():
		logger.Info("Context cancelled, initiating shutdown")

		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer shutdownCancel()

		// Gracefully shutdown the server
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown server gracefully", zap.Error(err))
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		return nil
	}
}

// initializeLogger creates and configures the application logger
func initializeLogger(cfg config.LoggingConfig) (logger.Logger, error) {
	var loggerInstance logger.Logger
	var err error

	switch cfg.Format {
	case "development", "dev":
		loggerInstance = logger.NewDevelopment()
	case "production", "prod", "json":
		loggerInstance = logger.NewProduction()
	default:
		// Default to development logger
		loggerInstance = logger.NewDevelopment()
	}

	if loggerInstance == nil {
		return nil, fmt.Errorf("failed to create logger instance")
	}

	return loggerInstance, err
}

// getEnvOrDefault returns the value of the environment variable or the default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// mustGetEnv returns the value of the environment variable or panics if not set
func mustGetEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	panic(fmt.Sprintf("Environment variable %s is required", key))
}

// printStartupBanner prints the service startup banner
func printStartupBanner() {
	banner := `
    ╔══════════════════════════════════════════════════════════════╗
    ║                                                              ║
    ║   GlobeCo Portfolio Accounting Service                       ║
    ║   Version: ` + serviceVersion + `                                           ║
    ║                                                              ║
    ║   A microservice for processing financial transactions       ║
    ║   and maintaining portfolio balances.                        ║
    ║                                                              ║
    ╚══════════════════════════════════════════════════════════════╝
`
	fmt.Print(banner)
}

// validateConfiguration validates the loaded configuration
func validateConfiguration(cfg *config.Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Server.Host == "" {
		return fmt.Errorf("server host cannot be empty")
	}

	if cfg.Database.Host == "" {
		return fmt.Errorf("database host cannot be empty")
	}

	if cfg.Database.Database == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	return nil
}

// handlePanic recovers from panics and logs them appropriately
func handlePanic(logger logger.Logger) {
	if r := recover(); r != nil {
		logger.Fatal("Service panicked",
			zap.Any("panic", r),
			zap.Stack("stack"),
		)
	}
}

// printConfiguration logs the current configuration (without sensitive data)
func printConfiguration(cfg *config.Config, logger logger.Logger) {
	logger.Info("Service configuration loaded",
		zap.String("server.host", cfg.Server.Host),
		zap.Int("server.port", cfg.Server.Port),
		zap.Duration("server.read_timeout", cfg.Server.ReadTimeout),
		zap.Duration("server.write_timeout", cfg.Server.WriteTimeout),
		zap.Duration("server.idle_timeout", cfg.Server.IdleTimeout),
		zap.String("database.host", cfg.Database.Host),
		zap.Int("database.port", cfg.Database.Port),
		zap.String("database.database", cfg.Database.Database),
		zap.String("database.ssl_mode", cfg.Database.SSLMode),
		zap.Bool("cache.enabled", cfg.Cache.Enabled),
		zap.String("cache.cluster_name", cfg.Cache.ClusterName),
		zap.Bool("kafka.enabled", cfg.Kafka.Enabled),
		zap.String("logging.level", cfg.Logging.Level),
		zap.String("logging.format", cfg.Logging.Format),
		zap.Bool("metrics.enabled", cfg.Metrics.Enabled),
		zap.Bool("tracing.enabled", cfg.Tracing.Enabled),
	)
}

// initializeDatabase initializes the database connection and runs migrations
func initializeDatabase(ctx context.Context, cfg *config.Config, logger logger.Logger) error {
	logger.Info("Initializing database connection and migrations")

	// Create database connection
	db, err := database.NewConnection(cfg.Database, logger)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Error("Failed to close database connection", zap.Error(closeErr))
		}
	}()

	// Test database connectivity
	if err := db.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	logger.Info("Database connection established successfully")

	// Run migrations if enabled
	if cfg.Database.AutoMigrate {
		logger.Info("Auto-migration is enabled, running database migrations")

		if err := db.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}

		logger.Info("Database migrations completed successfully")
	} else {
		logger.Info("Auto-migration is disabled, skipping database migrations")
	}

	return nil
}
