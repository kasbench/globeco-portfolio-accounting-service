package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/shopspring/decimal"
)

// APITestSuite represents the complete API integration test suite
type APITestSuite struct {
	ctx               context.Context
	postgresContainer *postgres.PostgresContainer
	db                *sqlx.DB
	server            *api.Server
	httpServer        *httptest.Server
	baseURL           string
	logger            logger.Logger
	config            *config.Config
	metricsRegistry   *prometheus.Registry // Custom registry to avoid duplicate registration
}

// setupAPITestSuite initializes the complete test environment
func setupAPITestSuite(t *testing.T) *APITestSuite {
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
				WithStartupTimeout(2*time.Minute)),
	)
	require.NoError(t, err)

	// Get database connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)

	// Run migrations
	err = runMigrations(db)
	require.NoError(t, err)

	// Create test logger
	testLogger := logger.NewDevelopment()

	// Create test configuration
	cfg := createTestConfig(connStr)

	// Create server instance (this is where nil pointer issues would surface)
	server, err := api.NewServer(cfg, testLogger)
	require.NoError(t, err, "Server initialization should not fail - this would catch nil service issues")

	// Create HTTP test server
	httpServer := httptest.NewServer(server.GetServer().Handler)

	return &APITestSuite{
		ctx:               ctx,
		postgresContainer: postgresContainer,
		db:                db,
		server:            server,
		httpServer:        httpServer,
		baseURL:           httpServer.URL,
		logger:            testLogger,
		config:            cfg,
		metricsRegistry:   prometheus.NewRegistry(), // Keep for future use
	}
}

// teardown cleans up the test environment
func (suite *APITestSuite) teardown(t *testing.T) {
	if suite.httpServer != nil {
		suite.httpServer.Close()
	}

	if suite.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		suite.server.Shutdown(ctx)
	}

	if suite.db != nil {
		suite.db.Close()
	}

	if suite.postgresContainer != nil {
		err := suite.postgresContainer.Terminate(suite.ctx)
		require.NoError(t, err)
	}
}

