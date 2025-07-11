package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/api/handlers"
	apiMiddleware "github.com/kasbench/globeco-portfolio-accounting-service/internal/api/middleware"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Config holds router configuration
type Config struct {
	ServiceName   string
	Version       string
	Environment   string
	EnableMetrics bool
	EnableCORS    bool
	CORSConfig    apiMiddleware.CORSConfig
}

// RouterDependencies holds all dependencies needed for route setup
type RouterDependencies struct {
	TransactionHandler *handlers.TransactionHandler
	BalanceHandler     *handlers.BalanceHandler
	HealthHandler      *handlers.HealthHandler
	SwaggerHandler     *handlers.SwaggerHandler
	Logger             logger.Logger
	MetricsRegistry    prometheus.Registerer // Optional custom registry for metrics (used in tests)
}

// SetupRouter creates and configures the main router with all routes and middleware
func SetupRouter(config Config, deps RouterDependencies) http.Handler {
	r := chi.NewRouter()

	// Create middleware instances
	loggingMiddleware := apiMiddleware.NewLoggingMiddleware(deps.Logger)

	var metricsMiddleware *apiMiddleware.MetricsMiddleware
	if deps.MetricsRegistry != nil {
		// Use custom registry (for tests)
		metricsMiddleware = apiMiddleware.NewMetricsMiddlewareWithRegistry(config.ServiceName, deps.MetricsRegistry)
	} else {
		// Use default global registry (for production)
		metricsMiddleware = apiMiddleware.NewMetricsMiddleware(config.ServiceName)
	}

	// Global middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(apiMiddleware.RequestIDMiddleware())
	r.Use(apiMiddleware.CorrelationIDMiddleware())

	// Add CORS middleware if enabled
	if config.EnableCORS {
		if len(config.CORSConfig.AllowedOrigins) > 0 {
			r.Use(apiMiddleware.CORSWithConfig(config.CORSConfig))
		} else {
			r.Use(apiMiddleware.CORS())
		}
	}

	// Add metrics middleware if enabled
	if config.EnableMetrics {
		r.Use(metricsMiddleware.Handler(config.ServiceName))
		metricsMiddleware.RegisterMetrics()
	}

	// Add logging middleware
	r.Use(loggingMiddleware.Handler())

	// Setup routes
	setupHealthRoutes(r, deps.HealthHandler)
	setupAPIRoutes(r, deps)
	setupDocumentationRoutes(r, deps.SwaggerHandler)
	setupMetricsRoute(r, config.EnableMetrics)

	// Wrap router with OTel HTTP handler for tracing
	return otelhttp.NewHandler(r, config.ServiceName)
}

// setupHealthRoutes configures health check endpoints
func setupHealthRoutes(r chi.Router, healthHandler *handlers.HealthHandler) {
	// Basic health check
	r.Get("/health", healthHandler.GetHealth)

	// Kubernetes probes
	r.Get("/health/live", healthHandler.GetLiveness)
	r.Get("/health/ready", healthHandler.GetReadiness)
	r.Get("/health/detailed", healthHandler.GetDetailedHealth)
}

