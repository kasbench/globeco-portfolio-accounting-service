package dto

import (
	"time"
)

// PaginationRequest represents pagination parameters for requests
type PaginationRequest struct {
	Limit  int `json:"limit" validate:"min=1,max=1000"`
	Offset int `json:"offset" validate:"min=0"`
}

// PaginationResponse represents pagination information in responses
type PaginationResponse struct {
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	Total      int64 `json:"total"`
	HasMore    bool  `json:"hasMore"`
	Page       int   `json:"page"`
	TotalPages int   `json:"totalPages"`
}

// SortRequest represents sorting parameters
type SortRequest struct {
	Field     string `json:"field" validate:"required"`
	Direction string `json:"direction" validate:"required,oneof=asc desc"`
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains detailed error information
type ErrorDetail struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	TraceID   string                 `json:"traceId,omitempty"`
}

// SuccessResponse represents a standardized success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	Environment string                 `json:"environment"`
	Checks      map[string]interface{} `json:"checks"`
}

// MetricsResponse represents metrics information
type MetricsResponse struct {
	Service   string                 `json:"service"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
}

// NewPaginationResponse creates a new pagination response
func NewPaginationResponse(limit, offset int, total int64) PaginationResponse {
	page := (offset / limit) + 1
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	hasMore := int64(offset+limit) < total

	return PaginationResponse{
		Limit:      limit,
		Offset:     offset,
		Total:      total,
		HasMore:    hasMore,
		Page:       page,
		TotalPages: totalPages,
	}
}

// NewErrorResponse creates a new error response
func NewErrorResponse(code, message string, details map[string]interface{}) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			Details:   details,
			Timestamp: time.Now(),
		},
	}
}

// NewSuccessResponse creates a new success response
func NewSuccessResponse(message string, data interface{}) SuccessResponse {
	return SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}
