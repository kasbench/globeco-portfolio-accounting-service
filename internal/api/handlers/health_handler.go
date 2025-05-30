package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/external"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// HealthHandler handles HTTP requests for health check operations
type HealthHandler struct {
	portfolioClient external.PortfolioClient
	securityClient  external.SecurityClient
	logger          logger.Logger
	version         string
	environment     string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	portfolioClient external.PortfolioClient,
	securityClient external.SecurityClient,
	logger logger.Logger,
	version string,
	environment string,
) *HealthHandler {
	return &HealthHandler{
		portfolioClient: portfolioClient,
		securityClient:  securityClient,
		logger:          logger,
		version:         version,
		environment:     environment,
	}
}

// GetHealth performs a basic health check
// @Summary Basic health check
// @Description Returns basic service health status
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Service is healthy"
// @Failure 503 {object} dto.ErrorResponse "Service is unhealthy"
// @Router /health [get]
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("GET /health",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	response := dto.HealthResponse{
		Status:      "healthy",
		Timestamp:   time.Now(),
		Version:     h.version,
		Environment: h.environment,
		Checks:      make(map[string]interface{}),
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health response", zap.Error(err))
		return
	}

	h.logger.Info("Health check successful")
}

// GetLiveness handles GET /health/live - Kubernetes liveness probe
// @Summary Kubernetes liveness probe
// @Description Returns liveness status for Kubernetes health checking (always returns healthy for running service)
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Service is alive"
// @Router /health/live [get]
func (h *HealthHandler) GetLiveness(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("GET /health/live",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
		"service":   "globeco-portfolio-accounting-service",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode liveness response", zap.Error(err))
		return
	}
}

// GetReadiness performs a Kubernetes readiness probe check
// @Summary Kubernetes readiness probe
// @Description Returns readiness status for Kubernetes traffic routing. Checks external service connectivity (portfolio and security services).
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Service is ready to receive traffic"
// @Failure 503 {object} dto.ErrorResponse "Service is not ready (external services unavailable)"
// @Router /health/ready [get]
func (h *HealthHandler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Debug("GET /health/ready",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	checks := make(map[string]interface{})
	allHealthy := true

	// Check portfolio service health (handle nil service gracefully)
	if h.portfolioClient != nil {
		if err := h.portfolioClient.Health(ctx); err != nil {
			h.logger.Warn("Portfolio service health check failed", zap.Error(err))
			checks["portfolio_service"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			allHealthy = false
		} else {
			checks["portfolio_service"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	} else {
		checks["portfolio_service"] = map[string]interface{}{
			"status":  "not_initialized",
			"message": "Portfolio service not yet initialized",
		}
		allHealthy = false
	}

	// Check security service health (handle nil service gracefully)
	if h.securityClient != nil {
		if err := h.securityClient.Health(ctx); err != nil {
			h.logger.Warn("Security service health check failed", zap.Error(err))
			checks["security_service"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			allHealthy = false
		} else {
			checks["security_service"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	} else {
		checks["security_service"] = map[string]interface{}{
			"status":  "not_initialized",
			"message": "Security service not yet initialized",
		}
		allHealthy = false
	}

	status := "ready"
	statusCode := http.StatusOK

	if !allHealthy {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now(),
		"checks":    checks,
		"service":   "globeco-portfolio-accounting-service",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode readiness response", zap.Error(err))
		return
	}

	if allHealthy {
		h.logger.Debug("Readiness check successful")
	} else {
		h.logger.Warn("Readiness check failed", zap.Any("checks", checks))
	}
}

// GetDetailedHealth performs comprehensive health checks with detailed status
// @Summary Detailed health check with dependencies
// @Description Returns comprehensive health status including external services (portfolio and security services) connectivity and response times
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Detailed health status with all dependency checks"
// @Failure 503 {object} dto.ErrorResponse "Service or external services are unhealthy"
// @Router /health/detailed [get]
func (h *HealthHandler) GetDetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("GET /health/detailed",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	checks := make(map[string]interface{})
	allHealthy := true

	// Check portfolio service health (handle nil service gracefully)
	if h.portfolioClient != nil {
		if err := h.portfolioClient.Health(ctx); err != nil {
			checks["portfolio_service"] = map[string]interface{}{
				"status":     "unhealthy",
				"error":      err.Error(),
				"checked_at": time.Now(),
			}
			allHealthy = false
		} else {
			checks["portfolio_service"] = map[string]interface{}{
				"status":     "healthy",
				"checked_at": time.Now(),
			}
		}
	} else {
		checks["portfolio_service"] = map[string]interface{}{
			"status":     "not_initialized",
			"message":    "Portfolio service not yet initialized",
			"checked_at": time.Now(),
		}
		allHealthy = false
	}

	// Check security service health (handle nil service gracefully)
	if h.securityClient != nil {
		if err := h.securityClient.Health(ctx); err != nil {
			checks["security_service"] = map[string]interface{}{
				"status":     "unhealthy",
				"error":      err.Error(),
				"checked_at": time.Now(),
			}
			allHealthy = false
		} else {
			checks["security_service"] = map[string]interface{}{
				"status":     "healthy",
				"checked_at": time.Now(),
			}
		}
	} else {
		checks["security_service"] = map[string]interface{}{
			"status":     "not_initialized",
			"message":    "Security service not yet initialized",
			"checked_at": time.Now(),
		}
		allHealthy = false
	}

	overallStatus := "healthy"
	if !allHealthy {
		overallStatus = "degraded"
	}

	response := dto.HealthResponse{
		Status:      overallStatus,
		Timestamp:   time.Now(),
		Version:     h.version,
		Environment: h.environment,
		Checks:      checks,
	}

	statusCode := http.StatusOK
	if !allHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode detailed health response", zap.Error(err))
		return
	}

	if allHealthy {
		h.logger.Info("Detailed health check successful")
	} else {
		h.logger.Warn("Detailed health check shows degraded status", zap.Any("checks", checks))
	}
}
