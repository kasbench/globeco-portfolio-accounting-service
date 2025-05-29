package database

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// TestDB wraps the test database container and connection
type TestDB struct {
	Container *postgres.PostgresContainer
	DB        *DB
	Config    config.DatabaseConfig
}

// NewTestDatabase creates a new test database using TestContainers
func NewTestDatabase(ctx context.Context) (*TestDB, error) {
	// Get the project root directory for migrations
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..")
	migrationsPath := filepath.Join(projectRoot, "migrations")

	// Create PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17"),
		postgres.WithDatabase("portfolio_accounting_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection details
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	// Create database configuration
	dbConfig := config.DatabaseConfig{
		Host:            host,
		Port:            port.Int(),
		User:            "testuser",
		Password:        "testpass",
		Database:        "portfolio_accounting_test",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 15 * time.Minute,
		MigrationsPath:  migrationsPath,
	}

	// Create database connection
	db, err := NewConnection(dbConfig, logger.NewNoop())
	if err != nil {
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Run migrations
	if err := db.RunMigrations(); err != nil {
		db.Close()
		postgresContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to run test migrations: %w", err)
	}

	return &TestDB{
		Container: postgresContainer,
		DB:        db,
		Config:    dbConfig,
	}, nil
}

// Close closes the test database and terminates the container
func (tdb *TestDB) Close(ctx context.Context) error {
	if tdb.DB != nil {
		tdb.DB.Close()
	}

	if tdb.Container != nil {
		return tdb.Container.Terminate(ctx)
	}

	return nil
}

// Cleanup removes all data from test tables
func (tdb *TestDB) Cleanup(ctx context.Context) error {
	queries := []string{
		"TRUNCATE TABLE balances RESTART IDENTITY CASCADE",
		"TRUNCATE TABLE transactions RESTART IDENTITY CASCADE",
	}

	for _, query := range queries {
		if _, err := tdb.DB.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to cleanup table: %w", err)
		}
	}

	return nil
}

// GetConnectionString returns the connection string for the test database
func (tdb *TestDB) GetConnectionString() string {
	return tdb.Config.ConnectionString()
}

// HealthCheck performs a health check on the test database
func (tdb *TestDB) HealthCheck(ctx context.Context) error {
	return tdb.DB.HealthCheck(ctx)
}

// SetupTestData inserts test data for testing purposes
func (tdb *TestDB) SetupTestData(ctx context.Context) error {
	// Sample test transactions
	testTransactions := []string{
		`INSERT INTO transactions (portfolio_id, security_id, source_id, status, transaction_type, quantity, price, transaction_date)
		 VALUES ('portfolio_1234567890123456', 'security_abcd1234567890123456', 'source_001', 'NEW', 'BUY', 100.00000000, 50.25000000, '2024-01-15')`,

		`INSERT INTO transactions (portfolio_id, security_id, source_id, status, transaction_type, quantity, price, transaction_date)
		 VALUES ('portfolio_1234567890123456', NULL, 'source_002', 'NEW', 'DEP', 10000.00000000, 1.00000000, '2024-01-14')`,

		`INSERT INTO transactions (portfolio_id, security_id, source_id, status, transaction_type, quantity, price, transaction_date)
		 VALUES ('portfolio_1234567890123456', 'security_abcd1234567890123456', 'source_003', 'PROC', 'SELL', -50.00000000, 52.75000000, '2024-01-16')`,
	}

	for _, query := range testTransactions {
		if _, err := tdb.DB.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to insert test transaction: %w", err)
		}
	}

	// Sample test balances
	testBalances := []string{
		`INSERT INTO balances (portfolio_id, security_id, quantity_long, quantity_short, last_updated)
		 VALUES ('portfolio_1234567890123456', 'security_abcd1234567890123456', 50.00000000, 0.00000000, CURRENT_TIMESTAMP)`,

		`INSERT INTO balances (portfolio_id, security_id, quantity_long, quantity_short, last_updated)
		 VALUES ('portfolio_1234567890123456', NULL, 7362.50000000, 0.00000000, CURRENT_TIMESTAMP)`,
	}

	for _, query := range testBalances {
		if _, err := tdb.DB.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to insert test balance: %w", err)
		}
	}

	return nil
}

// TestWithDatabase is a helper function for running tests with a clean database
func TestWithDatabase(ctx context.Context, testFunc func(*TestDB) error) error {
	testDB, err := NewTestDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}
	defer testDB.Close(ctx)

	// Clean up before test
	if err := testDB.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup test database: %w", err)
	}

	// Run the test
	if err := testFunc(testDB); err != nil {
		return err
	}

	// Clean up after test
	if err := testDB.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup test database after test: %w", err)
	}

	return nil
}
