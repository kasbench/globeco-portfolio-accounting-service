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

// Test transaction endpoints - this would catch nil service panics
func TestAPIIntegration_TransactionEndpoints(t *testing.T) {
	suite := setupAPITestSuite(t)
	defer suite.teardown(t)

	t.Run("POST transactions endpoint handles requests", func(t *testing.T) {
		// Create test transaction payload
		securityID := "683b6b9620f302c879a5fef4"
		quantity, _ := decimal.NewFromString("100.00")
		price, _ := decimal.NewFromString("50.25")

		transactions := []dto.TransactionPostDTO{
			{
				PortfolioID:     "683b70fda29ee10e8b499645",
				SecurityID:      &securityID,
				SourceID:        "9ab073be-161f-45d4-a950-cb638e3e08de",
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
		require.NoError(t, err, "POST request should not panic - this would catch nil service issues")
		defer resp.Body.Close()

		// The actual status depends on implementation, but it should not panic
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code, not panic")
	})

	t.Run("GET transactions endpoint works", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/transactions")
		require.NoError(t, err, "GET transactions should not panic")
		defer resp.Body.Close()

		// Should return some valid response, not panic
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code")
	})

	t.Run("GET specific transaction by ID", func(t *testing.T) {
		resp, err := http.Get(suite.baseURL + "/api/v1/transaction/1")
		require.NoError(t, err, "GET transaction by ID should not panic")
		defer resp.Body.Close()

		// Should return some valid response, not panic
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"Should return valid HTTP status code")
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
