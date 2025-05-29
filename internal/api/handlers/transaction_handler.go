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

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	transactionService services.TransactionService
	logger             logger.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(transactionService services.TransactionService, logger logger.Logger) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		logger:             logger,
	}
}

// GetTransactions handles GET /api/v1/transactions
func (h *TransactionHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filter, err := h.parseTransactionFilter(r)
	if err != nil {
		h.logger.Error("Failed to parse transaction filter", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FILTER", err.Error())
		return
	}

	// Log the request
	h.logger.Info("GET /api/v1/transactions",
		zap.Any("filter", filter),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Get transactions from service
	result, err := h.transactionService.GetTransactions(ctx, *filter)
	if err != nil {
		h.logger.Error("Failed to get transactions", zap.Error(err), zap.Any("filter", filter))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve transactions")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully retrieved transactions",
		zap.Int("count", len(result.Transactions)),
		zap.Int64("total", result.Pagination.Total),
		zap.Int("page", result.Pagination.Page),
		zap.Int("limit", result.Pagination.Limit))
}

// GetTransactionByID handles GET /api/v1/transaction/{id}
func (h *TransactionHandler) GetTransactionByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse transaction ID from URL
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_ID", "Transaction ID is required")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid transaction ID", zap.String("id", idStr), zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_ID", "Transaction ID must be a valid integer")
		return
	}

	// Log the request
	h.logger.Info("GET /api/v1/transaction/{id}",
		zap.Int64("id", id),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Get transaction from service
	transaction, err := h.transactionService.GetTransaction(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.logger.Warn("Transaction not found", zap.Int64("id", id))
			h.writeErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Transaction not found")
			return
		}
		h.logger.Error("Failed to get transaction", zap.Error(err), zap.Int64("id", id))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve transaction")
		return
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(transaction); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully retrieved transaction", zap.Int64("id", id))
}

// CreateTransactions handles POST /api/v1/transactions
func (h *TransactionHandler) CreateTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Log the request
	h.logger.Info("POST /api/v1/transactions",
		zap.String("content_type", r.Header.Get("Content-Type")),
		zap.Int64("content_length", r.ContentLength),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("remote_addr", r.RemoteAddr))

	// Parse request body
	var transactions []dto.TransactionPostDTO
	if err := json.NewDecoder(r.Body).Decode(&transactions); err != nil {
		h.logger.Error("Failed to decode request body", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	// Validate batch size
	if len(transactions) == 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "EMPTY_BATCH", "At least one transaction is required")
		return
	}

	if len(transactions) > 1000 { // Configurable limit
		h.writeErrorResponse(w, http.StatusBadRequest, "BATCH_TOO_LARGE", "Maximum 1000 transactions per batch")
		return
	}

	// Create transactions using service
	result, err := h.transactionService.CreateTransactions(ctx, transactions)
	if err != nil {
		h.logger.Error("Failed to create transactions", zap.Error(err), zap.Int("count", len(transactions)))
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create transactions")
		return
	}

	// Determine HTTP status based on results
	status := http.StatusCreated
	if result.Summary.Failed > 0 {
		status = http.StatusMultiStatus // 207 Multi-Status for partial success
	}

	// Write successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		return
	}

	h.logger.Info("Successfully processed transaction batch",
		zap.Int("input_count", len(transactions)),
		zap.Int("success_count", result.Summary.Successful),
		zap.Int("error_count", result.Summary.Failed),
		zap.Int("status", status))
}

// parseTransactionFilter parses query parameters into TransactionFilter
func (h *TransactionHandler) parseTransactionFilter(r *http.Request) (*dto.TransactionFilter, error) {
	filter := &dto.TransactionFilter{}

	// Portfolio ID
	if portfolioID := r.URL.Query().Get("portfolio_id"); portfolioID != "" {
		filter.PortfolioID = &portfolioID
	}

	// Security ID
	if securityID := r.URL.Query().Get("security_id"); securityID != "" {
		filter.SecurityID = &securityID
	}

	// Transaction Type
	if transactionType := r.URL.Query().Get("transaction_type"); transactionType != "" {
		filter.TransactionType = &transactionType
	}

	// Status
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = &status
	}

	// Transaction Date
	if transactionDate := r.URL.Query().Get("transaction_date"); transactionDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", transactionDate); err == nil {
			filter.TransactionDate = &parsedDate
		}
	}

	// Date Range
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", fromDate); err == nil {
			filter.TransactionDateFrom = &parsedDate
		}
	}
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", toDate); err == nil {
			filter.TransactionDateTo = &parsedDate
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
		var validFields []string

		validSortFields := map[string]bool{
			"portfolio_id":     true,
			"security_id":      true,
			"transaction_date": true,
			"transaction_type": true,
			"status":           true,
			"created_at":       true,
		}

		for _, field := range fields {
			field = strings.TrimSpace(field)
			if validSortFields[field] {
				validFields = append(validFields, field)
			}
		}

		if len(validFields) > 0 {
			filter.SortBy = append(filter.SortBy, dto.SortRequest{
				Field:     validFields[0],
				Direction: "asc", // Default direction
			})
		}
	}

	// Default sort if none specified
	if len(filter.SortBy) == 0 {
		filter.SortBy = []dto.SortRequest{
			{Field: "transaction_date", Direction: "desc"},
			{Field: "id", Direction: "asc"},
		}
	}

	return filter, nil
}

// writeErrorResponse writes a standardized error response
func (h *TransactionHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
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
