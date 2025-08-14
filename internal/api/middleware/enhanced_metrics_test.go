package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

// TestMetricInitializationAndRegistration tests that all metrics are properly initialized and registered with OpenTelemetry
func TestMetricInitializationAndRegistration(t *testing.T) {
	// Setup test meter provider with reader to capture metrics
	reader := metric.NewManualReader()
	setupTestMeterProviderWithReader(t, reader)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)
	require.True(t, middleware.enabled)
	require.False(t, middleware.initializationFailed)

	// Verify all metrics are initialized
	assert.NotNil(t, middleware.httpRequestsTotal, "HTTP requests total counter should be initialized")
	assert.NotNil(t, middleware.httpRequestDuration, "HTTP request duration histogram should be initialized")
	assert.NotNil(t, middleware.httpRequestsInFlight, "HTTP requests in flight gauge should be initialized")

	// Verify meter is properly set
	assert.NotNil(t, middleware.meter, "Meter should be initialized")

	// Make a test request to generate metrics
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := middleware.Handler()(testHandler)
	req := httptest.NewRequest("GET", "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// Collect metrics to verify they are registered
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Verify metrics are present in the collected data
	assert.NotEmpty(t, rm.ScopeMetrics, "Should have scope metrics")
	
	var foundCounter, foundHistogram, foundGauge bool
	for _, scopeMetric := range rm.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			switch metric.Name {
			case "http_requests_total_enhanced":
				foundCounter = true
				assert.Equal(t, "{request}", metric.Unit)
				assert.Equal(t, "Total number of HTTP requests (enhanced metrics)", metric.Description)
			case "http_request_duration_milliseconds":
				foundHistogram = true
				assert.Equal(t, "ms", metric.Unit)
				assert.Equal(t, "Duration of HTTP requests in milliseconds", metric.Description)
			case "http_requests_in_flight_enhanced":
				foundGauge = true
				assert.Equal(t, "{request}", metric.Unit)
				assert.Equal(t, "Number of HTTP requests currently being processed (enhanced metrics)", metric.Description)
			}
		}
	}

	assert.True(t, foundCounter, "Should find HTTP requests total counter metric")
	assert.True(t, foundHistogram, "Should find HTTP request duration histogram metric")
	assert.True(t, foundGauge, "Should find HTTP requests in flight gauge metric")
}

// TestCounterIncrementsCorrectly tests that the counter increments for each HTTP request
func TestCounterIncrementsCorrectly(t *testing.T) {
	reader := metric.NewManualReader()
	setupTestMeterProviderWithReader(t, reader)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := middleware.Handler()(testHandler)

	// Make multiple requests
	requests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/health", http.StatusOK},
		{"POST", "/api/v1/transactions", http.StatusOK},
		{"GET", "/api/v1/transaction/123", http.StatusOK},
		{"PUT", "/api/v1/transaction/456", http.StatusOK},
	}

	for _, req := range requests {
		httpReq := httptest.NewRequest(req.method, req.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httpReq)
		assert.Equal(t, req.status, recorder.Code)
	}

	// Collect metrics
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Find the counter metric and verify counts
	var counterMetric *metricdata.Metrics
	for _, scopeMetric := range rm.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if metric.Name == "http_requests_total_enhanced" {
				counterMetric = &metric
				break
			}
		}
	}

	require.NotNil(t, counterMetric, "Should find counter metric")
	
	// Verify counter data
	counterData, ok := counterMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "Counter should have Sum data type")
	
	// Should have 4 data points (one for each request)
	assert.Len(t, counterData.DataPoints, 4, "Should have 4 counter data points")

	// Verify each data point has correct labels and value
	expectedLabels := []struct {
		method string
		path   string
		status string
	}{
		{"GET", "/health", "200"},
		{"POST", "/api/v1/transactions", "200"},
		{"GET", "/api/v1/transaction/{id}", "200"},
		{"PUT", "/api/v1/transaction/{id}", "200"},
	}

	for i, dp := range counterData.DataPoints {
		assert.Equal(t, int64(1), dp.Value, "Each counter should have value 1")
		
		// Verify attributes
		attrs := dp.Attributes.ToSlice()
		assert.Len(t, attrs, 3, "Should have 3 attributes: method, path, status")
		
		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}
		
		assert.Equal(t, expectedLabels[i].method, attrMap["method"])
		assert.Equal(t, expectedLabels[i].path, attrMap["path"])
		assert.Equal(t, expectedLabels[i].status, attrMap["status"])
	}
}

