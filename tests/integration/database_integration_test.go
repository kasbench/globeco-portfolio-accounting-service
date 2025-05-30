package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
)

type IntegrationTestSuite struct {
	ctx               context.Context
	postgresContainer *postgres.PostgresContainer
	db                *sqlx.DB
}

func setupIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)

	// Run migrations
	err = runMigrations(db)
	require.NoError(t, err)

	return &IntegrationTestSuite{
		ctx:               ctx,
		postgresContainer: postgresContainer,
		db:                db,
	}
}

func (suite *IntegrationTestSuite) teardown(t *testing.T) {
	if suite.db != nil {
		suite.db.Close()
	}
	if suite.postgresContainer != nil {
		err := suite.postgresContainer.Terminate(suite.ctx)
		require.NoError(t, err)
	}
}

func runMigrations(db *sqlx.DB) error {
	// Create transactions table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id SERIAL PRIMARY KEY,
			portfolio_id CHAR(24) NOT NULL,
			security_id CHAR(24),
			source_id VARCHAR(50) NOT NULL,
			status CHAR(5) NOT NULL DEFAULT 'NEW',
			transaction_type CHAR(5) NOT NULL,
			quantity DECIMAL(18,8) NOT NULL,
			price DECIMAL(18,8) NOT NULL,
			transaction_date DATE NOT NULL DEFAULT CURRENT_DATE,
			reprocessing_attempts INTEGER DEFAULT 0,
			version INTEGER NOT NULL DEFAULT 1,
			error_message TEXT,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create balances table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS balances (
			id SERIAL PRIMARY KEY,
			portfolio_id CHAR(24) NOT NULL,
			security_id CHAR(24),
			quantity_long DECIMAL(18,8) NOT NULL DEFAULT 0,
			quantity_short DECIMAL(18,8) NOT NULL DEFAULT 0,
			last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS transaction_source_ndx ON transactions (source_id);
		CREATE UNIQUE INDEX IF NOT EXISTS balance_portfolio_security_ndx ON balances (portfolio_id, security_id);
	`)
	return err
}

func TestDatabaseIntegration_BasicOperations(t *testing.T) {
	suite := setupIntegrationTestSuite(t)
	defer suite.teardown(t)

	t.Run("Database connection and table creation", func(t *testing.T) {
		// Test basic connectivity
		err := suite.db.Ping()
		require.NoError(t, err)

		// Verify tables exist
		var count int
		err = suite.db.Get(&count, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'transactions'")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		err = suite.db.Get(&count, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'balances'")
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Transaction CRUD operations", func(t *testing.T) {
		// Insert transaction
		insertQuery := `
			INSERT INTO transactions (portfolio_id, security_id, source_id, status, transaction_type, quantity, price, transaction_date, version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id`

		var transactionID int64
		err := suite.db.Get(&transactionID, insertQuery,
			"PORTFOLIO123456789012345",
			"SECURITY1234567890123456",
			"SOURCE001",
			"NEW",
			"BUY",
			decimal.NewFromFloat(100.0),
			decimal.NewFromFloat(50.25),
			time.Now().UTC().Truncate(24*time.Hour),
			1,
		)
		require.NoError(t, err)
		assert.Greater(t, transactionID, int64(0))

		// Retrieve transaction
		selectQuery := `
			SELECT id, portfolio_id, security_id, source_id, status, transaction_type, quantity, price
			FROM transactions WHERE id = $1`

		type TransactionRow struct {
			ID              int64           `db:"id"`
			PortfolioID     string          `db:"portfolio_id"`
			SecurityID      *string         `db:"security_id"`
			SourceID        string          `db:"source_id"`
			Status          string          `db:"status"`
			TransactionType string          `db:"transaction_type"`
			Quantity        decimal.Decimal `db:"quantity"`
			Price           decimal.Decimal `db:"price"`
		}

		var retrieved TransactionRow
		err = suite.db.Get(&retrieved, selectQuery, transactionID)
		require.NoError(t, err)

		assert.Equal(t, transactionID, retrieved.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", retrieved.PortfolioID)
		assert.NotNil(t, retrieved.SecurityID)
		assert.Equal(t, "SECURITY1234567890123456", *retrieved.SecurityID)
		assert.Equal(t, "SOURCE001", retrieved.SourceID)
		assert.Equal(t, "NEW", strings.TrimSpace(retrieved.Status))
		assert.Equal(t, "BUY", strings.TrimSpace(retrieved.TransactionType))
		assert.True(t, decimal.NewFromFloat(100.0).Equal(retrieved.Quantity))
		assert.True(t, decimal.NewFromFloat(50.25).Equal(retrieved.Price))

		// Update transaction status
		updateQuery := `UPDATE transactions SET status = $1, version = version + 1 WHERE id = $2 AND version = $3`
		result, err := suite.db.Exec(updateQuery, "PROC", transactionID, 1)
		require.NoError(t, err)

		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)

		// Verify update
		var status string
		var version int
		err = suite.db.Get(&status, "SELECT status FROM transactions WHERE id = $1", transactionID)
		require.NoError(t, err)
		assert.Equal(t, "PROC", strings.TrimSpace(status))

		err = suite.db.Get(&version, "SELECT version FROM transactions WHERE id = $1", transactionID)
		require.NoError(t, err)
		assert.Equal(t, 2, version)
	})

	t.Run("Balance operations", func(t *testing.T) {
		// Insert balance
		insertQuery := `
			INSERT INTO balances (portfolio_id, security_id, quantity_long, quantity_short, version)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`

		var balanceID int64
		err := suite.db.Get(&balanceID, insertQuery,
			"PORTFOLIO123456789012345",
			"SECURITY1234567890123456",
			decimal.NewFromFloat(100.0),
			decimal.NewFromFloat(0.0),
			1,
		)
		require.NoError(t, err)
		assert.Greater(t, balanceID, int64(0))

		// Retrieve balance
		selectQuery := `
			SELECT id, portfolio_id, security_id, quantity_long, quantity_short
			FROM balances WHERE id = $1`

		type BalanceRow struct {
			ID            int64           `db:"id"`
			PortfolioID   string          `db:"portfolio_id"`
			SecurityID    *string         `db:"security_id"`
			QuantityLong  decimal.Decimal `db:"quantity_long"`
			QuantityShort decimal.Decimal `db:"quantity_short"`
		}

		var retrieved BalanceRow
		err = suite.db.Get(&retrieved, selectQuery, balanceID)
		require.NoError(t, err)

		assert.Equal(t, balanceID, retrieved.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", retrieved.PortfolioID)
		assert.NotNil(t, retrieved.SecurityID)
		assert.Equal(t, "SECURITY1234567890123456", *retrieved.SecurityID)
		assert.True(t, decimal.NewFromFloat(100.0).Equal(retrieved.QuantityLong))
		assert.True(t, decimal.NewFromFloat(0.0).Equal(retrieved.QuantityShort))
	})

	t.Run("Domain model validation", func(t *testing.T) {
		// Test domain model creation and validation
		transaction, err := models.NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithSourceID("SOURCE_DOMAIN_TEST").
			WithTransactionType("BUY").
			WithQuantity(decimal.NewFromFloat(50.0)).
			WithPrice(decimal.NewFromFloat(25.50)).
			WithTransactionDate(time.Now().UTC().Truncate(24 * time.Hour)).
			Build()
		require.NoError(t, err)

		// Verify domain model properties
		assert.Equal(t, "PORTFOLIO123456789012345", transaction.PortfolioID().Value())
		assert.Equal(t, "SECURITY1234567890123456", transaction.SecurityID().String())
		assert.Equal(t, "SOURCE_DOMAIN_TEST", transaction.SourceID().Value())
		assert.Equal(t, models.TransactionTypeBuy, transaction.TransactionType())
		assert.Equal(t, models.TransactionStatusNew, transaction.Status())
		assert.True(t, decimal.NewFromFloat(50.0).Equal(transaction.Quantity().Value()))
		assert.True(t, decimal.NewFromFloat(25.50).Equal(transaction.Price().Value()))

		// Test business methods
		assert.False(t, transaction.IsCashTransaction())
		assert.False(t, transaction.IsProcessed())
		notionalAmount := transaction.CalculateNotionalAmount()
		expectedNotional := decimal.NewFromFloat(50.0).Mul(decimal.NewFromFloat(25.50))
		assert.True(t, expectedNotional.Equal(notionalAmount.Value()))
	})
}
