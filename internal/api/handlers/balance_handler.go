package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"go.uber.org/zap"
)

// BalanceHandler handles HTTP requests for balance operations
type BalanceHandler struct {
	balanceService services.BalanceService
	logger         logger.Logger
}

// NewBalanceHandler creates a new balance handler
func NewBalanceHandler(balanceService services.BalanceService, logger logger.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
		logger:         logger,
	}
}

// GetBalances retrieves balances with optional filtering and pagination
// @Summary Get balances with filtering
// @Description Retrieve portfolio balances with optional filtering by portfolio, security, and quantity ranges. Supports pagination and sorting.
// @Tags Balances
// @Accept json
// @Produce json
// @Param portfolio_id query string false "Filter by portfolio ID (24 characters)"
// @Param security_id query string false "Filter by security ID (24 characters). Use 'null' for cash balances"
// @Param offset query int false "Pagination offset (default: 0)" minimum(0)
// @Param limit query int false "Number of records to return (default: 50, max: 1000)" minimum(1) maximum(1000)
// @Param sortby query string false "Sort fields (comma-separated): portfolio_id,security_id"
// @Success 200 {object} dto.BalanceListResponse "Successfully retrieved balances"
// @Failure 400 {object} dto.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /balances [get]
func (h *BalanceHandler) GetBalances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filter, err := h.parseBalanceFilter(r)
	if err != nil {
		h.logger.Error("Failed to parse balance filter", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILTER", err.Error())
		return
	}

	// Log the request
	h.logger.Info("GET /api/v1/balances",
		zap.Any("filter", filter),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Get balances from service
	result, err := h.balanceService.GetBalances(ctx, *filter)
	if err != nil {
		h.logger.Error("Failed to get balances", zap.Error(err), zap.Any("filter", filter))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve balances")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully retrieved balances",
		zap.Int("count", len(result.Balances)),
		zap.Int64("total", result.Pagination.Total),
		zap.Int("page", result.Pagination.Page),
		zap.Int("limit", result.Pagination.Limit))
}

// GetBalanceByID retrieves a specific balance by its ID
// @Summary Get balance by ID
// @Description Retrieve a specific balance record using its unique ID
// @Tags Balances
// @Accept json
// @Produce json
// @Param id path int true "Balance ID" minimum(1)
// @Success 200 {object} dto.BalanceDTO "Successfully retrieved balance"
// @Failure 400 {object} dto.ErrorResponse "Invalid balance ID"
// @Failure 404 {object} dto.ErrorResponse "Balance not found"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /balance/{id} [get]
func (h *BalanceHandler) GetBalanceByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse balance ID from URL
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_ID", "Balance ID is required")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid balance ID", zap.String("id", idStr), zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_ID", "Balance ID must be a valid integer")
		return
	}

	// Log the request
	h.logger.Info("GET /api/v1/balance/{id}",
		zap.Int64("id", id),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Get balance from service
	balance, err := h.balanceService.GetBalance(ctx, id)
	if err != nil {
		// Check if balance not found (assuming error message contains this info)
		if strings.Contains(err.Error(), "not found") {
			h.logger.Warn("Balance not found", zap.Int64("id", id))
			h.writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Balance not found")
			return
		}
		h.logger.Error("Failed to get balance", zap.Error(err), zap.Int64("id", id))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve balance")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(balance); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully retrieved balance", zap.Int64("id", id))
}

// GetPortfolioSummary retrieves a comprehensive portfolio summary
// @Summary Get portfolio summary
// @Description Get a comprehensive summary of a portfolio including cash balance and all security positions with market values and statistics
// @Tags Balances
// @Accept json
// @Produce json
// @Param portfolioId path string true "Portfolio ID (24 characters)"
// @Success 200 {object} dto.PortfolioSummaryDTO "Successfully retrieved portfolio summary"
// @Failure 400 {object} dto.ErrorResponse "Invalid portfolio ID"
// @Failure 404 {object} dto.ErrorResponse "Portfolio not found"
// @Failure 500 {object} dto.ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /portfolios/{portfolioId}/summary [get]
func (h *BalanceHandler) GetPortfolioSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse portfolio ID from URL
	portfolioID := chi.URLParam(r, "portfolioId")
	if portfolioID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_PORTFOLIO_ID", "Portfolio ID is required")
		return
	}

	// Log the request
	h.logger.Info("GET /api/v1/portfolios/{portfolioId}/summary",
		zap.String("portfolioId", portfolioID),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Get portfolio summary from service
	summary, err := h.balanceService.GetPortfolioSummary(ctx, portfolioID)
	if err != nil {
		// Check if portfolio not found
		if strings.Contains(err.Error(), "not found") {
			h.logger.Warn("Portfolio not found", zap.String("portfolioId", portfolioID))
			h.writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Portfolio not found")
			return
		}
		h.logger.Error("Failed to get portfolio summary", zap.Error(err), zap.String("portfolioId", portfolioID))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve portfolio summary")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(summary); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully retrieved portfolio summary", zap.String("portfolioId", portfolioID))
}

