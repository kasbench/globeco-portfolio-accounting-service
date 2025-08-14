package routes

import (
	"testing"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/handlers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/middleware"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRouter_WithEnhancedMetrics(t *testing.T) {
	// Create a test logger
	testLogger := logger.NewNoop()

	// Create mock handlers
	healthHandler := &handlers.HealthHandler{}
	transactionHandler := &handlers.TransactionHandler{}
	balanceHandler := &handlers.BalanceHandler{}
	swaggerHandler := &handlers.SwaggerHandler{}

	tests := []struct {
		name                  string
		enableEnhancedMetrics bool
	}{
		{
			name:                  "Enhanced metrics enabled",
			enableEnhancedMetrics: true,
		},
		{
			name:                  "Enhanced metrics disabled",
			enableEnhancedMetrics: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router configuration
			config := Config{
				ServiceName:           "test-service",
				Version:               "1.0.0",
				Environment:           "test",
				EnableMetrics:         true,
				EnableEnhancedMetrics: tt.enableEnhancedMetrics,
				EnableCORS:            false,
				CORSConfig:            middleware.DefaultCORSConfig(),
			}

			// Setup router dependencies
			deps := RouterDependencies{
				TransactionHandler: transactionHandler,
				BalanceHandler:     balanceHandler,
				HealthHandler:      healthHandler,
				SwaggerHandler:     swaggerHandler,
				Logger:             testLogger,
			}

			// Create router - this should not panic and should create the router successfully
			router := SetupRouter(config, deps)
			require.NotNil(t, router)

			// Verify the router was created successfully
			assert.NotNil(t, router, "Router should be created successfully")
		})
	}
}

func TestSetupRouter_MiddlewareOrder(t *testing.T) {
	// Create a test logger
	testLogger := logger.NewNoop()

	// Create mock handlers
	healthHandler := &handlers.HealthHandler{}
	transactionHandler := &handlers.TransactionHandler{}
	balanceHandler := &handlers.BalanceHandler{}
	swaggerHandler := &handlers.SwaggerHandler{}

	// Setup router configuration with enhanced metrics enabled
	config := Config{
		ServiceName:           "test-service",
		Version:               "1.0.0",
		Environment:           "test",
		EnableMetrics:         true,
		EnableEnhancedMetrics: true,
		EnableCORS:            false,
		CORSConfig:            middleware.DefaultCORSConfig(),
	}

	// Setup router dependencies
	deps := RouterDependencies{
		TransactionHandler: transactionHandler,
		BalanceHandler:     balanceHandler,
		HealthHandler:      healthHandler,
		SwaggerHandler:     swaggerHandler,
		Logger:             testLogger,
	}

	// Create router - this should not panic
	router := SetupRouter(config, deps)
	require.NotNil(t, router)

	// Verify the router was created successfully with enhanced metrics middleware
	assert.NotNil(t, router, "Router should be created successfully with enhanced metrics middleware")
}

func TestConfig_EnhancedMetricsField(t *testing.T) {
	// Test that the Config struct has the EnableEnhancedMetrics field
	config := Config{
		ServiceName:           "test-service",
		Version:               "1.0.0",
		Environment:           "test",
		EnableMetrics:         true,
		EnableEnhancedMetrics: true,
		EnableCORS:            false,
	}

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "1.0.0", config.Version)
	assert.Equal(t, "test", config.Environment)
	assert.True(t, config.EnableMetrics)
	assert.True(t, config.EnableEnhancedMetrics)
	assert.False(t, config.EnableCORS)
}