package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func TestNewEnhancedMetricsMiddleware(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	tests := []struct {
		name     string
		config   EnhancedMetricsConfig
		expected bool
	}{
		{
			name: "Enabled middleware",
			config: EnhancedMetricsConfig{
				ServiceName: "test-service",
				Enabled:     true,
			},
			expected: true,
		},
		{
			name: "Disabled middleware",
			config: EnhancedMetricsConfig{
				ServiceName: "test-service",
				Enabled:     false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewEnhancedMetricsMiddleware(tt.config)
			assert.NotNil(t, middleware)
			assert.Equal(t, tt.expected, middleware.enabled)
			
			if tt.expected {
				assert.Equal(t, tt.config.ServiceName, middleware.serviceName)
				assert.NotNil(t, middleware.meter)
				assert.NotNil(t, middleware.httpRequestsTotal)
				assert.NotNil(t, middleware.httpRequestDuration)
				assert.NotNil(t, middleware.httpRequestsInFlight)
			}
		})
	}
}

func TestEnhancedMetricsMiddleware_Handler(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	tests := []struct {
		name           string
		enabled        bool
		method         string
		path           string
		expectedStatus int
		expectedPath   string
	}{
		{
			name:           "GET request to health endpoint",
			enabled:        true,
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			expectedPath:   "/health",
		},
		{
			name:           "POST request to transactions",
			enabled:        true,
			method:         "POST",
			path:           "/api/v1/transactions",
			expectedStatus: http.StatusOK,
			expectedPath:   "/api/v1/transactions",
		},
		{
			name:           "GET request with parameter",
			enabled:        true,
			method:         "GET",
			path:           "/api/v1/transaction/123",
			expectedStatus: http.StatusOK,
			expectedPath:   "/api/v1/transaction/{id}",
		},
		{
			name:           "Disabled middleware",
			enabled:        false,
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			expectedPath:   "/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := EnhancedMetricsConfig{
				ServiceName: "test-service",
				Enabled:     tt.enabled,
			}
			
			middleware := NewEnhancedMetricsMiddleware(config)
			require.NotNil(t, middleware)

			// Create a test handler that returns the expected status
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
				w.Write([]byte("OK"))
			})

			// Wrap with middleware
			handler := middleware.Handler()(testHandler)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(recorder, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, recorder.Code)
			assert.Equal(t, "OK", recorder.Body.String())
		})
	}
}