// parseBalanceFilter parses query parameters into BalanceFilter
func (h *BalanceHandler) parseBalanceFilter(r *http.Request) (*dto.BalanceFilter, error) {
	filter := &dto.BalanceFilter{}

	// Portfolio ID
	if portfolioID := r.URL.Query().Get("portfolio_id"); portfolioID != "" {
		filter.PortfolioID = &portfolioID
	}

	// Security ID
	if securityID := r.URL.Query().Get("security_id"); securityID != "" {
		filter.SecurityID = &securityID
	}

	// Cash only filter
	if cashOnlyStr := r.URL.Query().Get("cash_only"); cashOnlyStr != "" {
		if cashOnly, err := strconv.ParseBool(cashOnlyStr); err == nil {
			filter.CashOnly = &cashOnly
		}
	}

	// Zero balances filter
	if zeroBalancesOnlyStr := r.URL.Query().Get("zero_balances_only"); zeroBalancesOnlyStr != "" {
		if zeroBalancesOnly, err := strconv.ParseBool(zeroBalancesOnlyStr); err == nil {
			filter.ZeroBalancesOnly = &zeroBalancesOnly
		}
	}

	if nonZeroBalancesOnlyStr := r.URL.Query().Get("non_zero_balances_only"); nonZeroBalancesOnlyStr != "" {
		if nonZeroBalancesOnly, err := strconv.ParseBool(nonZeroBalancesOnlyStr); err == nil {
			filter.NonZeroBalancesOnly = &nonZeroBalancesOnly
		}
	}

	// Date Range
	if lastUpdatedFrom := r.URL.Query().Get("last_updated_from"); lastUpdatedFrom != "" {
		if parsedDate, err := time.Parse("2006-01-02", lastUpdatedFrom); err == nil {
			filter.LastUpdatedFrom = &parsedDate
		}
	}
	if lastUpdatedTo := r.URL.Query().Get("last_updated_to"); lastUpdatedTo != "" {
		if parsedDate, err := time.Parse("2006-01-02", lastUpdatedTo); err == nil {
			filter.LastUpdatedTo = &parsedDate
		}
	}

	// Pagination
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Pagination.Offset = offset
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 1000 {
			filter.Pagination.Limit = limit
		}
	}

	// Default pagination values
	if filter.Pagination.Limit == 0 {
		filter.Pagination.Limit = 50 // Default page size
	}

	// Sort fields
	if sortBy := r.URL.Query().Get("sortby"); sortBy != "" {
		// Parse comma-separated sort fields
		fields := strings.Split(sortBy, ",")

		validSortFields := map[string]bool{
			"portfolio_id":   true,
			"security_id":    true,
			"last_updated":   true,
			"quantity_long":  true,
			"quantity_short": true,
		}

		for _, field := range fields {
			field = strings.TrimSpace(field)
			if validSortFields[field] {
				filter.SortBy = append(filter.SortBy, dto.SortRequest{
					Field:     field,
					Direction: "asc", // Default direction
				})
			}
		}
	}

	// Default sort if none specified
	if len(filter.SortBy) == 0 {
		filter.SortBy = []dto.SortRequest{
			{Field: "portfolio_id", Direction: "asc"},
			{Field: "security_id", Direction: "asc"},
		}
	}

	return filter, nil
}

// writeErrorResponse writes a standardized error response
func (h *BalanceHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	errorResp := dto.ErrorResponse{
		Error: dto.ErrorDetail{
			Code:      errorCode,
			Message:   message,
			Timestamp: time.Now(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		h.logger.Error("Failed to write error response", zap.Error(err))
	}
}