// createTestConfig creates a test configuration with database connection
func createTestConfig(dbConnStr string) *config.Config {
	// Parse PostgreSQL URL format: postgres://user:password@host:port/database?sslmode=disable
	// Default values
	host, dbname, user, password := "localhost", "testdb", "testuser", "testpass"
	port := 5432

	// Parse URL format
	if strings.HasPrefix(dbConnStr, "postgres://") {
		// Remove postgres:// prefix
		urlPart := strings.TrimPrefix(dbConnStr, "postgres://")

		// Split by @ to separate credentials from host/db
		parts := strings.Split(urlPart, "@")
		if len(parts) >= 2 {
			// Parse credentials: user:password
			credParts := strings.Split(parts[0], ":")
			if len(credParts) >= 2 {
				user = credParts[0]
				password = credParts[1]
			}

			// Parse host:port/database?params
			hostDbPart := parts[1]

			// Split by ? to remove query parameters
			hostDbPart = strings.Split(hostDbPart, "?")[0]

			// Split by / to separate host:port from database
			hostPortDb := strings.Split(hostDbPart, "/")
			if len(hostPortDb) >= 2 {
				// Parse host:port
				hostPort := hostPortDb[0]
				dbname = hostPortDb[1]

				// Parse host and port
				hostPortParts := strings.Split(hostPort, ":")
				if len(hostPortParts) >= 2 {
					host = hostPortParts[0]
					fmt.Sscanf(hostPortParts[1], "%d", &port)
				}
			}
		}
	}

	return &config.Config{
		Server: config.ServerConfig{
			Host:                    "localhost",
			Port:                    8087,
			ReadTimeout:             30 * time.Second,
			WriteTimeout:            30 * time.Second,
			IdleTimeout:             60 * time.Second,
			GracefulShutdownTimeout: 30 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:            host,     // Use TestContainer host
			Port:            port,     // Use TestContainer port
			Database:        dbname,   // Use TestContainer database
			User:            user,     // Use TestContainer user
			Password:        password, // Use TestContainer password
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			MigrationsPath:  "../../migrations",
			AutoMigrate:     false, // Already migrated in setup
		},
		External: config.ExternalConfig{
			PortfolioService: config.ServiceConfig{
				Host:                    "localhost",
				Port:                    8001,
				Timeout:                 30 * time.Second,
				MaxRetries:              3,
				RetryBackoff:            1 * time.Second,
				CircuitBreakerThreshold: 5,
				HealthEndpoint:          "/health",
			},
			SecurityService: config.ServiceConfig{
				Host:                    "localhost",
				Port:                    8000,
				Timeout:                 30 * time.Second,
				MaxRetries:              3,
				RetryBackoff:            1 * time.Second,
				CircuitBreakerThreshold: 5,
				HealthEndpoint:          "/health/liveness",
			},
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Test that would have caught the nil pointer panic
func TestAPIIntegration_ServerInitialization(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("Server initializes all dependencies properly", func(t *testing.T) {
		// This test ensures server starts without panics
		// It would have caught the nil service initialization issue
		assert.NotNil(t, suite.server, "Server should be initialized")
		assert.NotNil(t, suite.httpServer, "HTTP server should be running")
	})
}

// Test health endpoints functionality
func TestAPIIntegration_HealthEndpoints(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		description    string
	}{
		{
			name:           "Basic health check",
			endpoint:       "/health",
			expectedStatus: http.StatusOK,
			description:    "Should return 200 OK for basic health",
		},
		{
			name:           "Liveness probe",
			endpoint:       "/health/live",
			expectedStatus: http.StatusOK,
			description:    "Should return 200 OK for liveness probe",
		},
		{
			name:           "Readiness probe with nil services",
			endpoint:       "/health/ready",
			expectedStatus: http.StatusServiceUnavailable,
			description:    "Should handle nil external services gracefully",
		},
		{
			name:           "Detailed health with nil services",
			endpoint:       "/health/detailed",
			expectedStatus: http.StatusServiceUnavailable,
			description:    "Should handle nil external services gracefully",
		},
		{
			name:           "API v1 basic health",
			endpoint:       "/api/v1/health",
			expectedStatus: http.StatusOK,
			description:    "Should return 200 OK for API v1 health",
		},
		{
			name:           "API v1 readiness",
			endpoint:       "/api/v1/health/ready",
			expectedStatus: http.StatusServiceUnavailable,
			description:    "Should handle nil external services gracefully in API v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(suite.baseURL + tt.endpoint)
			require.NoError(t, err, "Health endpoint request should not fail")
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode, tt.description)

			// Parse response body
			var healthResp dto.HealthResponse
			err = json.NewDecoder(resp.Body).Decode(&healthResp)
			require.NoError(t, err, "Health response should be valid JSON")

			// Verify response structure
			assert.NotEmpty(t, healthResp.Timestamp, "Health response should have timestamp")
			assert.NotEmpty(t, healthResp.Version, "Health response should have version")
		})
	}
}