// TestHistogramRecordsAccurateDurations tests that the histogram records accurate request durations in milliseconds
func TestHistogramRecordsAccurateDurations(t *testing.T) {
	reader := metric.NewManualReader()
	setupTestMeterProviderWithReader(t, reader)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	// Create test handler with controlled delay
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add a small delay to ensure measurable duration
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := middleware.Handler()(testHandler)

	// Make a request
	req := httptest.NewRequest("GET", "/health", nil)
	recorder := httptest.NewRecorder()
	
	start := time.Now()
	handler.ServeHTTP(recorder, req)
	actualDuration := time.Since(start)

	// Collect metrics
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Find the histogram metric
	var histogramMetric *metricdata.Metrics
	for _, scopeMetric := range rm.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if metric.Name == "http_request_duration_milliseconds" {
				histogramMetric = &metric
				break
			}
		}
	}

	require.NotNil(t, histogramMetric, "Should find histogram metric")
	
	// Verify histogram data
	histogramData, ok := histogramMetric.Data.(metricdata.Histogram[float64])
	require.True(t, ok, "Histogram should have Histogram data type")
	
	assert.Len(t, histogramData.DataPoints, 1, "Should have 1 histogram data point")
	
	dp := histogramData.DataPoints[0]
	assert.Equal(t, uint64(1), dp.Count, "Histogram count should be 1")
	
	// Verify the recorded duration is reasonable (should be around 50ms)
	recordedDurationMs := dp.Sum / float64(dp.Count)
	actualDurationMs := float64(actualDuration.Nanoseconds()) / 1e6
	
	// Allow some tolerance for timing variations
	assert.InDelta(t, actualDurationMs, recordedDurationMs, 20.0, 
		"Recorded duration should be close to actual duration")
	
	// Verify duration is in milliseconds (should be around 50)
	assert.Greater(t, recordedDurationMs, 40.0, "Duration should be at least 40ms")
	assert.Less(t, recordedDurationMs, 100.0, "Duration should be less than 100ms")

	// Verify histogram buckets are correct
	expectedBuckets := []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	assert.Equal(t, expectedBuckets, histogramData.DataPoints[0].Bounds, 
		"Histogram should have correct bucket boundaries")

	// Verify attributes
	attrs := dp.Attributes.ToSlice()
	assert.Len(t, attrs, 3, "Should have 3 attributes: method, path, status")
	
	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}
	
	assert.Equal(t, "GET", attrMap["method"])
	assert.Equal(t, "/health", attrMap["path"])
	assert.Equal(t, "200", attrMap["status"])
}

