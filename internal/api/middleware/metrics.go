package middleware

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Global metrics instances to prevent duplicate registration
	globalMetrics     *MetricsMiddleware
	globalMetricsOnce sync.Once
)

// MetricsMiddleware provides Prometheus metrics collection
type MetricsMiddleware struct {
	requestDuration   *prometheus.HistogramVec
	requestCount      *prometheus.CounterVec
	requestSize       *prometheus.HistogramVec
	responseSize      *prometheus.HistogramVec
	activeConnections prometheus.Gauge
	registry          prometheus.Registerer
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(serviceName string) *MetricsMiddleware {
	return NewMetricsMiddlewareWithRegistry(serviceName, nil)
}

// NewMetricsMiddlewareWithRegistry creates a new metrics middleware with custom registry
// If registry is nil, uses the default global registry
func NewMetricsMiddlewareWithRegistry(serviceName string, registry prometheus.Registerer) *MetricsMiddleware {
	// Check if we're in a test environment - if so, use a new registry each time
	if isTestEnvironment() {
		if registry == nil {
			registry = prometheus.NewRegistry()
		}
		return createMetricsMiddleware(serviceName, registry)
	}

	// In production, use singleton pattern to avoid duplicate registration
	globalMetricsOnce.Do(func() {
		if registry == nil {
			registry = prometheus.DefaultRegisterer
		}
		globalMetrics = createMetricsMiddleware(serviceName, registry)
	})

	return globalMetrics
}

// isTestEnvironment detects if we're running in a test environment
func isTestEnvironment() bool {
	// Check common test environment indicators
	for _, arg := range os.Args {
		if arg == "-test.v" || arg == "-test.run" ||
			len(arg) > 5 && arg[:5] == "-test" {
			return true
		}
	}
	// Check if the executable name suggests testing
	executable := os.Args[0]
	return len(executable) > 5 && executable[len(executable)-5:] == ".test" ||
		len(executable) > 4 && executable[len(executable)-4:] == "test"
}

// createMetricsMiddleware creates the actual metrics middleware with registry
func createMetricsMiddleware(serviceName string, registry prometheus.Registerer) *MetricsMiddleware {
	factory := promauto.With(registry)

	return &MetricsMiddleware{
		registry: registry,
		requestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"service", "method", "endpoint", "status_code"},
		),
		requestCount: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"service", "method", "endpoint", "status_code"},
		),
		requestSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "Size of HTTP requests in bytes",
				Buckets: []float64{1, 10, 100, 1000, 10000, 100000, 1000000},
			},
			[]string{"service", "method", "endpoint"},
		),
		responseSize: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "Size of HTTP responses in bytes",
				Buckets: []float64{1, 10, 100, 1000, 10000, 100000, 1000000},
			},
			[]string{"service", "method", "endpoint", "status_code"},
		),
		activeConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_active_connections",
				Help: "Number of active HTTP connections",
			},
		),
	}
}

// Handler returns a middleware handler function for metrics collection
func (m *MetricsMiddleware) Handler(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Increment active connections
			m.activeConnections.Inc()
			defer m.activeConnections.Dec()

			// Extract endpoint pattern (remove path parameters)
			endpoint := m.getEndpointPattern(r.URL.Path)

			// Record request size
			if r.ContentLength > 0 {
				m.requestSize.WithLabelValues(
					serviceName,
					r.Method,
					endpoint,
				).Observe(float64(r.ContentLength))
			}

			// Create metrics response writer to capture status code and size
			metricsWriter := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				size:           0,
			}

			// Call next handler
			next.ServeHTTP(metricsWriter, r)

			// Calculate duration
			duration := time.Since(start)
			statusCode := strconv.Itoa(metricsWriter.statusCode)

			// Record metrics
			m.requestDuration.WithLabelValues(
				serviceName,
				r.Method,
				endpoint,
				statusCode,
			).Observe(duration.Seconds())

			m.requestCount.WithLabelValues(
				serviceName,
				r.Method,
				endpoint,
				statusCode,
			).Inc()

			m.responseSize.WithLabelValues(
				serviceName,
				r.Method,
				endpoint,
				statusCode,
			).Observe(float64(metricsWriter.size))
		})
	}
}

// metricsResponseWriter wraps http.ResponseWriter to capture metrics
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

// WriteHeader captures the status code
func (mrw *metricsResponseWriter) WriteHeader(code int) {
	mrw.statusCode = code
	mrw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and writes the data
func (mrw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := mrw.ResponseWriter.Write(b)
	mrw.size += int64(n)
	return n, err
}

// getEndpointPattern extracts the endpoint pattern from URL path
func (m *MetricsMiddleware) getEndpointPattern(path string) string {
	// Map specific paths to patterns for better metric grouping
	endpointPatterns := map[string]string{
		"/api/v1/transactions": "/api/v1/transactions",
		"/api/v1/balances":     "/api/v1/balances",
		"/health":              "/health",
		"/health/live":         "/health/live",
		"/health/ready":        "/health/ready",
		"/health/detailed":     "/health/detailed",
		"/metrics":             "/metrics",
	}

	// Check for exact matches first
	if pattern, exists := endpointPatterns[path]; exists {
		return pattern
	}

	// Check for parameterized paths
	if len(path) >= 20 && path[:20] == "/api/v1/transaction/" {
		return "/api/v1/transaction/{id}"
	}
	if len(path) >= 18 && path[:18] == "/api/v1/balance/" {
		return "/api/v1/balance/{id}"
	}
	if len(path) >= 17 && path[:17] == "/api/v1/portfolios/" &&
		len(path) > 25 && path[len(path)-8:] == "/summary" {
		return "/api/v1/portfolios/{portfolioId}/summary"
	}

	// Default to the path itself for unknown patterns
	return path
}

// RegisterMetrics registers additional custom metrics
func (m *MetricsMiddleware) RegisterMetrics() {
	// Use the same registry as the middleware instance
	factory := promauto.With(m.registry)

	// Additional business-specific metrics can be registered here
	_ = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transaction_operations_total",
			Help: "Total number of transaction operations",
		},
		[]string{"operation", "status"},
	)

	_ = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "balance_operations_total",
			Help: "Total number of balance operations",
		},
		[]string{"operation", "status"},
	)

	_ = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "table"},
	)

	_ = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hit_ratio",
			Help: "Cache hit ratio",
		},
		[]string{"cache_type"},
	)
}