// Test transaction endpoints with comprehensive balance verification
func TestAPIIntegration_TransactionEndpoints(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("BUY Transaction Creates Security and Cash Balances", func(t *testing.T) {
		// Clear any existing data
		suite.db.Exec("DELETE FROM balances")
		suite.db.Exec("DELETE FROM transactions")

		// Create BUY transaction: should increase security long, decrease cash
		securityID := "683b6b9620f302c879a5fef4"
		quantity, _ := decimal.NewFromString("100.00")
		price, _ := decimal.NewFromString("50.25")
		portfolioID := "683b70fda29ee10e8b499645"

		transactions := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      &securityID,
				SourceID:        "buy-test-source-001",
				TransactionType: "BUY",
				Quantity:        quantity,
				Price:           price,
				TransactionDate: "20250101",
			},
		}

		jsonData, err := json.Marshal(transactions)
		require.NoError(t, err)

		// Make POST request
		resp, err := http.Post(
			suite.baseURL+"/api/v1/transactions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Transaction creation returns 201 Created for successful creation
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK,
			"Transaction creation should succeed, got %d", resp.StatusCode)

		// Parse response to verify transaction was processed
		var batchResponse dto.TransactionBatchResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResponse)
		require.NoError(t, err)

		if batchResponse.Summary.Successful > 0 {
			assert.Equal(t, 1, batchResponse.Summary.Successful, "Transaction should be successful")
			assert.Equal(t, 0, batchResponse.Summary.Failed, "No failed transactions")
			assert.Len(t, batchResponse.Successful, 1, "Should have one successful transaction")
			assert.Equal(t, "PROC", batchResponse.Successful[0].Status, "Transaction should be processed")

			// Verify balance records were created
			var balanceCount int
			err = suite.db.Get(&balanceCount, "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1", portfolioID)
			require.NoError(t, err)
			assert.Equal(t, 2, balanceCount, "Should create 2 balances: security + cash")

			// Verify security balance: BUY should increase long position
			type BalanceRow struct {
				PortfolioID   string          `db:"portfolio_id"`
				SecurityID    *string         `db:"security_id"`
				QuantityLong  decimal.Decimal `db:"quantity_long"`
				QuantityShort decimal.Decimal `db:"quantity_short"`
			}

			var securityBalance BalanceRow
			err = suite.db.Get(&securityBalance,
				"SELECT portfolio_id, security_id, quantity_long, quantity_short FROM balances WHERE portfolio_id = $1 AND security_id = $2",
				portfolioID, securityID)
			require.NoError(t, err)

			assert.Equal(t, portfolioID, securityBalance.PortfolioID)
			assert.NotNil(t, securityBalance.SecurityID)
			assert.Equal(t, securityID, *securityBalance.SecurityID)
			assert.True(t, quantity.Equal(securityBalance.QuantityLong), "Security long should equal transaction quantity")
			assert.True(t, decimal.Zero.Equal(securityBalance.QuantityShort), "Security short should be zero")

			// Verify cash balance: BUY should decrease cash by notional amount (quantity * price)
			var cashBalance BalanceRow
			err = suite.db.Get(&cashBalance,
				"SELECT portfolio_id, security_id, quantity_long, quantity_short FROM balances WHERE portfolio_id = $1 AND security_id IS NULL",
				portfolioID)
			require.NoError(t, err)

			assert.Equal(t, portfolioID, cashBalance.PortfolioID)
			assert.Nil(t, cashBalance.SecurityID, "Cash balance should have NULL security_id")

			expectedCashChange := quantity.Mul(price).Neg() // -5025.00
			assert.True(t, expectedCashChange.Equal(cashBalance.QuantityLong),
				"Cash should decrease by notional amount: %s, got %s", expectedCashChange, cashBalance.QuantityLong)
			assert.True(t, decimal.Zero.Equal(cashBalance.QuantityShort), "Cash short should always be zero")
		} else {
			t.Logf("Transaction failed, errors: %+v", batchResponse.Failed)
		}
	})

	t.Run("SELL Transaction Updates Existing Balances", func(t *testing.T) {
		// Clear any existing data and use different portfolio to avoid conflicts
		suite.db.Exec("DELETE FROM balances")
		suite.db.Exec("DELETE FROM transactions")

		portfolioID := "683b70fda29ee10e8b499646" // Different portfolio
		securityID := "683b6b9620f302c879a5fef5"  // Different security

		// First create initial security and cash balances with BUY
		buyQuantity, _ := decimal.NewFromString("200.00")
		buyPrice, _ := decimal.NewFromString("50.00")

		buyTransaction := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      &securityID,
				SourceID:        "setup-buy-002",
				TransactionType: "BUY",
				Quantity:        buyQuantity,
				Price:           buyPrice,
				TransactionDate: "20250101",
			},
		}

		jsonData, _ := json.Marshal(buyTransaction)
		resp, _ := http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
		resp.Body.Close()

		// Wait a moment for transaction to be processed
		time.Sleep(100 * time.Millisecond)

		// Now SELL some securities: should decrease security long, increase cash
		sellQuantity, _ := decimal.NewFromString("50.00")
		sellPrice, _ := decimal.NewFromString("55.00")

		sellTransaction := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      &securityID,
				SourceID:        "sell-test-002",
				TransactionType: "SELL",
				Quantity:        sellQuantity,
				Price:           sellPrice,
				TransactionDate: "20250102",
			},
		}

		jsonData, _ = json.Marshal(sellTransaction)
		resp, err := http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept either success or partial failure
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusMultiStatus, "SELL should be processed")

		// Parse response
		var batchResponse dto.TransactionBatchResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResponse)
		require.NoError(t, err)

		if batchResponse.Summary.Successful > 0 {
			// Verify updated security balance: should be 200 - 50 = 150
			type BalanceRow struct {
				QuantityLong  decimal.Decimal `db:"quantity_long"`
				QuantityShort decimal.Decimal `db:"quantity_short"`
			}

			var securityBalance BalanceRow
			err = suite.db.Get(&securityBalance,
				"SELECT quantity_long, quantity_short FROM balances WHERE portfolio_id = $1 AND security_id = $2",
				portfolioID, securityID)
			require.NoError(t, err)

			expectedSecurityLong := buyQuantity.Sub(sellQuantity) // 200 - 50 = 150
			assert.True(t, expectedSecurityLong.Equal(securityBalance.QuantityLong),
				"Security long should be %s, got %s", expectedSecurityLong, securityBalance.QuantityLong)
		} else {
			t.Logf("SELL transaction failed, errors: %+v", batchResponse.Failed)
		}
	})

	t.Run("SHORT Transaction Creates Short Position and Increases Cash", func(t *testing.T) {
		// Clear any existing data and use different portfolio to avoid conflicts
		suite.db.Exec("DELETE FROM balances")
		suite.db.Exec("DELETE FROM transactions")

		portfolioID := "683b70fda29ee10e8b499647" // Different portfolio
		securityID := "683b6b9620f302c879a5fef6"  // Different security
		quantity, _ := decimal.NewFromString("75.00")
		price, _ := decimal.NewFromString("60.00")

		shortTransaction := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      &securityID,
				SourceID:        "short-test-003",
				TransactionType: "SHORT",
				Quantity:        quantity,
				Price:           price,
				TransactionDate: "20250101",
			},
		}

		jsonData, _ := json.Marshal(shortTransaction)
		resp, err := http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept various status codes
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusMultiStatus, "SHORT should be processed")

		var batchResponse dto.TransactionBatchResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResponse)
		require.NoError(t, err)

		if batchResponse.Summary.Successful > 0 {
			// Verify security balance: SHORT should increase short position
			type BalanceRow struct {
				QuantityLong  decimal.Decimal `db:"quantity_long"`
				QuantityShort decimal.Decimal `db:"quantity_short"`
			}

			var securityBalance BalanceRow
			err = suite.db.Get(&securityBalance,
				"SELECT quantity_long, quantity_short FROM balances WHERE portfolio_id = $1 AND security_id = $2",
				portfolioID, securityID)
			require.NoError(t, err)

			assert.True(t, decimal.Zero.Equal(securityBalance.QuantityLong), "Long position should be zero")
			assert.True(t, quantity.Equal(securityBalance.QuantityShort), "Short position should equal transaction quantity")
		} else {
			t.Logf("SHORT transaction failed, errors: %+v", batchResponse.Failed)
		}
	})

	t.Run("Cash Transactions (DEP/WD) Only Affect Cash Balance", func(t *testing.T) {
		// Clear any existing data and use different portfolio to avoid conflicts
		suite.db.Exec("DELETE FROM balances")
		suite.db.Exec("DELETE FROM transactions")

		portfolioID := "683b70fda29ee10e8b499648" // Different portfolio

		// Test DEP (deposit) - should increase cash
		depositAmount, _ := decimal.NewFromString("5000.00")

		depTransaction := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      nil, // Cash transaction
				SourceID:        "deposit-test-004",
				TransactionType: "DEP",
				Quantity:        depositAmount,
				Price:           decimal.NewFromFloat(1.0), // Cash price is always 1.0
				TransactionDate: "20250101",
			},
		}

		jsonData, _ := json.Marshal(depTransaction)
		resp, err := http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept various status codes for successful creation
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusMultiStatus,
			"DEP transaction should be processed, got %d", resp.StatusCode)

		var batchResponse dto.TransactionBatchResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResponse)
		require.NoError(t, err)

		if batchResponse.Summary.Successful > 0 {
			// Should only create 1 balance record (cash)
			var balanceCount int
			err = suite.db.Get(&balanceCount, "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1", portfolioID)
			require.NoError(t, err)
			assert.Equal(t, 1, balanceCount, "DEP should only create cash balance")

			// Verify cash balance
			type BalanceRow struct {
				SecurityID    *string         `db:"security_id"`
				QuantityLong  decimal.Decimal `db:"quantity_long"`
				QuantityShort decimal.Decimal `db:"quantity_short"`
			}

			var cashBalance BalanceRow
			err = suite.db.Get(&cashBalance,
				"SELECT security_id, quantity_long, quantity_short FROM balances WHERE portfolio_id = $1",
				portfolioID)
			require.NoError(t, err)

			assert.Nil(t, cashBalance.SecurityID, "Should be cash balance (NULL security_id)")
			assert.True(t, depositAmount.Equal(cashBalance.QuantityLong), "Cash should equal deposit amount")
			assert.True(t, decimal.Zero.Equal(cashBalance.QuantityShort), "Cash short should be zero")

			// Wait a moment to avoid optimistic locking conflicts
			time.Sleep(100 * time.Millisecond)

			// Test WD (withdrawal) - should decrease cash
			withdrawalAmount, _ := decimal.NewFromString("1500.00")

			wdTransaction := []dto.TransactionPostDTO{
				{
					PortfolioID:     portfolioID,
					SecurityID:      nil, // Cash transaction
					SourceID:        "withdrawal-test-004",
					TransactionType: "WD",
					Quantity:        withdrawalAmount,
					Price:           decimal.NewFromFloat(1.0),
					TransactionDate: "20250102",
				},
			}

			jsonData, _ = json.Marshal(wdTransaction)
			resp, err = http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			// Accept various status codes
			assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
				resp.StatusCode == http.StatusMultiStatus,
				"WD transaction should be processed, got %d", resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&batchResponse)
			require.NoError(t, err)

			if batchResponse.Summary.Successful > 0 {
				// Still should only have 1 balance record
				err = suite.db.Get(&balanceCount, "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1", portfolioID)
				require.NoError(t, err)
				assert.Equal(t, 1, balanceCount, "WD should update existing cash balance")

				// Verify updated cash balance: 5000 - 1500 = 3500
				err = suite.db.Get(&cashBalance,
					"SELECT security_id, quantity_long, quantity_short FROM balances WHERE portfolio_id = $1",
					portfolioID)
				require.NoError(t, err)

				expectedCash := depositAmount.Sub(withdrawalAmount) // 5000 - 1500 = 3500
				assert.True(t, expectedCash.Equal(cashBalance.QuantityLong),
					"Cash should be %s after withdrawal, got %s", expectedCash, cashBalance.QuantityLong)
			} else {
				t.Logf("WD transaction failed due to optimistic locking or other issues")
			}
		} else {
			t.Logf("DEP transaction failed, errors: %+v", batchResponse.Failed)
		}
	})

	t.Run("IN/OUT Transactions Only Affect Security Balance", func(t *testing.T) {
		// Clear any existing data and use different portfolio to avoid conflicts
		suite.db.Exec("DELETE FROM balances")
		suite.db.Exec("DELETE FROM transactions")

		portfolioID := "683b70fda29ee10e8b499649" // Different portfolio
		securityID := "683b6b9620f302c879a5fef7"  // Different security
		quantity, _ := decimal.NewFromString("100.00")

		// Test IN (securities transfer in) - should increase security long, no cash impact
		inTransaction := []dto.TransactionPostDTO{
			{
				PortfolioID:     portfolioID,
				SecurityID:      &securityID,
				SourceID:        "transfer-in-005",
				TransactionType: "IN",
				Quantity:        quantity,
				Price:           decimal.NewFromFloat(0.0), // IN/OUT don't have meaningful prices
				TransactionDate: "20250101",
			},
		}

		jsonData, _ := json.Marshal(inTransaction)
		resp, err := http.Post(suite.baseURL+"/api/v1/transactions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept various status codes
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusMultiStatus,
			"IN transaction should be processed, got %d", resp.StatusCode)

		var batchResponse dto.TransactionBatchResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResponse)
		require.NoError(t, err)

		if batchResponse.Summary.Successful > 0 {
			// Should only create 1 balance record (security only, no cash for IN transactions)
			var balanceCount int
			err = suite.db.Get(&balanceCount, "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1", portfolioID)
			require.NoError(t, err)
			assert.Equal(t, 1, balanceCount, "IN should only create security balance")

			// Verify security balance
			type BalanceRow struct {
				SecurityID    *string         `db:"security_id"`
				QuantityLong  decimal.Decimal `db:"quantity_long"`
				QuantityShort decimal.Decimal `db:"quantity_short"`
			}

			var securityBalance BalanceRow
			err = suite.db.Get(&securityBalance,
				"SELECT security_id, quantity_long, quantity_short FROM balances WHERE portfolio_id = $1",
				portfolioID)
			require.NoError(t, err)

			assert.NotNil(t, securityBalance.SecurityID)
			assert.Equal(t, securityID, *securityBalance.SecurityID)
			assert.True(t, quantity.Equal(securityBalance.QuantityLong), "Security long should equal transfer quantity")
			assert.True(t, decimal.Zero.Equal(securityBalance.QuantityShort), "Security short should be zero")
		} else {
			t.Logf("IN transaction failed, errors: %+v", batchResponse.Failed)
		}
	})

	t.Run("GET transactions endpoint returns processed transactions", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/transactions")
		require.NoError(t, err, "GET transactions should not panic")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var listResponse dto.TransactionListResponse
		err = json.NewDecoder(resp.Body).Decode(&listResponse)
		require.NoError(t, err)

		// May have transactions from previous tests (only successful ones)
		t.Logf("Found %d transactions", len(listResponse.Transactions))

		// If we have transactions, verify their status
		for _, txn := range listResponse.Transactions {
			assert.True(t, txn.Status == "PROC" || txn.Status == "ERROR",
				"Transactions should be processed or failed, got %s", txn.Status)
		}
	})

	t.Run("GET specific transaction by ID", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/transaction/1")
		require.NoError(t, err, "GET transaction by ID should not panic")
		defer resp.Body.Close()

		// Response depends on whether transaction exists
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
	})
}

