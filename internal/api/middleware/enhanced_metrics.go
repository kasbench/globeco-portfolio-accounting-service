package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// EnhancedMetricsConfig holds configuration for enhanced metrics middleware
type EnhancedMetricsConfig struct {
	ServiceName           string
	Enabled               bool
	MaxPathPatternCache   int  // Maximum number of path patterns to cache
	MaxPathLength         int  // Maximum path length to prevent cardinality explosion
	EnableFailsafeLogging bool // Enable detailed error logging for debugging
}

// EnhancedMetricsMiddleware provides OpenTelemetry-based HTTP metrics collection
type EnhancedMetricsMiddleware struct {
	// OpenTelemetry metrics
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter

	// Configuration
	serviceName           string
	meter                 metric.Meter
	logger                logger.Logger
	enabled               bool
	maxPathPatternCache   int
	maxPathLength         int
	enableFailsafeLogging bool

	// Path pattern cache for performance with thread safety
	pathPatterns map[string]string
	cacheMutex   sync.RWMutex

	// Error tracking for graceful degradation
	initializationFailed bool
	errorCount           int64
	lastErrorTime        time.Time
	errorMutex           sync.RWMutex
}

// NewEnhancedMetricsMiddleware creates a new enhanced metrics middleware
func NewEnhancedMetricsMiddleware(config EnhancedMetricsConfig) *EnhancedMetricsMiddleware {
	// Set default values for configuration
	if config.MaxPathPatternCache <= 0 {
		config.MaxPathPatternCache = 1000
	}
	if config.MaxPathLength <= 0 {
		config.MaxPathLength = 100
	}

	middleware := &EnhancedMetricsMiddleware{
		serviceName:           config.ServiceName,
		enabled:               config.Enabled,
		maxPathPatternCache:   config.MaxPathPatternCache,
		maxPathLength:         config.MaxPathLength,
		enableFailsafeLogging: config.EnableFailsafeLogging,
		pathPatterns:          make(map[string]string),
		logger:                logger.GetGlobal(),
		initializationFailed:  false,
	}

	if !config.Enabled {
		middleware.logger.Info("Enhanced metrics middleware is disabled")
		return middleware
	}

	// Attempt to get the meter from the global meter provider with error handling
	meterProvider := otel.GetMeterProvider()
	if meterProvider == nil {
		middleware.logger.Warn("OpenTelemetry meter provider is not available, disabling enhanced metrics",
			zap.String("service", config.ServiceName))
		middleware.enabled = false
		middleware.initializationFailed = true
		return middleware
	}

	middleware.meter = meterProvider.Meter(
		"github.com/kasbench/globeco-portfolio-accounting-service/middleware",
		metric.WithInstrumentationVersion("1.0.0"),
	)

	// Initialize metrics with comprehensive error handling
	if err := middleware.initializeMetrics(); err != nil {
		middleware.logger.Error("Failed to initialize enhanced metrics, disabling middleware",
			zap.Error(err),
			zap.String("service", config.ServiceName))
		middleware.enabled = false
		middleware.initializationFailed = true
		return middleware
	}

	middleware.logger.Info("Enhanced metrics middleware initialized successfully",
		zap.String("service", config.ServiceName),
		zap.Int("max_path_cache", config.MaxPathPatternCache),
		zap.Int("max_path_length", config.MaxPathLength))

	return middleware
}