func TestEnhancedMetricsMiddleware_extractPathPattern(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Static paths
		{
			name:     "Health endpoint",
			path:     "/health",
			expected: "/health",
		},
		{
			name:     "Health live endpoint",
			path:     "/health/live",
			expected: "/health/live",
		},
		{
			name:     "Metrics endpoint",
			path:     "/metrics",
			expected: "/metrics",
		},
		{
			name:     "API v1 transactions",
			path:     "/api/v1/transactions",
			expected: "/api/v1/transactions",
		},
		{
			name:     "API v1 balances",
			path:     "/api/v1/balances",
			expected: "/api/v1/balances",
		},
		
		// Dynamic paths
		{
			name:     "Transaction by ID",
			path:     "/api/v1/transaction/123",
			expected: "/api/v1/transaction/{id}",
		},
		{
			name:     "Transaction by UUID",
			path:     "/api/v1/transaction/550e8400-e29b-41d4-a716-446655440000",
			expected: "/api/v1/transaction/{id}",
		},
		{
			name:     "Balance by ID",
			path:     "/api/v1/balance/456",
			expected: "/api/v1/balance/{id}",
		},
		{
			name:     "Portfolio summary",
			path:     "/api/v1/portfolios/PORTFOLIO123/summary",
			expected: "/api/v1/portfolios/{portfolioId}/summary",
		},
		{
			name:     "Swagger UI path",
			path:     "/swagger/index.html",
			expected: "/swagger/*",
		},
		{
			name:     "API v2 placeholder",
			path:     "/api/v2/test",
			expected: "/api/v2/*",
		},
		
		// Unknown paths
		{
			name:     "Unknown short path",
			path:     "/unknown",
			expected: "/unknown",
		},
		{
			name:     "Very long unknown path",
			path:     "/very/long/unknown/path/that/exceeds/the/maximum/length/limit/and/should/be/truncated/to/prevent/cardinality/explosion",
			expected: "/path_too_long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.extractPathPatternSafely(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnhancedMetricsMiddleware_pathPatternCaching(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	// Test that caching works
	path := "/api/v1/transaction/123"
	
	// First call should compute and cache
	result1 := middleware.extractPathPatternSafely(path)
	assert.Equal(t, "/api/v1/transaction/{id}", result1)
	
	// Second call should use cache
	result2 := middleware.extractPathPatternSafely(path)
	assert.Equal(t, "/api/v1/transaction/{id}", result2)
	
	// Verify it's in the cache
	cached, exists := middleware.pathPatterns[path]
	assert.True(t, exists)
	assert.Equal(t, "/api/v1/transaction/{id}", cached)
}

func TestEnhancedMetricsMiddleware_ErrorHandling(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName:           "test-service",
		Enabled:               true,
		MaxPathPatternCache:   2, // Small cache for testing
		MaxPathLength:         20, // Short length for testing
		EnableFailsafeLogging: true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	// Test path length limit
	longPath := "/this/is/a/very/long/path/that/exceeds/limit"
	result := middleware.extractPathPatternSafely(longPath)
	assert.Equal(t, "/path_too_long", result)

	// Test cache limit
	middleware.extractPathPatternSafely("/path1")
	middleware.extractPathPatternSafely("/path2")
	// This should not be cached due to limit
	result = middleware.extractPathPatternSafely("/path3")
	assert.Equal(t, "/path3", result)
	
	// Verify cache size is at limit
	assert.Equal(t, 2, len(middleware.pathPatterns))
}

func TestEnhancedMetricsMiddleware_GetMetricsStatus(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	status := middleware.GetMetricsStatus()
	
	assert.True(t, status["enabled"].(bool))
	assert.False(t, status["initialization_failed"].(bool))
	assert.Equal(t, int64(0), status["error_count"].(int64))
	assert.Equal(t, 0, status["cache_size"].(int))
	assert.Equal(t, 1000, status["max_cache_size"].(int))
}

func TestEnhancedMetricsMiddleware_SanitizeMethods(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	tests := []struct {
		input    string
		expected string
	}{
		{"get", "GET"},
		{"POST", "POST"},
		{" put ", "PUT"},
		{"", "UNKNOWN"},
		{"VERYLONGMETHOD", "INVALID"},
	}

	for _, tt := range tests {
		result := middleware.sanitizeMethod(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestEnhancedMetricsMiddleware_SanitizeStatus(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	tests := []struct {
		input    int
		expected string
	}{
		{200, "200"},
		{404, "404"},
		{500, "500"},
		{99, "unknown"},   // Below valid range
		{600, "unknown"},  // Above valid range
	}

	for _, tt := range tests {
		result := middleware.sanitizeStatus(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestEnhancedMetricsMiddleware_InitializationFailure(t *testing.T) {
	// Reset the global meter provider to simulate OpenTelemetry unavailability
	originalProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(nil)
	
	// Clean up after test
	t.Cleanup(func() {
		otel.SetMeterProvider(originalProvider)
	})
	
	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	// This should handle the case where OpenTelemetry is not properly initialized
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)
	
	// Middleware should be disabled due to initialization failure
	assert.False(t, middleware.enabled)
	assert.True(t, middleware.initializationFailed)
	
	// Handler should return a no-op middleware
	handler := middleware.Handler()
	require.NotNil(t, handler)
	
	// Test that the no-op handler works
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	handler(testHandler).ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
	
	// Check status
	status := middleware.GetMetricsStatus()
	assert.False(t, status["enabled"].(bool))
	assert.True(t, status["initialization_failed"].(bool))
}

func TestEnhancedMetricsResponseWriter(t *testing.T) {
	tests := []struct {
		name           string
		writeHeader    bool
		statusCode     int
		writeBody      bool
		body           string
		expectedStatus int
	}{
		{
			name:           "Explicit status code",
			writeHeader:    true,
			statusCode:     http.StatusCreated,
			writeBody:      true,
			body:           "Created",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Default status code with body",
			writeHeader:    false,
			statusCode:     0,
			writeBody:      true,
			body:           "OK",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Error status code",
			writeHeader:    true,
			statusCode:     http.StatusInternalServerError,
			writeBody:      true,
			body:           "Error",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "No body written",
			writeHeader:    true,
			statusCode:     http.StatusNoContent,
			writeBody:      false,
			body:           "",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writer := &enhancedMetricsResponseWriter{
				ResponseWriter: recorder,
				statusCode:     http.StatusOK,
			}

			if tt.writeHeader {
				writer.WriteHeader(tt.statusCode)
			}

			if tt.writeBody {
				_, err := writer.Write([]byte(tt.body))
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedStatus, writer.statusCode)
			assert.Equal(t, tt.expectedStatus, recorder.Code)
			assert.Equal(t, tt.body, recorder.Body.String())
		})
	}
}

func TestEnhancedMetricsMiddleware_Integration(t *testing.T) {
	// Setup test meter provider
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}
	
	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)
	require.True(t, middleware.enabled)

	// Create a test handler that simulates some processing time
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Wrap with middleware
	handler := middleware.Handler()(testHandler)

	// Test multiple requests to verify metrics are recorded
	testCases := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/health", http.StatusOK},
		{"POST", "/api/v1/transactions", http.StatusOK},
		{"GET", "/api/v1/transaction/123", http.StatusOK},
		{"GET", "/api/v1/balance/456", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tc.status, recorder.Code)
			assert.Equal(t, "Success", recorder.Body.String())
		})
	}
}

// setupTestMeterProvider creates a test meter provider for testing
func setupTestMeterProvider(t *testing.T) {
	// Create a test resource
	res, err := resource.New(
		nil,
		resource.WithAttributes(
			semconv.ServiceName("test-service"),
		),
	)
	require.NoError(t, err)

	// Create a test meter provider
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
	)

	// Set the global meter provider
	otel.SetMeterProvider(meterProvider)

	// Clean up after test
	t.Cleanup(func() {
		// Reset to default meter provider
		otel.SetMeterProvider(metric.NewMeterProvider())
	})
}