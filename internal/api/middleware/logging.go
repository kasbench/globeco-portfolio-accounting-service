package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// LoggingMiddleware provides structured request/response logging
type LoggingMiddleware struct {
	logger logger.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger logger.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// Handler returns a middleware handler function for request/response logging
func (m *LoggingMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate correlation ID if not present
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
			}

			// Add correlation ID to response headers
			w.Header().Set("X-Correlation-ID", correlationID)

			// Add correlation ID to request context
			ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
			r = r.WithContext(ctx)

			// Create response writer wrapper to capture status code and size
			wrappedWriter := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				size:           0,
			}

			// Log request
			m.logger.Info("HTTP Request",
				zap.String("correlation_id", correlationID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("user_agent", r.Header.Get("User-Agent")),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("referer", r.Header.Get("Referer")),
				zap.Int64("content_length", r.ContentLength),
				zap.String("content_type", r.Header.Get("Content-Type")),
			)

			// Call next handler
			next.ServeHTTP(wrappedWriter, r)

			// Calculate duration
			duration := time.Since(start)

			// Log response
			if wrappedWriter.statusCode >= 500 {
				m.logger.Error("HTTP Response",
					zap.String("correlation_id", correlationID),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status_code", wrappedWriter.statusCode),
					zap.Int64("response_size", wrappedWriter.size),
					zap.Duration("duration", duration),
					zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
				)
			} else if wrappedWriter.statusCode >= 400 {
				m.logger.Warn("HTTP Response",
					zap.String("correlation_id", correlationID),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status_code", wrappedWriter.statusCode),
					zap.Int64("response_size", wrappedWriter.size),
					zap.Duration("duration", duration),
					zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
				)
			} else {
				m.logger.Info("HTTP Response",
					zap.String("correlation_id", correlationID),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status_code", wrappedWriter.statusCode),
					zap.Int64("response_size", wrappedWriter.size),
					zap.Duration("duration", duration),
					zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
				)
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and writes the data
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

// CorrelationIDMiddleware adds correlation ID to requests if not present
func CorrelationIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
				r.Header.Set("X-Correlation-ID", correlationID)
			}

			// Add to response headers
			w.Header().Set("X-Correlation-ID", correlationID)

			// Add to context
			ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		return correlationID
	}
	return ""
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()

			// Add to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}