// initializeMetrics creates and registers all OpenTelemetry metrics with comprehensive error handling
func (m *EnhancedMetricsMiddleware) initializeMetrics() error {
	if m.meter == nil {
		return fmt.Errorf("meter is nil, cannot initialize metrics")
	}

	var err error
	var initErrors []string

	// HTTP Requests Total Counter
	m.httpRequestsTotal, err = m.meter.Int64Counter(
		"http_requests_total_enhanced",
		metric.WithDescription("Total number of HTTP requests (enhanced metrics)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		initErrors = append(initErrors, fmt.Sprintf("failed to create http_requests_total_enhanced counter: %v", err))
		m.logger.Error("Failed to create HTTP requests total counter",
			zap.Error(err))
	}

	// HTTP Request Duration Histogram with specified buckets in milliseconds
	m.httpRequestDuration, err = m.meter.Float64Histogram(
		"http_request_duration_milliseconds",
		metric.WithDescription("Duration of HTTP requests in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
	)
	if err != nil {
		initErrors = append(initErrors, fmt.Sprintf("failed to create http_request_duration_milliseconds histogram: %v", err))
		m.logger.Error("Failed to create HTTP request duration histogram",
			zap.Error(err))
	}

	// HTTP Requests In Flight Gauge
	m.httpRequestsInFlight, err = m.meter.Int64UpDownCounter(
		"http_requests_in_flight_enhanced",
		metric.WithDescription("Number of HTTP requests currently being processed (enhanced metrics)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		initErrors = append(initErrors, fmt.Sprintf("failed to create http_requests_in_flight_enhanced gauge: %v", err))
		m.logger.Error("Failed to create HTTP requests in flight gauge",
			zap.Error(err))
	}

	// If any metrics failed to initialize, return a combined error
	if len(initErrors) > 0 {
		combinedError := fmt.Errorf("metric initialization failed: %v", strings.Join(initErrors, "; "))
		m.logger.Error("Enhanced metrics initialization completed with errors",
			zap.Int("failed_metrics", len(initErrors)),
			zap.Strings("errors", initErrors))
		return combinedError
	}

	m.logger.Info("All enhanced metrics initialized successfully",
		zap.String("counter", "http_requests_total_enhanced"),
		zap.String("histogram", "http_request_duration_milliseconds"),
		zap.String("gauge", "http_requests_in_flight_enhanced"))

	return nil
}

// Handler returns a middleware handler function for enhanced metrics collection
func (m *EnhancedMetricsMiddleware) Handler() func(http.Handler) http.Handler {
	if !m.enabled {
		// Return a no-op middleware if disabled
		if m.initializationFailed {
			m.logger.Debug("Enhanced metrics middleware is disabled due to initialization failure")
		}
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ensure we don't panic if metrics are nil due to initialization failures
			if m.httpRequestsInFlight == nil || m.httpRequestsTotal == nil || m.httpRequestDuration == nil {
				m.logMetricError("Metrics not properly initialized, skipping metric collection", "initialization_check", nil)
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Increment in-flight requests gauge with error recovery
			m.recordMetricSafely(func() error {
				if m.httpRequestsInFlight == nil {
					return fmt.Errorf("in-flight gauge is nil")
				}
				m.httpRequestsInFlight.Add(r.Context(), 1)
				return nil
			}, "http_requests_in_flight_enhanced_increment")

			// Ensure we decrement the gauge when the request completes, even if other operations fail
			defer func() {
				// Use a separate context with timeout to ensure decrement doesn't hang
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				m.recordMetricSafely(func() error {
					if m.httpRequestsInFlight == nil {
						return fmt.Errorf("in-flight gauge is nil")
					}
					m.httpRequestsInFlight.Add(ctx, -1)
					return nil
				}, "http_requests_in_flight_enhanced_decrement")
			}()

			// Extract path pattern for consistent labeling with cardinality protection
			pathPattern := m.extractPathPatternSafely(r.URL.Path)

			// Create enhanced metrics response writer to capture status code
			enhancedWriter := &enhancedMetricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default to 200
			}

			// Call next handler with panic recovery
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						m.logger.Error("Panic recovered in enhanced metrics middleware",
							zap.Any("panic", recovered),
							zap.String("path", r.URL.Path),
							zap.String("method", r.Method))
						// Re-panic to let the application handle it properly
						panic(recovered)
					}
				}()
				next.ServeHTTP(enhancedWriter, r)
			}()

			// Calculate duration in milliseconds
			duration := float64(time.Since(start).Nanoseconds()) / 1e6

			// Prepare labels with validation
			method := m.sanitizeMethod(r.Method)
			status := m.sanitizeStatus(enhancedWriter.statusCode)

			// Create attributes for metrics with validation
			attrs := []attribute.KeyValue{
				attribute.String("method", method),
				attribute.String("path", pathPattern),
				attribute.String("status", status),
			}

			// Record counter metric with error handling
			m.recordMetricSafely(func() error {
				if m.httpRequestsTotal == nil {
					return fmt.Errorf("requests total counter is nil")
				}
				m.httpRequestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
				return nil
			}, "http_requests_total_enhanced")

			// Record histogram metric in milliseconds with validation
			m.recordMetricSafely(func() error {
				if m.httpRequestDuration == nil {
					return fmt.Errorf("request duration histogram is nil")
				}
				// Validate duration to prevent extreme values
				if duration < 0 || duration > 300000 { // 5 minutes max
					return fmt.Errorf("invalid duration: %f ms", duration)
				}
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

// extractPathPatternSafely normalizes URL paths to route patterns with cardinality protection
func (m *EnhancedMetricsMiddleware) extractPathPatternSafely(path string) string {
	// Validate path length to prevent cardinality explosion
	if len(path) > m.maxPathLength {
		m.logMetricError("Path too long, truncating for metrics", "path_length_limit", 
			fmt.Errorf("path length %d exceeds limit %d", len(path), m.maxPathLength))
		return "/path_too_long"
	}

	// Check cache first for performance with read lock
	m.cacheMutex.RLock()
	if pattern, exists := m.pathPatterns[path]; exists {
		m.cacheMutex.RUnlock()
		return pattern
	}
	m.cacheMutex.RUnlock()

	// Normalize the path pattern
	pattern := m.normalizePathPattern(path)

	// Cache the result with write lock and size limit protection
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if existingPattern, exists := m.pathPatterns[path]; exists {
		return existingPattern
	}

	// Prevent unlimited cache growth
	if len(m.pathPatterns) >= m.maxPathPatternCache {
		// Log warning about cache limit reached
		m.logMetricError("Path pattern cache limit reached, not caching new patterns", "cache_limit_reached",
			fmt.Errorf("cache size %d reached limit %d", len(m.pathPatterns), m.maxPathPatternCache))
		return pattern
	}

	m.pathPatterns[path] = pattern
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

// recordMetricSafely wraps metric recording with comprehensive error handling to prevent request blocking
func (m *EnhancedMetricsMiddleware) recordMetricSafely(recordFunc func() error, metricName string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			m.logMetricError("Panic recovered during metric recording", metricName, 
				fmt.Errorf("panic: %v", recovered))
		}
	}()

	// Use a timeout context to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create a channel to handle the metric recording with timeout
	done := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- fmt.Errorf("panic in metric recording goroutine: %v", recovered)
			}
		}()
		done <- recordFunc()
	}()

	select {
	case err := <-done:
		if err != nil {
			m.logMetricError("Failed to record metric", metricName, err)
		}
	case <-ctx.Done():
		m.logMetricError("Metric recording timed out", metricName, ctx.Err())
	}
}

