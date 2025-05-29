package repositories

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// BalanceFilter holds filtering options for balance queries
type BalanceFilter struct {
	// ID filters
	ID          *int64  `json:"id,omitempty"`
	PortfolioID *string `json:"portfolio_id,omitempty"`
	SecurityID  *string `json:"security_id,omitempty"`

	// Cash vs security filters
	CashOnly       bool  `json:"cash_only,omitempty"`       // filter for cash balances (security_id IS NULL)
	SecuritiesOnly bool  `json:"securities_only,omitempty"` // filter for security balances (security_id IS NOT NULL)
	IncludeCash    *bool `json:"include_cash,omitempty"`    // include cash balances
	OnlyCash       *bool `json:"only_cash,omitempty"`       // only cash balances

	// Date filters
	LastUpdatedFrom *time.Time `json:"last_updated_from,omitempty"`
	LastUpdatedTo   *time.Time `json:"last_updated_to,omitempty"`

	// Quantity filters
	QuantityLongMin  *decimal.Decimal `json:"quantity_long_min,omitempty"`
	QuantityLongMax  *decimal.Decimal `json:"quantity_long_max,omitempty"`
	QuantityShortMin *decimal.Decimal `json:"quantity_short_min,omitempty"`
	QuantityShortMax *decimal.Decimal `json:"quantity_short_max,omitempty"`

	// Zero balance filters
	ExcludeZeroBalances bool `json:"exclude_zero_balances,omitempty"`
	OnlyZeroBalances    bool `json:"only_zero_balances,omitempty"`

	// Collections for IN queries
	IDs          []int64  `json:"ids,omitempty"`
	PortfolioIDs []string `json:"portfolio_ids,omitempty"`
	SecurityIDs  []string `json:"security_ids,omitempty"`

	// Pagination and sorting
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
	SortFields []SortField `json:"sort_fields,omitempty"`
	SortBy     []string    `json:"sort_by,omitempty"` // Legacy support for simple sorting
}

// Balance represents a portfolio balance entity for repository operations
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
