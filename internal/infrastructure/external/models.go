package external

import (
	"time"
)

// Portfolio service models

// PortfolioResponse represents a portfolio response from the portfolio service
type PortfolioResponse struct {
	PortfolioID string     `json:"portfolioId"`
	Name        string     `json:"name"`
	DateCreated *time.Time `json:"dateCreated,omitempty"`
	Version     int        `json:"version"`
}

// PortfolioListResponse represents a list of portfolios
type PortfolioListResponse []PortfolioResponse

// Security service models

// SecurityTypeNested represents nested security type information
type SecurityTypeNested struct {
	SecurityTypeID string `json:"securityTypeId"`
	Abbreviation   string `json:"abbreviation"`
	Description    string `json:"description"`
}

// SecurityResponse represents a security response from the security service
type SecurityResponse struct {
	SecurityID     string             `json:"securityId"`
	Ticker         string             `json:"ticker"`
	Description    string             `json:"description"`
	SecurityTypeID string             `json:"securityTypeId"`
	Version        int                `json:"version"`
	SecurityType   SecurityTypeNested `json:"securityType"`
}

// SecurityListResponse represents a list of securities
type SecurityListResponse []SecurityResponse

// SecurityTypeResponse represents a security type response
type SecurityTypeResponse struct {
	SecurityTypeID string `json:"securityTypeId"`
	Abbreviation   string `json:"abbreviation"`
	Description    string `json:"description"`
	Version        int    `json:"version"`
}

// SecurityTypeListResponse represents a list of security types
type SecurityTypeListResponse []SecurityTypeResponse

// Error response models

// ValidationError represents a validation error from external services
type ValidationError struct {
	Location []interface{} `json:"loc"`
	Message  string        `json:"msg"`
	Type     string        `json:"type"`
}

// HTTPValidationError represents HTTP validation error response
type HTTPValidationError struct {
	Detail []ValidationError `json:"detail"`
}

// ServiceError represents a generic service error
type ServiceError struct {
	Service    string `json:"service"`
	Operation  string `json:"operation"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
}

// NewServiceError creates a new service error
func NewServiceError(service, operation string, statusCode int, message string) *ServiceError {
	return &ServiceError{
		Service:    service,
		Operation:  operation,
		StatusCode: statusCode,
		Message:    message,
		Timestamp:  time.Now().Format(time.RFC3339),
	}
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	return e.Message
}

// IsNotFound checks if the error indicates a resource was not found
func (e *ServiceError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsValidationError checks if the error is a validation error
func (e *ServiceError) IsValidationError() bool {
	return e.StatusCode == 422
}

// IsServerError checks if the error is a server error (5xx)
func (e *ServiceError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsClientError checks if the error is a client error (4xx)
func (e *ServiceError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}