// logMetricError handles error logging with rate limiting to prevent log spam
func (m *EnhancedMetricsMiddleware) logMetricError(message, metricName string, err error) {
	m.errorMutex.Lock()
	defer m.errorMutex.Unlock()

	m.errorCount++
	now := time.Now()

	// Rate limit error logging to prevent spam (max 1 error log per second per metric type)
	if now.Sub(m.lastErrorTime) < time.Second {
		return
	}
	m.lastErrorTime = now

	fields := []zap.Field{
		zap.String("metric", metricName),
		zap.Int64("total_errors", m.errorCount),
		zap.String("component", "enhanced_metrics_middleware"),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	if m.enableFailsafeLogging {
		m.logger.Error(message, fields...)
	} else {
		// In production, use warn level to reduce noise
		m.logger.Warn(message, fields...)
	}
}

// sanitizeMethod ensures HTTP method is valid and uppercase
func (m *EnhancedMetricsMiddleware) sanitizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return "UNKNOWN"
	}
	// Limit method length to prevent cardinality issues
	if len(method) > 10 {
		return "INVALID"
	}
	return method
}

// sanitizeStatus ensures HTTP status code is valid
func (m *EnhancedMetricsMiddleware) sanitizeStatus(statusCode int) string {
	if statusCode < 100 || statusCode > 599 {
		return "unknown"
	}
	return strconv.Itoa(statusCode)
}

// GetMetricsStatus returns the current status of the metrics middleware for health checks
func (m *EnhancedMetricsMiddleware) GetMetricsStatus() map[string]interface{} {
	m.errorMutex.RLock()
	defer m.errorMutex.RUnlock()

	m.cacheMutex.RLock()
	cacheSize := len(m.pathPatterns)
	m.cacheMutex.RUnlock()

	return map[string]interface{}{
		"enabled":               m.enabled,
		"initialization_failed": m.initializationFailed,
		"error_count":           m.errorCount,
		"cache_size":            cacheSize,
		"max_cache_size":        m.maxPathPatternCache,
		"last_error_time":       m.lastErrorTime,
	}
}