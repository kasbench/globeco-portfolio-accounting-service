package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// HealthHandler handles HTTP requests for health check operations
type HealthHandler struct {
	transactionService services.TransactionService
	balanceService     services.BalanceService
	logger             logger.Logger
	version            string
	environment        string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	transactionService services.TransactionService,
	balanceService services.BalanceService,
	logger logger.Logger,
	version string,
	environment string,
) *HealthHandler {
	return &HealthHandler{
		transactionService: transactionService,
		balanceService:     balanceService,
		logger:             logger,
		version:            version,
		environment:        environment,
	}
}

// GetHealth handles GET /health - basic health check
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

// GetReadiness handles GET /health/ready - Kubernetes readiness probe
func (h *HealthHandler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Debug("GET /health/ready",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	checks := make(map[string]interface{})
	allHealthy := true

	// Check transaction service health
	if err := h.transactionService.GetServiceHealth(ctx); err != nil {
		h.logger.Warn("Transaction service health check failed", zap.Error(err))
		checks["transaction_service"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		allHealthy = false
	} else {
		checks["transaction_service"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	// Check balance service health
	if err := h.balanceService.GetServiceHealth(ctx); err != nil {
		h.logger.Warn("Balance service health check failed", zap.Error(err))
		checks["balance_service"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		allHealthy = false
	} else {
		checks["balance_service"] = map[string]interface{}{
			"status": "healthy",
		}
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

// GetDetailedHealth handles GET /health/detailed - comprehensive health check
func (h *HealthHandler) GetDetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("GET /health/detailed",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	checks := make(map[string]interface{})
	allHealthy := true

	// Check transaction service health
	if err := h.transactionService.GetServiceHealth(ctx); err != nil {
		checks["transaction_service"] = map[string]interface{}{
			"status":     "unhealthy",
			"error":      err.Error(),
			"checked_at": time.Now(),
		}
		allHealthy = false
	} else {
		checks["transaction_service"] = map[string]interface{}{
			"status":     "healthy",
			"checked_at": time.Now(),
		}
	}

	// Check balance service health
	if err := h.balanceService.GetServiceHealth(ctx); err != nil {
		checks["balance_service"] = map[string]interface{}{
			"status":     "unhealthy",
			"error":      err.Error(),
			"checked_at": time.Now(),
		}
		allHealthy = false
	} else {
		checks["balance_service"] = map[string]interface{}{
			"status":     "healthy",
			"checked_at": time.Now(),
		}
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
