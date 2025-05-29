package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// DB wraps sqlx.DB with additional functionality
type DB struct {
	*sqlx.DB
	config config.DatabaseConfig
	logger logger.Logger
}

// Connection represents a database connection with transaction support
type Connection interface {
	// Query methods
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row

	// Named query methods
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)

	// Transaction support
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)

	// Health check
	PingContext(ctx context.Context) error

	// Close
	Close() error
}

// NewConnection creates a new database connection
func NewConnection(cfg config.DatabaseConfig, log logger.Logger) (*DB, error) {
	if log == nil {
		log = logger.NewDevelopment()
	}

	// Create connection string
	connStr := cfg.ConnectionString()

	log.Info("Connecting to database",
		logger.String("host", cfg.Host),
		logger.Int("port", cfg.Port),
		logger.String("database", cfg.Database),
		logger.String("user", cfg.User),
	)

	// Open database connection
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Successfully connected to database")

	return &DB{
		DB:     db,
		config: cfg,
		logger: log,
	}, nil
}

// RunMigrations runs database migrations
func (db *DB) RunMigrations() error {
	if db.config.MigrationsPath == "" {
		db.logger.Warn("Migrations path not configured, skipping migrations")
		return nil
	}

	db.logger.Info("Running database migrations",
		logger.String("path", db.config.MigrationsPath),
	)

	// Create postgres driver instance
	driver, err := postgres.WithInstance(db.DB.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", db.config.MigrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		db.logger.Info("No migrations applied")
	} else {
		db.logger.Info("Migrations completed successfully",
			logger.Int("version", int(version)),
			logger.Bool("dirty", dirty),
		)
	}

	return nil
}

// WithTransaction executes a function within a database transaction
func (db *DB) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := db.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				db.logger.Error("Failed to rollback transaction during panic",
					logger.Err(rollbackErr),
				)
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			db.logger.Error("Failed to rollback transaction",
				logger.Err(rollbackErr),
			)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetStats returns database connection statistics
func (db *DB) GetStats() sql.DBStats {
	return db.DB.Stats()
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck(ctx context.Context) error {
	// Test basic connectivity
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test a simple query
	var result int
	if err := db.GetContext(ctx, &result, "SELECT 1"); err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}

	// Check connection pool health
	stats := db.GetStats()
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open connections")
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}

// GetContext wraps sqlx.GetContext with logging
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	start := time.Now()
	err := db.DB.GetContext(ctx, dest, query, args...)
	duration := time.Since(start)

	if err != nil {
		db.logger.Error("Database query failed",
			logger.String("query", query),
			logger.Duration("duration", duration),
			logger.Err(err),
		)
	} else {
		db.logger.Debug("Database query executed",
			logger.String("query", query),
			logger.Duration("duration", duration),
		)
	}

	return err
}

// SelectContext wraps sqlx.SelectContext with logging
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	start := time.Now()
	err := db.DB.SelectContext(ctx, dest, query, args...)
	duration := time.Since(start)

	if err != nil {
		db.logger.Error("Database query failed",
			logger.String("query", query),
			logger.Duration("duration", duration),
			logger.Err(err),
		)
	} else {
		db.logger.Debug("Database query executed",
			logger.String("query", query),
			logger.Duration("duration", duration),
		)
	}

	return err
}

// ExecContext wraps sqlx.ExecContext with logging
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := db.DB.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	if err != nil {
		db.logger.Error("Database exec failed",
			logger.String("query", query),
			logger.Duration("duration", duration),
			logger.Err(err),
		)
	} else {
		db.logger.Debug("Database exec executed",
			logger.String("query", query),
			logger.Duration("duration", duration),
		)
	}

	return result, err
}

// NamedExecContext wraps sqlx.NamedExecContext with logging
func (db *DB) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := db.DB.NamedExecContext(ctx, query, arg)
	duration := time.Since(start)

	if err != nil {
		db.logger.Error("Database named exec failed",
			logger.String("query", query),
			logger.Duration("duration", duration),
			logger.Err(err),
		)
	} else {
		db.logger.Debug("Database named exec executed",
			logger.String("query", query),
			logger.Duration("duration", duration),
		)
	}

	return result, err
}
