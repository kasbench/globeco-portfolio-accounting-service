package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// EnhancedMetricsConfig holds configuration for enhanced metrics middleware
type EnhancedMetricsConfig struct {
	ServiceName string
	Enabled     bool
}

// EnhancedMetricsMiddleware provides OpenTelemetry-based HTTP metrics collection
type EnhancedMetricsMiddleware struct {
	// OpenTelemetry metrics
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter

	// Configuration
	serviceName string
	meter       metric.Meter
	logger      *zap.Logger
	enabled     bool

	// Path pattern cache for performance
	pathPatterns map[string]string
}

// NewEnhancedMetricsMiddleware creates a new enhanced metrics middleware
func NewEnhancedMetricsMiddleware(config EnhancedMetricsConfig) *EnhancedMetricsMiddleware {
	if !config.Enabled {
		return &EnhancedMetricsMiddleware{
			enabled: false,
		}
	}

	// Get the meter from the global meter provider
	meter := otel.GetMeterProvider().Meter(
		"github.com/kasbench/globeco-portfolio-accounting-service/middleware",
		metric.WithInstrumentationVersion("1.0.0"),
	)

	middleware := &EnhancedMetricsMiddleware{
		serviceName:  config.ServiceName,
		meter:        meter,
		enabled:      true,
		pathPatterns: make(map[string]string),
		logger:       zap.L(), // Use global logger
	}

	// Initialize metrics
	if err := middleware.initializeMetrics(); err != nil {
		middleware.logger.Error("Failed to initialize enhanced metrics, disabling middleware",
			zap.Error(err))
		middleware.enabled = false
		return middleware
	}

	return middleware
}