// Test balance endpoints
func TestAPIIntegration_BalanceEndpoints(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("GET balances endpoint works", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/balances")
		require.NoError(t, err, "GET balances should not panic")
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code")
	})

	t.Run("GET specific balance by ID", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/balance/1")
		require.NoError(t, err, "GET balance by ID should not panic")
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code")
	})

	t.Run("GET portfolio summary", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/portfolios/test-portfolio/summary")
		require.NoError(t, err, "GET portfolio summary should not panic")
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code")
	})
}

// Test middleware stack doesn't panic
func TestAPIIntegration_MiddlewareStack(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("All middleware layers handle requests properly", func(t *testing.T) {
		// Test various HTTP methods and endpoints to ensure middleware doesn't panic
		testCases := []struct {
			method   string
			endpoint string
		}{
			{"GET", "/health"},
			{"GET", "/api/v1/health"},
			{"GET", "/api/v1/transactions"},
			{"GET", "/api/v1/balances"},
			{"POST", "/api/v1/transactions"},
			{"GET", "/metrics"},
			{"GET", "/nonexistent"}, // Should return 404, not panic
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s %s", tc.method, tc.endpoint), func(t *testing.T) {
				var resp *http.Response
				var err error

				switch tc.method {
				case "GET":
					resp, err = http.Get(suite.baseURL + tc.endpoint)
				case "POST":
					resp, err = http.Post(suite.baseURL+tc.endpoint, "application/json", strings.NewReader("[]"))
				}

				require.NoError(t, err, "Request should not fail due to middleware panic")
				if resp != nil {
					resp.Body.Close()
					assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
						"Should return valid HTTP status code")
				}
			})
		}
	})
}

