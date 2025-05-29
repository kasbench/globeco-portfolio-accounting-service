package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionFilter represents filters for transaction queries
type TransactionFilter struct {
	// ID filters
	IDs []int64 `json:"ids,omitempty" validate:"omitempty,max=100"`

	// Portfolio and Security filters
	PortfolioID  *string  `json:"portfolioId,omitempty" validate:"omitempty,len=24"`
	PortfolioIDs []string `json:"portfolioIds,omitempty" validate:"omitempty,max=50,dive,len=24"`
	SecurityID   *string  `json:"securityId,omitempty" validate:"omitempty,len=24"`
	SecurityIDs  []string `json:"securityIds,omitempty" validate:"omitempty,max=50,dive,len=24"`
	CashOnly     *bool    `json:"cashOnly,omitempty"`

	// Status and Type filters
	Status           *string  `json:"status,omitempty" validate:"omitempty,oneof=NEW PROC FATAL ERROR"`
	Statuses         []string `json:"statuses,omitempty" validate:"omitempty,max=10,dive,oneof=NEW PROC FATAL ERROR"`
	TransactionType  *string  `json:"transactionType,omitempty" validate:"omitempty,oneof=BUY SELL SHORT COVER DEP WD IN OUT"`
	TransactionTypes []string `json:"transactionTypes,omitempty" validate:"omitempty,max=10,dive,oneof=BUY SELL SHORT COVER DEP WD IN OUT"`

	// Date filters
	TransactionDate     *time.Time `json:"transactionDate,omitempty"`
	TransactionDateFrom *time.Time `json:"transactionDateFrom,omitempty"`
	TransactionDateTo   *time.Time `json:"transactionDateTo,omitempty"`

	// Amount filters
	MinQuantity *decimal.Decimal `json:"minQuantity,omitempty"`
	MaxQuantity *decimal.Decimal `json:"maxQuantity,omitempty"`
	MinPrice    *decimal.Decimal `json:"minPrice,omitempty"`
	MaxPrice    *decimal.Decimal `json:"maxPrice,omitempty"`
	MinAmount   *decimal.Decimal `json:"minAmount,omitempty"` // quantity * price
	MaxAmount   *decimal.Decimal `json:"maxAmount,omitempty"` // quantity * price

	// Source filters
	SourceID  *string  `json:"sourceId,omitempty" validate:"omitempty,max=50"`
	SourceIDs []string `json:"sourceIds,omitempty" validate:"omitempty,max=100,dive,max=50"`

	// Error filters
	HasErrors               *bool `json:"hasErrors,omitempty"`
	ReprocessingAttempts    *int  `json:"reprocessingAttempts,omitempty" validate:"omitempty,min=0"`
	MinReprocessingAttempts *int  `json:"minReprocessingAttempts,omitempty" validate:"omitempty,min=0"`
	MaxReprocessingAttempts *int  `json:"maxReprocessingAttempts,omitempty" validate:"omitempty,min=0"`

	// Pagination and Sorting
	Pagination PaginationRequest `json:"pagination"`
	SortBy     []SortRequest     `json:"sortBy,omitempty" validate:"omitempty,max=5"`

	// Advanced filters
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	UpdatedFrom *time.Time `json:"updatedFrom,omitempty"`
	UpdatedTo   *time.Time `json:"updatedTo,omitempty"`
}

// BalanceFilter represents filters for balance queries
type BalanceFilter struct {
	// ID filters
	IDs []int64 `json:"ids,omitempty" validate:"omitempty,max=100"`

	// Portfolio and Security filters
	PortfolioID  *string  `json:"portfolioId,omitempty" validate:"omitempty,len=24"`
	PortfolioIDs []string `json:"portfolioIds,omitempty" validate:"omitempty,max=50,dive,len=24"`
	SecurityID   *string  `json:"securityId,omitempty" validate:"omitempty,len=24"`
	SecurityIDs  []string `json:"securityIds,omitempty" validate:"omitempty,max=50,dive,len=24"`
	CashOnly     *bool    `json:"cashOnly,omitempty"`

	// Quantity filters
	MinQuantityLong  *decimal.Decimal `json:"minQuantityLong,omitempty"`
	MaxQuantityLong  *decimal.Decimal `json:"maxQuantityLong,omitempty"`
	MinQuantityShort *decimal.Decimal `json:"minQuantityShort,omitempty"`
	MaxQuantityShort *decimal.Decimal `json:"maxQuantityShort,omitempty"`
	MinNetQuantity   *decimal.Decimal `json:"minNetQuantity,omitempty"`
	MaxNetQuantity   *decimal.Decimal `json:"maxNetQuantity,omitempty"`

	// Zero balance filters
	ZeroBalancesOnly    *bool `json:"zeroBalancesOnly,omitempty"`
	NonZeroBalancesOnly *bool `json:"nonZeroBalancesOnly,omitempty"`

	// Date filters
	LastUpdatedFrom *time.Time `json:"lastUpdatedFrom,omitempty"`
	LastUpdatedTo   *time.Time `json:"lastUpdatedTo,omitempty"`

	// Pagination and Sorting
	Pagination PaginationRequest `json:"pagination"`
	SortBy     []SortRequest     `json:"sortBy,omitempty" validate:"omitempty,max=5"`

	// Advanced filters
	HasLongPositions  *bool `json:"hasLongPositions,omitempty"`
	HasShortPositions *bool `json:"hasShortPositions,omitempty"`
}