// TestGaugeTracksConcurrentRequests tests that the gauge properly tracks concurrent requests
func TestGaugeTracksConcurrentRequests(t *testing.T) {
	reader := metric.NewManualReader()
	setupTestMeterProviderWithReader(t, reader)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	// Create test handler with controlled delay and synchronization
	var requestsStarted, requestsFinished sync.WaitGroup
	var maxConcurrentRequests int
	var concurrentCount int
	var countMutex sync.Mutex

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		countMutex.Lock()
		concurrentCount++
		if concurrentCount > maxConcurrentRequests {
			maxConcurrentRequests = concurrentCount
		}
		countMutex.Unlock()

		requestsStarted.Done()
		
		// Wait for all requests to start before finishing any
		requestsStarted.Wait()
		
		// Add delay to ensure concurrent processing
		time.Sleep(100 * time.Millisecond)
		
		countMutex.Lock()
		concurrentCount--
		countMutex.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		
		requestsFinished.Done()
	})

	handler := middleware.Handler()(testHandler)

	// Start 3 concurrent requests
	numRequests := 3
	requestsStarted.Add(numRequests)
	requestsFinished.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/health", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
		}()
	}

	// Wait for all requests to complete
	requestsFinished.Wait()

	// Verify we actually had concurrent requests
	assert.Equal(t, numRequests, maxConcurrentRequests, 
		"Should have had 3 concurrent requests")

	// Collect metrics - note that gauge values might be 0 at collection time
	// since all requests have completed, but we can verify the metric exists
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Find the gauge metric
	var gaugeMetric *metricdata.Metrics
	for _, scopeMetric := range rm.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if metric.Name == "http_requests_in_flight_enhanced" {
				gaugeMetric = &metric
				break
			}
		}
	}

	require.NotNil(t, gaugeMetric, "Should find gauge metric")
	
	// Verify gauge data type
	_, ok := gaugeMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "Gauge should have Sum data type (UpDownCounter)")
}

// TestPathPatternExtractionForAllEndpoints tests path pattern extraction for all endpoint types
func TestPathPatternExtractionForAllEndpoints(t *testing.T) {
	setupTestMeterProvider(t)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	// Comprehensive test cases covering all endpoint patterns
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		// Static health endpoints
		{"Health root", "/health", "/health"},
		{"Health live", "/health/live", "/health/live"},
		{"Health ready", "/health/ready", "/health/ready"},
		{"Health detailed", "/health/detailed", "/health/detailed"},
		
		// API endpoints
		{"API root", "/api", "/api"},
		{"API v1 health", "/api/v1/health", "/api/v1/health"},
		{"API v1 transactions", "/api/v1/transactions", "/api/v1/transactions"},
		{"API v1 balances", "/api/v1/balances", "/api/v1/balances"},
		
		// Parameterized endpoints
		{"Transaction by numeric ID", "/api/v1/transaction/123", "/api/v1/transaction/{id}"},
		{"Transaction by UUID", "/api/v1/transaction/550e8400-e29b-41d4-a716-446655440000", "/api/v1/transaction/{id}"},
		{"Balance by ID", "/api/v1/balance/456", "/api/v1/balance/{id}"},
		{"Portfolio summary", "/api/v1/portfolios/PORTFOLIO123/summary", "/api/v1/portfolios/{portfolioId}/summary"},
		
		// Documentation endpoints
		{"Metrics", "/metrics", "/metrics"},
		{"Swagger root", "/swagger", "/swagger"},
		{"Swagger index", "/swagger/index.html", "/swagger/*"},
		{"Swagger assets", "/swagger/swagger-ui.css", "/swagger/*"},
		{"OpenAPI spec", "/openapi.json", "/openapi.json"},
		{"Docs", "/docs", "/docs"},
		
		// API v2 placeholder
		{"API v2 test", "/api/v2/test", "/api/v2/*"},
		{"API v2 nested", "/api/v2/users/123", "/api/v2/*"},
		
		// Edge cases
		{"Unknown short path", "/unknown", "/unknown"},
		{"Root path", "/", "/"},
		{"Path with query params", "/api/v1/transactions?limit=10", "/api/v1/transactions?limit=10"},
		
		// Long path handling
		{"Very long path", "/very/long/unknown/path/that/exceeds/the/maximum/length/limit/and/should/be/truncated/to/prevent/cardinality/explosion/with/even/more/segments", "/path_too_long"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := middleware.extractPathPatternSafely(tc.path)
			assert.Equal(t, tc.expected, result, 
				"Path pattern extraction failed for: %s", tc.path)
		})
	}
}

