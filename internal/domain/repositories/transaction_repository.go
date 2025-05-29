package repositories

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Transaction represents a portfolio transaction entity for repository operations
type Transaction struct {
	ID                   int64           `json:"id" db:"id"`
	PortfolioID          string          `json:"portfolio_id" db:"portfolio_id"`
	SecurityID           *string         `json:"security_id" db:"security_id"`
	SourceID             string          `json:"source_id" db:"source_id"`
	Status               string          `json:"status" db:"status"`
	TransactionType      string          `json:"transaction_type" db:"transaction_type"`
	Quantity             decimal.Decimal `json:"quantity" db:"quantity"`
	Price                decimal.Decimal `json:"price" db:"price"`
	TransactionDate      time.Time       `json:"transaction_date" db:"transaction_date"`
	ReprocessingAttempts int             `json:"reprocessing_attempts" db:"reprocessing_attempts"`
	Version              int             `json:"version" db:"version"`
	CreatedAt            time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at" db:"updated_at"`
	ErrorMessage         *string         `json:"error_message,omitempty"`
}

// TransactionFilter holds filtering options for transaction queries
type TransactionFilter struct {
	// ID filters
	ID          *int64  `json:"id,omitempty"`
	PortfolioID *string `json:"portfolio_id,omitempty"`
	SecurityID  *string `json:"security_id,omitempty"`
	SourceID    *string `json:"source_id,omitempty"`

	// Status and type filters
	Status          *string `json:"status,omitempty"`
	TransactionType *string `json:"transaction_type,omitempty"`

	// Date filters
	TransactionDate     *time.Time `json:"transaction_date,omitempty"`
	TransactionDateFrom *time.Time `json:"transaction_date_from,omitempty"`
	TransactionDateTo   *time.Time `json:"transaction_date_to,omitempty"`

	// Amount filters
	QuantityMin *decimal.Decimal `json:"quantity_min,omitempty"`
	QuantityMax *decimal.Decimal `json:"quantity_max,omitempty"`
	PriceMin    *decimal.Decimal `json:"price_min,omitempty"`
	PriceMax    *decimal.Decimal `json:"price_max,omitempty"`

	// Collections for IN queries
	IDs              []int64  `json:"ids,omitempty"`
	PortfolioIDs     []string `json:"portfolio_ids,omitempty"`
	SecurityIDs      []string `json:"security_ids,omitempty"`
	Statuses         []string `json:"statuses,omitempty"`
	TransactionTypes []string `json:"transaction_types,omitempty"`

	// Pagination and sorting
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
	SortFields []SortField `json:"sort_fields,omitempty"`
	SortBy     []string    `json:"sort_by,omitempty"` // Legacy support for simple sorting
}

// TransactionRepository defines the contract for transaction data access
type TransactionRepository interface {
	// Create operations
	Create(ctx context.Context, transaction *Transaction) error
	CreateBatch(ctx context.Context, transactions []*Transaction) error

	// Read operations
	GetByID(ctx context.Context, id int64) (*Transaction, error)
	GetBySourceID(ctx context.Context, sourceID string) (*Transaction, error)
	List(ctx context.Context, filter TransactionFilter) ([]*Transaction, error)
	Count(ctx context.Context, filter TransactionFilter) (int64, error)

	// Update operations
	Update(ctx context.Context, transaction *Transaction) error
	UpdateStatus(ctx context.Context, id int64, status string, errorMessage *string, version int) error
	IncrementReprocessingAttempts(ctx context.Context, id int64, version int) error

	// Query operations for processing
	GetNewTransactions(ctx context.Context, limit int) ([]*Transaction, error)
	GetTransactionsByPortfolio(ctx context.Context, portfolioID string, limit int, offset int) ([]*Transaction, error)
	GetTransactionsByStatus(ctx context.Context, status string, limit int, offset int) ([]*Transaction, error)

	// Batch operations
	UpdateTransactionsStatus(ctx context.Context, ids []int64, status string, errorMessage *string) error

	// Statistics
	GetTransactionStats(ctx context.Context) (*TransactionStats, error)
}

// TransactionStats holds transaction statistics
type TransactionStats struct {
	TotalCount         int64            `json:"total_count"`
	StatusCounts       map[string]int64 `json:"status_counts"`
	TypeCounts         map[string]int64 `json:"type_counts"`
	RecentCount24h     int64            `json:"recent_count_24h"`
	AverageProcessTime *time.Duration   `json:"average_process_time,omitempty"`
}