// setupAPIRoutes configures API endpoints with versioning
func setupAPIRoutes(r chi.Router, deps RouterDependencies) {
	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health endpoints under API v1
		r.Get("/health", deps.HealthHandler.GetHealth)
		r.Get("/health/live", deps.HealthHandler.GetLiveness)
		r.Get("/health/ready", deps.HealthHandler.GetReadiness)
		r.Get("/health/detailed", deps.HealthHandler.GetDetailedHealth)

		// Transaction endpoints
		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", deps.TransactionHandler.GetTransactions)
			r.Post("/", deps.TransactionHandler.CreateTransactions)
		})

		r.Route("/transaction", func(r chi.Router) {
			r.Get("/{id}", deps.TransactionHandler.GetTransactionByID)
		})

		// Balance endpoints
		r.Route("/balances", func(r chi.Router) {
			r.Get("/", deps.BalanceHandler.GetBalances)
		})

		r.Route("/balance", func(r chi.Router) {
			r.Get("/{id}", deps.BalanceHandler.GetBalanceByID)
		})

		// Portfolio endpoints
		r.Route("/portfolios", func(r chi.Router) {
			r.Get("/{portfolioId}/summary", deps.BalanceHandler.GetPortfolioSummary)
		})
	})

	// API v2 routes (placeholder for future versions)
	r.Route("/api/v2", func(r chi.Router) {
		// Future API version endpoints can be added here
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"message": "API v2 not implemented yet"}`))
		})
	})
}

// setupMetricsRoute configures Prometheus metrics endpoint
func setupMetricsRoute(r chi.Router, enableMetrics bool) {
	if enableMetrics {
		r.Handle("/metrics", promhttp.Handler())
	}
}

// setupDocumentationRoutes configures Swagger UI and API documentation endpoints
func setupDocumentationRoutes(r chi.Router, swaggerHandler *handlers.SwaggerHandler) {
	// API information endpoint
	r.Get("/api", swaggerHandler.GetAPIInfo)

	// Swagger UI endpoints
	r.Get("/swagger/*", swaggerHandler.GetSwaggerUI)
	r.Get("/swagger", swaggerHandler.RedirectToSwagger)

	// Direct access to OpenAPI spec
	r.Get("/openapi.json", swaggerHandler.GetOpenAPISpec)

	// Alternative documentation (could add Redoc here in the future)
	r.Get("/docs", swaggerHandler.RedirectToSwagger)
}

// SetupV1Router creates a router with only v1 API endpoints (for testing)
func SetupV1Router(deps RouterDependencies) chi.Router {
	r := chi.NewRouter()

	// Minimal middleware for testing
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Only v1 API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Transaction endpoints
		r.Get("/transactions", deps.TransactionHandler.GetTransactions)
		r.Post("/transactions", deps.TransactionHandler.CreateTransactions)
		r.Get("/transaction/{id}", deps.TransactionHandler.GetTransactionByID)

		// Balance endpoints
		r.Get("/balances", deps.BalanceHandler.GetBalances)
		r.Get("/balance/{id}", deps.BalanceHandler.GetBalanceByID)

		// Portfolio endpoints
		r.Get("/portfolios/{portfolioId}/summary", deps.BalanceHandler.GetPortfolioSummary)
	})

	return r
}

// GetAPIVersion returns the current API version
func GetAPIVersion() string {
	return "v1"
}

// GetAllRoutes returns a list of all configured routes (for documentation)
func GetAllRoutes() []Route {
	return []Route{
		// Health endpoints (root level)
		{Method: "GET", Path: "/health", Description: "Basic health check"},
		{Method: "GET", Path: "/health/live", Description: "Kubernetes liveness probe"},
		{Method: "GET", Path: "/health/ready", Description: "Kubernetes readiness probe"},
		{Method: "GET", Path: "/health/detailed", Description: "Detailed health check"},

		// Health endpoints (API v1)
		{Method: "GET", Path: "/api/v1/health", Description: "Basic health check (API v1)"},
		{Method: "GET", Path: "/api/v1/health/live", Description: "Kubernetes liveness probe (API v1)"},
		{Method: "GET", Path: "/api/v1/health/ready", Description: "Kubernetes readiness probe (API v1)"},
		{Method: "GET", Path: "/api/v1/health/detailed", Description: "Detailed health check (API v1)"},

		// Metrics endpoint
		{Method: "GET", Path: "/metrics", Description: "Prometheus metrics"},

		// Documentation endpoints
		{Method: "GET", Path: "/api", Description: "API information and links"},
		{Method: "GET", Path: "/swagger", Description: "Redirect to Swagger UI"},
		{Method: "GET", Path: "/swagger/*", Description: "Swagger UI interface"},
		{Method: "GET", Path: "/openapi.json", Description: "OpenAPI 3.0 specification"},
		{Method: "GET", Path: "/docs", Description: "API documentation (redirect to Swagger)"},

		// API v1 endpoints
		{Method: "GET", Path: "/api/v1/transactions", Description: "Get transactions"},
		{Method: "POST", Path: "/api/v1/transactions", Description: "Create transactions"},
		{Method: "GET", Path: "/api/v1/transaction/{id}", Description: "Get transaction by ID"},
		{Method: "GET", Path: "/api/v1/balances", Description: "Get balances"},
		{Method: "GET", Path: "/api/v1/balance/{id}", Description: "Get balance by ID"},
		{Method: "GET", Path: "/api/v1/portfolios/{portfolioId}/summary", Description: "Get portfolio summary"},

		// API v2 placeholder
		{Method: "GET", Path: "/api/v2/", Description: "API v2 placeholder (not implemented)"},
	}
}

// Route represents a single API route
type Route struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}
