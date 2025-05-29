package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionPostDTO represents the request DTO for creating transactions
type TransactionPostDTO struct {
	PortfolioID     string          `json:"portfolioId" validate:"required,len=24"`
	SecurityID      *string         `json:"securityId,omitempty" validate:"omitempty,len=24"`
	SourceID        string          `json:"sourceId" validate:"required,max=50"`
	TransactionType string          `json:"transactionType" validate:"required,oneof=BUY SELL SHORT COVER DEP WD IN OUT"`
	Quantity        decimal.Decimal `json:"quantity" validate:"required"`
	Price           decimal.Decimal `json:"price" validate:"required,gt=0"`
	TransactionDate string          `json:"transactionDate" validate:"required"`
}

// TransactionResponseDTO represents the response DTO for transactions
type TransactionResponseDTO struct {
	ID                   int64           `json:"id"`
	PortfolioID          string          `json:"portfolioId"`
	SecurityID           *string         `json:"securityId,omitempty"`
	SourceID             string          `json:"sourceId"`
	Status               string          `json:"status"`
	TransactionType      string          `json:"transactionType"`
	Quantity             decimal.Decimal `json:"quantity"`
	Price                decimal.Decimal `json:"price"`
	TransactionDate      string          `json:"transactionDate"`
	ReprocessingAttempts int             `json:"reprocessingAttempts"`
	Version              int             `json:"version"`
	ErrorMessage         *string         `json:"errorMessage,omitempty"`
}

// TransactionListResponse represents a paginated list of transactions
type TransactionListResponse struct {
	Transactions []TransactionResponseDTO `json:"transactions"`
	Pagination   PaginationResponse       `json:"pagination"`
}

// TransactionStatsDTO represents transaction statistics
type TransactionStatsDTO struct {
	TotalCount     int64            `json:"totalCount"`
	StatusCounts   map[string]int64 `json:"statusCounts"`
	TypeCounts     map[string]int64 `json:"typeCounts"`
	PortfolioCount int64            `json:"portfolioCount"`
	DateRange      *DateRangeDTO    `json:"dateRange,omitempty"`
}

// DateRangeDTO represents a date range
type DateRangeDTO struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

// TransactionBatchResponse represents a response for batch transaction operations
type TransactionBatchResponse struct {
	Successful []TransactionResponseDTO `json:"successful"`
	Failed     []TransactionErrorDTO    `json:"failed"`
	Summary    BatchSummaryDTO          `json:"summary"`
}

// TransactionErrorDTO represents a failed transaction in batch operations
type TransactionErrorDTO struct {
	Transaction TransactionPostDTO `json:"transaction"`
	Errors      []ValidationError  `json:"errors"`
}

// BatchSummaryDTO represents summary information for batch operations
type BatchSummaryDTO struct {
	TotalRequested int     `json:"totalRequested"`
	Successful     int     `json:"successful"`
	Failed         int     `json:"failed"`
	SuccessRate    float64 `json:"successRate"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// TransactionProcessingResult represents the result of transaction processing
type TransactionProcessingResult struct {
	TransactionID  int64     `json:"transactionId"`
	Status         string    `json:"status"`
	ProcessedAt    time.Time `json:"processedAt"`
	BalanceUpdated bool      `json:"balanceUpdated"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
}

// FileProcessingStatus represents the status of file processing
type FileProcessingStatus struct {
	Filename         string     `json:"filename"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"startedAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	TotalRecords     int        `json:"totalRecords"`
	ProcessedRecords int        `json:"processedRecords"`
	FailedRecords    int        `json:"failedRecords"`
	ErrorFilename    *string    `json:"errorFilename,omitempty"`
}