// initializeMetrics creates and registers all OpenTelemetry metrics
func (m *EnhancedMetricsMiddleware) initializeMetrics() error {
	var err error

	// HTTP Requests Total Counter
	m.httpRequestsTotal, err = m.meter.Int64Counter(
		"http_requests_total_enhanced",
		metric.WithDescription("Total number of HTTP requests (enhanced metrics)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// HTTP Request Duration Histogram with specified buckets in milliseconds
	m.httpRequestDuration, err = m.meter.Float64Histogram(
		"http_request_duration_milliseconds",
		metric.WithDescription("Duration of HTTP requests in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
	)
	if err != nil {
		return err
	}

	// HTTP Requests In Flight Gauge
	m.httpRequestsInFlight, err = m.meter.Int64UpDownCounter(
		"http_requests_in_flight_enhanced",
		metric.WithDescription("Number of HTTP requests currently being processed (enhanced metrics)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// Handler returns a middleware handler function for enhanced metrics collection
func (m *EnhancedMetricsMiddleware) Handler() func(http.Handler) http.Handler {
	if !m.enabled {
		// Return a no-op middleware if disabled
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Increment in-flight requests gauge
			m.recordMetricSafely(func() error {
				m.httpRequestsInFlight.Add(r.Context(), 1)
				return nil
			}, "http_requests_in_flight_enhanced_increment")

			// Ensure we decrement the gauge when the request completes
			defer m.recordMetricSafely(func() error {
				m.httpRequestsInFlight.Add(r.Context(), -1)
				return nil
			}, "http_requests_in_flight_enhanced_decrement")

			// Extract path pattern for consistent labeling
			pathPattern := m.extractPathPattern(r.URL.Path)

			// Create enhanced metrics response writer to capture status code
			enhancedWriter := &enhancedMetricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default to 200
			}

			// Call next handler
			next.ServeHTTP(enhancedWriter, r)

			// Calculate duration in milliseconds
			duration := float64(time.Since(start).Nanoseconds()) / 1e6

			// Prepare labels
			method := strings.ToUpper(r.Method)
			status := strconv.Itoa(enhancedWriter.statusCode)

			// Create attributes for metrics
			attrs := []attribute.KeyValue{
				attribute.String("method", method),
				attribute.String("path", pathPattern),
				attribute.String("status", status),
			}

			// Record counter metric
			m.recordMetricSafely(func() error {
				m.httpRequestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
				return nil
			}, "http_requests_total_enhanced")

			// Record histogram metric in milliseconds
			m.recordMetricSafely(func() error {
				m.httpRequestDuration.Record(r.Context(), duration, metric.WithAttributes(attrs...))
				return nil
			}, "http_request_duration_milliseconds")
		})
	}
}

// enhancedMetricsResponseWriter wraps http.ResponseWriter to capture status codes
type enhancedMetricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code
func (w *enhancedMetricsResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write ensures WriteHeader is called with default status if not already called
func (w *enhancedMetricsResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// extractPathPattern normalizes URL paths to route patterns for consistent metrics
func (m *EnhancedMetricsMiddleware) extractPathPattern(path string) string {
	// Check cache first for performance
	if pattern, exists := m.pathPatterns[path]; exists {
		return pattern
	}

	pattern := m.normalizePathPattern(path)

	// Cache the result (simple cache without eviction for now)
	// In production, consider implementing LRU cache with size limits
	if len(m.pathPatterns) < 1000 { // Prevent unlimited growth
		m.pathPatterns[path] = pattern
	}

	return pattern
}

// normalizePathPattern converts actual paths to route patterns
func (m *EnhancedMetricsMiddleware) normalizePathPattern(path string) string {
	// Static path mappings for exact matches
	staticPaths := map[string]string{
		"/health":                     "/health",
		"/health/live":                "/health/live",
		"/health/ready":               "/health/ready",
		"/health/detailed":            "/health/detailed",
		"/metrics":                    "/metrics",
		"/api":                        "/api",
		"/swagger":                    "/swagger",
		"/openapi.json":               "/openapi.json",
		"/docs":                       "/docs",
		"/api/v1/health":              "/api/v1/health",
		"/api/v1/health/live":         "/api/v1/health/live",
		"/api/v1/health/ready":        "/api/v1/health/ready",
		"/api/v1/health/detailed":     "/api/v1/health/detailed",
		"/api/v1/transactions":        "/api/v1/transactions",
		"/api/v1/balances":            "/api/v1/balances",
	}

	// Check for exact static matches first
	if pattern, exists := staticPaths[path]; exists {
		return pattern
	}

	// Dynamic path pattern matching using regex
	patterns := []struct {
		regex   *regexp.Regexp
		pattern string
	}{
		// Transaction by ID: /api/v1/transaction/{id}
		{
			regex:   regexp.MustCompile(`^/api/v1/transaction/[^/]+$`),
			pattern: "/api/v1/transaction/{id}",
		},
		// Balance by ID: /api/v1/balance/{id}
		{
			regex:   regexp.MustCompile(`^/api/v1/balance/[^/]+$`),
			pattern: "/api/v1/balance/{id}",
		},
		// Portfolio summary: /api/v1/portfolios/{portfolioId}/summary
		{
			regex:   regexp.MustCompile(`^/api/v1/portfolios/[^/]+/summary$`),
			pattern: "/api/v1/portfolios/{portfolioId}/summary",
		},
		// Swagger UI paths: /swagger/*
		{
			regex:   regexp.MustCompile(`^/swagger/.*$`),
			pattern: "/swagger/*",
		},
		// API v2 placeholder: /api/v2/*
		{
			regex:   regexp.MustCompile(`^/api/v2/.*$`),
			pattern: "/api/v2/*",
		},
	}

	// Check dynamic patterns
	for _, p := range patterns {
		if p.regex.MatchString(path) {
			return p.pattern
		}
	}

	// For unknown paths, return the path itself but limit length to prevent cardinality explosion
	if len(path) > 100 {
		return "/unknown_long_path"
	}

	return path
}

// recordMetricSafely wraps metric recording with error handling to prevent request blocking
func (m *EnhancedMetricsMiddleware) recordMetricSafely(recordFunc func() error, metricName string) {
	if err := recordFunc(); err != nil {
		m.logger.Error("Failed to record enhanced metric",
			zap.String("metric", metricName),
			zap.Error(err))
	}
}