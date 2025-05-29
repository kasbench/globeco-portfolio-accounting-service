package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// BalanceDTO represents a balance response DTO
type BalanceDTO struct {
	ID            int64           `json:"id"`
	PortfolioID   string          `json:"portfolioId"`
	SecurityID    *string         `json:"securityId,omitempty"`
	QuantityLong  decimal.Decimal `json:"quantityLong"`
	QuantityShort decimal.Decimal `json:"quantityShort"`
	LastUpdated   string          `json:"lastUpdated"`
	Version       int             `json:"version"`
}

// BalanceListResponse represents a paginated list of balances
type BalanceListResponse struct {
	Balances   []BalanceDTO       `json:"balances"`
	Pagination PaginationResponse `json:"pagination"`
}

// BalanceStatsDTO represents balance statistics
type BalanceStatsDTO struct {
	TotalBalances    int64     `json:"totalBalances"`
	PortfolioCount   int64     `json:"portfolioCount"`
	SecurityCount    int64     `json:"securityCount"`
	CashBalanceCount int64     `json:"cashBalanceCount"`
	ZeroBalanceCount int64     `json:"zeroBalanceCount"`
	LastUpdated      time.Time `json:"lastUpdated"`
}

// PortfolioSummaryDTO represents a summary of portfolio balances
type PortfolioSummaryDTO struct {
	PortfolioID   string                `json:"portfolioId"`
	CashBalance   decimal.Decimal       `json:"cashBalance"`
	SecurityCount int                   `json:"securityCount"`
	LastUpdated   time.Time             `json:"lastUpdated"`
	Securities    []SecurityPositionDTO `json:"securities"`
}

// SecurityPositionDTO represents a security position within a portfolio
type SecurityPositionDTO struct {
	SecurityID    string          `json:"securityId"`
	QuantityLong  decimal.Decimal `json:"quantityLong"`
	QuantityShort decimal.Decimal `json:"quantityShort"`
	NetQuantity   decimal.Decimal `json:"netQuantity"`
	LastUpdated   time.Time       `json:"lastUpdated"`
}

// BalanceUpdateRequest represents a request to update balance quantities
type BalanceUpdateRequest struct {
	QuantityLong  *decimal.Decimal `json:"quantityLong,omitempty" validate:"omitempty"`
	QuantityShort *decimal.Decimal `json:"quantityShort,omitempty" validate:"omitempty"`
	Version       int              `json:"version" validate:"required,min=1"`
}

// BalanceUpdateResponse represents a response for balance update operations
type BalanceUpdateResponse struct {
	Balance       BalanceDTO `json:"balance"`
	Updated       bool       `json:"updated"`
	PreviousValue BalanceDTO `json:"previousValue"`
}

// BulkBalanceUpdateRequest represents a request for bulk balance updates
type BulkBalanceUpdateRequest struct {
	Updates []BalanceUpdateItem `json:"updates" validate:"required,min=1,max=1000"`
}

// BalanceUpdateItem represents a single balance update item
type BalanceUpdateItem struct {
	BalanceID     int64            `json:"balanceId" validate:"required"`
	QuantityLong  *decimal.Decimal `json:"quantityLong,omitempty"`
	QuantityShort *decimal.Decimal `json:"quantityShort,omitempty"`
	Version       int              `json:"version" validate:"required,min=1"`
}

// BulkBalanceUpdateResponse represents a response for bulk balance updates
type BulkBalanceUpdateResponse struct {
	Successful []BalanceUpdateResponse `json:"successful"`
	Failed     []BalanceUpdateError    `json:"failed"`
	Summary    BulkUpdateSummaryDTO    `json:"summary"`
}

// BalanceUpdateError represents a failed balance update
type BalanceUpdateError struct {
	BalanceID int64             `json:"balanceId"`
	Errors    []ValidationError `json:"errors"`
}

// BulkUpdateSummaryDTO represents summary information for bulk updates
type BulkUpdateSummaryDTO struct {
	TotalRequested int     `json:"totalRequested"`
	Successful     int     `json:"successful"`
	Failed         int     `json:"failed"`
	SuccessRate    float64 `json:"successRate"`
}

// BalanceHistoryDTO represents historical balance information
type BalanceHistoryDTO struct {
	BalanceID     int64           `json:"balanceId"`
	PortfolioID   string          `json:"portfolioId"`
	SecurityID    *string         `json:"securityId,omitempty"`
	QuantityLong  decimal.Decimal `json:"quantityLong"`
	QuantityShort decimal.Decimal `json:"quantityShort"`
	Timestamp     time.Time       `json:"timestamp"`
	ChangeType    string          `json:"changeType"`
	TransactionID *int64          `json:"transactionId,omitempty"`
}

// BalanceHistoryResponse represents a list of balance history entries
type BalanceHistoryResponse struct {
	History    []BalanceHistoryDTO `json:"history"`
	Pagination PaginationResponse  `json:"pagination"`
}