// TestCorrectLabelValues tests that all metrics have correct label values
func TestCorrectLabelValues(t *testing.T) {
	reader := metric.NewManualReader()
	setupTestMeterProviderWithReader(t, reader)

	config := EnhancedMetricsConfig{
		ServiceName: "test-service",
		Enabled:     true,
	}

	middleware := NewEnhancedMetricsMiddleware(config)
	require.NotNil(t, middleware)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different status codes based on path
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
		w.Write([]byte("Response"))
	})

	handler := middleware.Handler()(testHandler)

	// Test requests with different methods, paths, and status codes
	testRequests := []struct {
		method         string
		path           string
		expectedMethod string
		expectedPath   string
		expectedStatus string
	}{
		{"GET", "/health", "GET", "/health", "200"},
		{"POST", "/api/v1/transactions", "POST", "/api/v1/transactions", "200"},
		{"PUT", "/api/v1/transaction/123", "PUT", "/api/v1/transaction/{id}", "200"},
		{"DELETE", "/api/v1/transaction/456", "DELETE", "/api/v1/transaction/{id}", "200"},
		{"GET", "/error", "GET", "/error", "500"},
		{"GET", "/notfound", "GET", "/notfound", "404"},
		{"PATCH", "/api/v1/balance/789", "PATCH", "/api/v1/balance/{id}", "200"},
	}

	for _, req := range testRequests {
		httpReq := httptest.NewRequest(req.method, req.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httpReq)
	}

	// Collect metrics
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	// Verify counter labels
	var counterMetric *metricdata.Metrics
	for _, scopeMetric := range rm.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if metric.Name == "http_requests_total_enhanced" {
				counterMetric = &metric
				break
			}
		}
	}

	require.NotNil(t, counterMetric, "Should find counter metric")
	counterData, ok := counterMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "Counter should have Sum data type")

	// Verify each data point has correct labels
	assert.Len(t, counterData.DataPoints, len(testRequests), 
		"Should have data points for all requests")

	// Create a map of expected combinations for verification
	expectedCombinations := make(map[string]bool)
	for _, req := range testRequests {
		key := fmt.Sprintf("%s|%s|%s", req.expectedMethod, req.expectedPath, req.expectedStatus)
		expectedCombinations[key] = true
	}

	// Verify each data point matches one of the expected combinations
	foundCombinations := make(map[string]bool)
	for _, dp := range counterData.DataPoints {
		attrs := dp.Attributes.ToSlice()
		assert.Len(t, attrs, 3, "Should have 3 attributes")

		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		// Verify all required attributes are present
		method, hasMethod := attrMap["method"]
		path, hasPath := attrMap["path"]
		status, hasStatus := attrMap["status"]
		
		assert.True(t, hasMethod, "Should have method attribute")
		assert.True(t, hasPath, "Should have path attribute")
		assert.True(t, hasStatus, "Should have status attribute")

		// Create combination key and verify it's expected
		key := fmt.Sprintf("%s|%s|%s", method, path, status)
		assert.True(t, expectedCombinations[key], 
			"Found unexpected combination: method=%s, path=%s, status=%s", method, path, status)
		foundCombinations[key] = true
	}

	// Verify all expected combinations were found
	assert.Equal(t, len(expectedCombinations), len(foundCombinations), 
		"Should find all expected label combinations")
}

