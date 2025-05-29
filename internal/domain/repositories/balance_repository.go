package repositories

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// BalanceFilter holds filtering options for balance queries
type BalanceFilter struct {
	PortfolioID *string  `json:"portfolio_id,omitempty"`
	SecurityID  *string  `json:"security_id,omitempty"`
	CashOnly    bool     `json:"cash_only,omitempty"` // filter for cash balances (security_id IS NULL)
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	SortBy      []string `json:"sort_by,omitempty"`
}

// Balance represents a portfolio balance entity
type Balance struct {
	ID            int64           `json:"id" db:"id"`
	PortfolioID   string          `json:"portfolio_id" db:"portfolio_id"`
	SecurityID    *string         `json:"security_id" db:"security_id"`
	QuantityLong  decimal.Decimal `json:"quantity_long" db:"quantity_long"`
	QuantityShort decimal.Decimal `json:"quantity_short" db:"quantity_short"`
	LastUpdated   time.Time       `json:"last_updated" db:"last_updated"`
	Version       int             `json:"version" db:"version"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// BalanceRepository defines the contract for balance data access
type BalanceRepository interface {
	// Create operations
	Create(ctx context.Context, balance *Balance) error
	CreateOrUpdate(ctx context.Context, balance *Balance) error

	// Read operations
	GetByID(ctx context.Context, id int64) (*Balance, error)
	GetByPortfolioAndSecurity(ctx context.Context, portfolioID string, securityID *string) (*Balance, error)
	List(ctx context.Context, filter BalanceFilter) ([]*Balance, error)
	Count(ctx context.Context, filter BalanceFilter) (int64, error)

	// Update operations
	Update(ctx context.Context, balance *Balance) error
	UpdateQuantities(ctx context.Context, id int64, quantityLong, quantityShort decimal.Decimal, version int) error

	// Batch operations
	UpdateMultipleBalances(ctx context.Context, updates []BalanceUpdate) error

	// Query operations
	GetBalancesByPortfolio(ctx context.Context, portfolioID string) ([]*Balance, error)
	GetCashBalance(ctx context.Context, portfolioID string) (*Balance, error)
	GetZeroBalances(ctx context.Context, limit int) ([]*Balance, error)

	// Statistics
	GetBalanceStats(ctx context.Context) (*BalanceStats, error)
	GetPortfolioSummary(ctx context.Context, portfolioID string) (*PortfolioSummary, error)
}

// BalanceUpdate represents a balance update operation
type BalanceUpdate struct {
	ID            int64           `json:"id"`
	QuantityLong  decimal.Decimal `json:"quantity_long"`
	QuantityShort decimal.Decimal `json:"quantity_short"`
	Version       int             `json:"version"`
}

// BalanceStats holds balance statistics
type BalanceStats struct {
	TotalBalances    int64 `json:"total_balances"`
	TotalPortfolios  int64 `json:"total_portfolios"`
	TotalSecurities  int64 `json:"total_securities"`
	CashBalances     int64 `json:"cash_balances"`
	ZeroBalances     int64 `json:"zero_balances"`
	PositiveBalances int64 `json:"positive_balances"`
	NegativeBalances int64 `json:"negative_balances"`
}

// PortfolioSummary holds a summary of a portfolio's balances
type PortfolioSummary struct {
	PortfolioID    string          `json:"portfolio_id"`
	TotalPositions int             `json:"total_positions"`
	CashBalance    decimal.Decimal `json:"cash_balance"`
	LongPositions  int             `json:"long_positions"`
	ShortPositions int             `json:"short_positions"`
	LastUpdated    time.Time       `json:"last_updated"`
}