// PortfolioSummaryFilter represents filters for portfolio summary queries
type PortfolioSummaryFilter struct {
	PortfolioIDs     []string          `json:"portfolioIds,omitempty" validate:"omitempty,max=50,dive,len=24"`
	MinCashBalance   *decimal.Decimal  `json:"minCashBalance,omitempty"`
	MaxCashBalance   *decimal.Decimal  `json:"maxCashBalance,omitempty"`
	MinSecurityCount *int              `json:"minSecurityCount,omitempty" validate:"omitempty,min=0"`
	MaxSecurityCount *int              `json:"maxSecurityCount,omitempty" validate:"omitempty,min=0"`
	LastUpdatedFrom  *time.Time        `json:"lastUpdatedFrom,omitempty"`
	LastUpdatedTo    *time.Time        `json:"lastUpdatedTo,omitempty"`
	Pagination       PaginationRequest `json:"pagination"`
	SortBy           []SortRequest     `json:"sortBy,omitempty" validate:"omitempty,max=3"`
}

// FileProcessingFilter represents filters for file processing status queries
type FileProcessingFilter struct {
	Filename         *string           `json:"filename,omitempty"`
	Status           *string           `json:"status,omitempty" validate:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED"`
	Statuses         []string          `json:"statuses,omitempty" validate:"omitempty,max=10,dive,oneof=PENDING PROCESSING COMPLETED FAILED"`
	StartedFrom      *time.Time        `json:"startedFrom,omitempty"`
	StartedTo        *time.Time        `json:"startedTo,omitempty"`
	CompletedFrom    *time.Time        `json:"completedFrom,omitempty"`
	CompletedTo      *time.Time        `json:"completedTo,omitempty"`
	MinTotalRecords  *int              `json:"minTotalRecords,omitempty" validate:"omitempty,min=0"`
	MaxTotalRecords  *int              `json:"maxTotalRecords,omitempty" validate:"omitempty,min=0"`
	MinFailedRecords *int              `json:"minFailedRecords,omitempty" validate:"omitempty,min=0"`
	MaxFailedRecords *int              `json:"maxFailedRecords,omitempty" validate:"omitempty,min=0"`
	HasErrors        *bool             `json:"hasErrors,omitempty"`
	Pagination       PaginationRequest `json:"pagination"`
	SortBy           []SortRequest     `json:"sortBy,omitempty" validate:"omitempty,max=3"`
}

// SearchRequest represents a general search request
type SearchRequest struct {
	Query      string                 `json:"query" validate:"required,min=1,max=100"`
	SearchType string                 `json:"searchType" validate:"required,oneof=transactions balances portfolios"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	Pagination PaginationRequest      `json:"pagination"`
	SortBy     []SortRequest          `json:"sortBy,omitempty" validate:"omitempty,max=3"`
}

// SearchResponse represents a general search response
type SearchResponse struct {
	Query      string                 `json:"query"`
	SearchType string                 `json:"searchType"`
	Results    []interface{}          `json:"results"`
	Pagination PaginationResponse     `json:"pagination"`
	Facets     map[string]interface{} `json:"facets,omitempty"`
}

// DateRangeFilter represents a common date range filter
type DateRangeFilter struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

// AmountRangeFilter represents a common amount range filter
type AmountRangeFilter struct {
	Min *decimal.Decimal `json:"min,omitempty"`
	Max *decimal.Decimal `json:"max,omitempty"`
}

// Validate methods for filters

// IsValid checks if the transaction filter is valid
func (tf *TransactionFilter) IsValid() bool {
	// Check date range validity
	if tf.TransactionDateFrom != nil && tf.TransactionDateTo != nil {
		if tf.TransactionDateFrom.After(*tf.TransactionDateTo) {
			return false
		}
	}

	// Check amount range validity
	if tf.MinAmount != nil && tf.MaxAmount != nil {
		if tf.MinAmount.GreaterThan(*tf.MaxAmount) {
			return false
		}
	}

	// Check quantity range validity
	if tf.MinQuantity != nil && tf.MaxQuantity != nil {
		if tf.MinQuantity.GreaterThan(*tf.MaxQuantity) {
			return false
		}
	}

	return true
}

// IsValid checks if the balance filter is valid
func (bf *BalanceFilter) IsValid() bool {
	// Check quantity range validity
	if bf.MinQuantityLong != nil && bf.MaxQuantityLong != nil {
		if bf.MinQuantityLong.GreaterThan(*bf.MaxQuantityLong) {
			return false
		}
	}

	if bf.MinQuantityShort != nil && bf.MaxQuantityShort != nil {
		if bf.MinQuantityShort.GreaterThan(*bf.MaxQuantityShort) {
			return false
		}
	}

	// Check date range validity
	if bf.LastUpdatedFrom != nil && bf.LastUpdatedTo != nil {
		if bf.LastUpdatedFrom.After(*bf.LastUpdatedTo) {
			return false
		}
	}

	return true
}