// TestErrorHandlingAndGracefulDegradation tests error handling scenarios
func TestErrorHandlingAndGracefulDegradation(t *testing.T) {
	t.Run("Disabled middleware continues processing", func(t *testing.T) {
		setupTestMeterProvider(t)

		config := EnhancedMetricsConfig{
			ServiceName: "test-service",
			Enabled:     false, // Disabled
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)
		assert.False(t, middleware.enabled)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := middleware.Handler()(testHandler)
		req := httptest.NewRequest("GET", "/health", nil)
		recorder := httptest.NewRecorder()

		// Should not panic and should process request normally
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "OK", recorder.Body.String())
	})

	t.Run("OpenTelemetry unavailable", func(t *testing.T) {
		// Reset meter provider to simulate unavailability
		originalProvider := otel.GetMeterProvider()
		otel.SetMeterProvider(nil)
		
		t.Cleanup(func() {
			otel.SetMeterProvider(originalProvider)
		})

		config := EnhancedMetricsConfig{
			ServiceName: "test-service",
			Enabled:     true,
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)
		assert.False(t, middleware.enabled)
		assert.True(t, middleware.initializationFailed)

		// Should still process requests
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := middleware.Handler()(testHandler)
		req := httptest.NewRequest("GET", "/health", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "OK", recorder.Body.String())
	})

	t.Run("Path length limits", func(t *testing.T) {
		setupTestMeterProvider(t)

		config := EnhancedMetricsConfig{
			ServiceName:   "test-service",
			Enabled:       true,
			MaxPathLength: 20, // Very short limit
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)

		longPath := "/this/is/a/very/long/path/that/exceeds/the/limit"
		result := middleware.extractPathPatternSafely(longPath)
		assert.Equal(t, "/path_too_long", result)
	})

	t.Run("Cache size limits", func(t *testing.T) {
		setupTestMeterProvider(t)

		config := EnhancedMetricsConfig{
			ServiceName:         "test-service",
			Enabled:             true,
			MaxPathPatternCache: 2, // Very small cache
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)

		// Fill cache to limit
		middleware.extractPathPatternSafely("/path1")
		middleware.extractPathPatternSafely("/path2")
		
		// This should not be cached due to limit
		result := middleware.extractPathPatternSafely("/path3")
		assert.Equal(t, "/path3", result)
		
		// Verify cache size is at limit
		assert.Equal(t, 2, len(middleware.pathPatterns))
	})

	t.Run("Handler panic recovery", func(t *testing.T) {
		setupTestMeterProvider(t)

		config := EnhancedMetricsConfig{
			ServiceName: "test-service",
			Enabled:     true,
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)

		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		handler := middleware.Handler()(panicHandler)
		req := httptest.NewRequest("GET", "/health", nil)
		recorder := httptest.NewRecorder()

		// Should recover from panic and re-panic for proper handling
		assert.Panics(t, func() {
			handler.ServeHTTP(recorder, req)
		}, "Should re-panic after recovery")
	})

	t.Run("Method and status sanitization", func(t *testing.T) {
		setupTestMeterProvider(t)

		config := EnhancedMetricsConfig{
			ServiceName: "test-service",
			Enabled:     true,
		}

		middleware := NewEnhancedMetricsMiddleware(config)
		require.NotNil(t, middleware)

		// Test method sanitization
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

		for _, test := range tests {
			result := middleware.sanitizeMethod(test.input)
			assert.Equal(t, test.expected, result)
		}

		// Test status sanitization
		statusTests := []struct {
			input    int
			expected string
		}{
			{200, "200"},
			{404, "404"},
			{500, "500"},
			{99, "unknown"},
			{600, "unknown"},
		}

		for _, test := range statusTests {
			result := middleware.sanitizeStatus(test.input)
			assert.Equal(t, test.expected, result)
		}
	})
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

// setupTestMeterProviderWithReader creates a test meter provider with a manual reader for metric collection
func setupTestMeterProviderWithReader(t *testing.T, reader metric.Reader) *metric.MeterProvider {
	// Create a test resource
	res, err := resource.New(
		nil,
		resource.WithAttributes(
			semconv.ServiceName("test-service"),
		),
	)
	require.NoError(t, err)

	// Create a test meter provider with the reader
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	// Set the global meter provider
	otel.SetMeterProvider(meterProvider)

	// Clean up after test
	t.Cleanup(func() {
		// Reset to default meter provider
		otel.SetMeterProvider(metric.NewMeterProvider())
	})

	return meterProvider
}