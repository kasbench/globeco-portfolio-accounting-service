package repositories

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Transaction represents a portfolio transaction entity
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
	PortfolioID     *string    `json:"portfolio_id,omitempty"`
	SecurityID      *string    `json:"security_id,omitempty"`
	TransactionDate *time.Time `json:"transaction_date,omitempty"`
	TransactionType *string    `json:"transaction_type,omitempty"`
	Status          *string    `json:"status,omitempty"`
	Limit           int        `json:"limit,omitempty"`
	Offset          int        `json:"offset,omitempty"`
	SortBy          []string   `json:"sort_by,omitempty"`
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