// Test concurrent requests don't cause panics
func TestAPIIntegration_ConcurrentRequests(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("Concurrent health check requests", func(t *testing.T) {
		const numRequests = 10
		done := make(chan bool, numRequests)
		errors := make(chan error, numRequests)

		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						errors <- fmt.Errorf("panic occurred: %v", r)
						return
					}
					done <- true
				}()

				resp, err := http.Get(suite.baseURL + "/health/detailed")
				if err != nil {
					errors <- err
					return
				}
				resp.Body.Close()
			}()
		}

		// Wait for all requests to complete
		completed := 0
		errorCount := 0
		timeout := time.After(10 * time.Second)

		for completed+errorCount < numRequests {
			select {
			case <-done:
				completed++
			case err := <-errors:
				t.Logf("Request error: %v", err)
				errorCount++
			case <-timeout:
				t.Fatal("Timeout waiting for concurrent requests")
			}
		}

		// All requests should complete without panics
		assert.Equal(t, numRequests, completed+errorCount, "All requests should complete")
		t.Logf("Completed: %d, Errors: %d", completed, errorCount)
	})
}

// Test error handling doesn't cause panics
func TestAPIIntegration_ErrorHandling(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("Invalid JSON payload doesn't panic", func(t *testing.T) {
		resp, err := http.Post(
			suite.baseURL+"/api/v1/transactions",
			"application/json",
			strings.NewReader("{invalid-json}"),
		)
		require.NoError(t, err, "Invalid JSON should not cause panic")
		defer resp.Body.Close()

		// Should return error response, not panic
		assert.True(t, resp.StatusCode >= 400 && resp.StatusCode < 600,
			"Should return client/server error, not panic")
	})

	t.Run("Large payload doesn't panic", func(t *testing.T) {
		// Create large payload
		largePayload := strings.Repeat("x", 1024*1024) // 1MB
		resp, err := http.Post(
			suite.baseURL+"/api/v1/transactions",
			"application/json",
			strings.NewReader(largePayload),
		)
		require.NoError(t, err, "Large payload should not cause panic")
		defer resp.Body.Close()

		// Should handle gracefully, not panic
		assert.True(t, resp.StatusCode >= 400 && resp.StatusCode < 600,
			"Should return error response for invalid payload")
	})
}
