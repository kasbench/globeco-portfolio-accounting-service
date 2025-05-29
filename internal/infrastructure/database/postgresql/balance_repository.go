package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// BalanceRepository implements the repositories.BalanceRepository interface for PostgreSQL
type BalanceRepository struct {
	db     *database.DB
	logger logger.Logger
}

// NewBalanceRepository creates a new PostgreSQL balance repository
func NewBalanceRepository(db *database.DB, logger logger.Logger) *BalanceRepository {
	return &BalanceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new balance
func (r *BalanceRepository) Create(ctx context.Context, balance *repositories.Balance) error {
	query := `
		INSERT INTO balances (
			portfolio_id, security_id, quantity_long, quantity_short, version
		) VALUES (
			:portfolio_id, :security_id, :quantity_long, :quantity_short, :version
		) RETURNING id, last_updated, created_at`

	rows, err := r.db.NamedQueryContext(ctx, query, balance)
	if err != nil {
		if isDuplicateKeyError(err) {
			return repositories.NewDuplicateKeyError("balance", "portfolio_security", fmt.Sprintf("%s-%v", balance.PortfolioID, balance.SecurityID))
		}
		return repositories.NewRepositoryError("create", "balance", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&balance.ID, &balance.LastUpdated, &balance.CreatedAt); err != nil {
			return repositories.NewRepositoryError("scan", "balance", err)
		}
	}

	r.logger.Info("Balance created",
		logger.Int64("id", balance.ID),
		logger.String("portfolioId", balance.PortfolioID))

	return nil
}

// CreateOrUpdate creates a new balance or updates existing one
func (r *BalanceRepository) CreateOrUpdate(ctx context.Context, balance *repositories.Balance) error {
	query := `
		INSERT INTO balances (
			portfolio_id, security_id, quantity_long, quantity_short, version
		) VALUES (
			:portfolio_id, :security_id, :quantity_long, :quantity_short, :version
		)
		ON CONFLICT (portfolio_id, security_id)
		DO UPDATE SET
			quantity_long = EXCLUDED.quantity_long,
			quantity_short = EXCLUDED.quantity_short,
			version = balances.version + 1,
			last_updated = CURRENT_TIMESTAMP
		RETURNING id, last_updated, created_at`

	rows, err := r.db.NamedQueryContext(ctx, query, balance)
	if err != nil {
		return repositories.NewRepositoryError("create_or_update", "balance", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&balance.ID, &balance.LastUpdated, &balance.CreatedAt); err != nil {
			return repositories.NewRepositoryError("scan", "balance", err)
		}
	}

	r.logger.Info("Balance created or updated",
		logger.Int64("id", balance.ID),
		logger.String("portfolioId", balance.PortfolioID))

	return nil
}

// GetByID retrieves a balance by ID
func (r *BalanceRepository) GetByID(ctx context.Context, id int64) (*repositories.Balance, error) {
	query := `
		SELECT id, portfolio_id, security_id, quantity_long, quantity_short,
			   last_updated, version, created_at
		FROM balances
		WHERE id = $1`

	var balance repositories.Balance
	err := r.db.GetContext(ctx, &balance, query, id)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.NewNotFoundError("balance", id)
		}
		return nil, repositories.NewRepositoryError("get", "balance", err)
	}

	return &balance, nil
}

// GetByPortfolioAndSecurity retrieves a balance by portfolio and security ID
func (r *BalanceRepository) GetByPortfolioAndSecurity(ctx context.Context, portfolioID string, securityID *string) (*repositories.Balance, error) {
	var query string
	var args []interface{}

	if securityID == nil {
		query = `
			SELECT id, portfolio_id, security_id, quantity_long, quantity_short,
				   last_updated, version, created_at
			FROM balances
			WHERE portfolio_id = $1 AND security_id IS NULL`
		args = []interface{}{portfolioID}
	} else {
		query = `
			SELECT id, portfolio_id, security_id, quantity_long, quantity_short,
				   last_updated, version, created_at
			FROM balances
			WHERE portfolio_id = $1 AND security_id = $2`
		args = []interface{}{portfolioID, *securityID}
	}

	var balance repositories.Balance
	err := r.db.GetContext(ctx, &balance, query, args...)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.NewNotFoundError("balance", fmt.Sprintf("%s-%v", portfolioID, securityID))
		}
		return nil, repositories.NewRepositoryError("get", "balance", err)
	}

	return &balance, nil
}

// List retrieves balances based on filter criteria
func (r *BalanceRepository) List(ctx context.Context, filter repositories.BalanceFilter) ([]*repositories.Balance, error) {
	query, args, err := r.buildListQuery(filter)
	if err != nil {
		return nil, repositories.NewRepositoryError("build_query", "balance", err)
	}

	var balances []*repositories.Balance
	err = r.db.SelectContext(ctx, &balances, query, args...)

	if err != nil {
		return nil, repositories.NewRepositoryError("list", "balance", err)
	}

	return balances, nil
}

// Count counts balances based on filter criteria
func (r *BalanceRepository) Count(ctx context.Context, filter repositories.BalanceFilter) (int64, error) {
	query, args, err := r.buildCountQuery(filter)
	if err != nil {
		return 0, repositories.NewRepositoryError("build_query", "balance", err)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, query, args...)

	if err != nil {
		return 0, repositories.NewRepositoryError("count", "balance", err)
	}

	return count, nil
}

// Update updates an existing balance with optimistic locking
func (r *BalanceRepository) Update(ctx context.Context, balance *repositories.Balance) error {
	query := `
		UPDATE balances SET
			portfolio_id = :portfolio_id,
			security_id = :security_id,
			quantity_long = :quantity_long,
			quantity_short = :quantity_short,
			version = version + 1,
			last_updated = CURRENT_TIMESTAMP
		WHERE id = :id AND version = :version
		RETURNING version, last_updated`

	originalVersion := balance.Version
	balance.Version++ // Optimistic increment

	rows, err := r.db.NamedQueryContext(ctx, query, balance)
	if err != nil {
		balance.Version = originalVersion // Restore on error
		if isDuplicateKeyError(err) {
			return repositories.NewDuplicateKeyError("balance", "portfolio_security", fmt.Sprintf("%s-%v", balance.PortfolioID, balance.SecurityID))
		}
		return repositories.NewRepositoryError("update", "balance", err)
	}
	defer rows.Close()

	if !rows.Next() {
		balance.Version = originalVersion // Restore on error
		return repositories.NewOptimisticLockError("balance", balance.ID, originalVersion, balance.Version)
	}

	if err := rows.Scan(&balance.Version, &balance.LastUpdated); err != nil {
		return repositories.NewRepositoryError("scan", "balance", err)
	}

	r.logger.Info("Balance updated",
		logger.Int64("id", balance.ID),
		logger.Int("version", balance.Version))

	return nil
}

// UpdateQuantities updates the quantities of a balance with optimistic locking
func (r *BalanceRepository) UpdateQuantities(ctx context.Context, id int64, quantityLong, quantityShort decimal.Decimal, version int) error {
	query := `
		UPDATE balances SET
			quantity_long = $1,
			quantity_short = $2,
			version = version + 1,
			last_updated = CURRENT_TIMESTAMP
		WHERE id = $3 AND version = $4`

	result, err := r.db.ExecContext(ctx, query, quantityLong, quantityShort, id, version)
	if err != nil {
		return repositories.NewRepositoryError("update_quantities", "balance", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return repositories.NewRepositoryError("check_rows", "balance", err)
	}

	if rowsAffected == 0 {
		return repositories.NewOptimisticLockError("balance", id, version, version+1)
	}

	r.logger.Info("Balance quantities updated",
		logger.Int64("id", id))

	return nil
}

// UpdateMultipleBalances updates multiple balances in a single transaction
func (r *BalanceRepository) UpdateMultipleBalances(ctx context.Context, updates []repositories.BalanceUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	return r.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			UPDATE balances SET
				quantity_long = $1,
				quantity_short = $2,
				version = version + 1,
				last_updated = CURRENT_TIMESTAMP
			WHERE id = $3 AND version = $4`

		for _, update := range updates {
			result, err := tx.ExecContext(ctx, query, update.QuantityLong, update.QuantityShort, update.ID, update.Version)
			if err != nil {
				return repositories.NewRepositoryError("update_batch", "balance", err)
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return repositories.NewRepositoryError("check_rows", "balance", err)
			}

			if rowsAffected == 0 {
				return repositories.NewOptimisticLockError("balance", update.ID, update.Version, update.Version+1)
			}
		}

		r.logger.Info("Multiple balances updated",
			logger.Int("count", len(updates)))

		return nil
	})
}

// GetBalancesByPortfolio retrieves all balances for a specific portfolio
func (r *BalanceRepository) GetBalancesByPortfolio(ctx context.Context, portfolioID string) ([]*repositories.Balance, error) {
	filter := repositories.BalanceFilter{
		PortfolioID: &portfolioID,
		SortBy:      []string{"security_id NULLS FIRST", "created_at"},
	}

	return r.List(ctx, filter)
}

// GetCashBalance retrieves the cash balance for a portfolio
func (r *BalanceRepository) GetCashBalance(ctx context.Context, portfolioID string) (*repositories.Balance, error) {
	return r.GetByPortfolioAndSecurity(ctx, portfolioID, nil)
}

// GetZeroBalances retrieves balances with zero quantities
func (r *BalanceRepository) GetZeroBalances(ctx context.Context, limit int) ([]*repositories.Balance, error) {
	filter := repositories.BalanceFilter{
		OnlyZeroBalances: true,
		Limit:            limit,
		SortBy:           []string{"last_updated"},
	}

	return r.List(ctx, filter)
}

// GetBalanceStats retrieves balance statistics
func (r *BalanceRepository) GetBalanceStats(ctx context.Context) (*repositories.BalanceStats, error) {
	stats := &repositories.BalanceStats{}

	// Get total balances
	if err := r.db.GetContext(ctx, &stats.TotalBalances, "SELECT COUNT(*) FROM balances"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get total portfolios
	if err := r.db.GetContext(ctx, &stats.TotalPortfolios, "SELECT COUNT(DISTINCT portfolio_id) FROM balances"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get total securities
	if err := r.db.GetContext(ctx, &stats.TotalSecurities, "SELECT COUNT(DISTINCT security_id) FROM balances WHERE security_id IS NOT NULL"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get cash balances count
	if err := r.db.GetContext(ctx, &stats.CashBalances, "SELECT COUNT(*) FROM balances WHERE security_id IS NULL"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get zero balances count
	if err := r.db.GetContext(ctx, &stats.ZeroBalances, "SELECT COUNT(*) FROM balances WHERE quantity_long = 0 AND quantity_short = 0"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get positive balances count
	if err := r.db.GetContext(ctx, &stats.PositiveBalances, "SELECT COUNT(*) FROM balances WHERE quantity_long > 0 OR quantity_short > 0"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	// Get negative balances count
	if err := r.db.GetContext(ctx, &stats.NegativeBalances, "SELECT COUNT(*) FROM balances WHERE quantity_long < 0 OR quantity_short < 0"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "balance", err)
	}

	return stats, nil
}

// GetPortfolioSummary retrieves a summary of a portfolio's balances
func (r *BalanceRepository) GetPortfolioSummary(ctx context.Context, portfolioID string) (*repositories.PortfolioSummary, error) {
	summary := &repositories.PortfolioSummary{
		PortfolioID: portfolioID,
	}

	// Get total positions count
	query := "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1"
	if err := r.db.GetContext(ctx, &summary.TotalPositions, query, portfolioID); err != nil {
		return nil, repositories.NewRepositoryError("get_summary", "balance", err)
	}

	// Get cash balance
	cashQuery := "SELECT COALESCE(quantity_long, 0) FROM balances WHERE portfolio_id = $1 AND security_id IS NULL"
	if err := r.db.GetContext(ctx, &summary.CashBalance, cashQuery, portfolioID); err != nil {
		if err != sql.ErrNoRows {
			return nil, repositories.NewRepositoryError("get_summary", "balance", err)
		}
		summary.CashBalance = decimal.Zero
	}

	// Get long positions count
	longQuery := "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1 AND quantity_long > 0"
	if err := r.db.GetContext(ctx, &summary.LongPositions, longQuery, portfolioID); err != nil {
		return nil, repositories.NewRepositoryError("get_summary", "balance", err)
	}

	// Get short positions count
	shortQuery := "SELECT COUNT(*) FROM balances WHERE portfolio_id = $1 AND quantity_short > 0"
	if err := r.db.GetContext(ctx, &summary.ShortPositions, shortQuery, portfolioID); err != nil {
		return nil, repositories.NewRepositoryError("get_summary", "balance", err)
	}

	// Get last updated time
	lastUpdatedQuery := "SELECT MAX(last_updated) FROM balances WHERE portfolio_id = $1"
	if err := r.db.GetContext(ctx, &summary.LastUpdated, lastUpdatedQuery, portfolioID); err != nil {
		if err != sql.ErrNoRows {
			return nil, repositories.NewRepositoryError("get_summary", "balance", err)
		}
		summary.LastUpdated = time.Time{}
	}

	return summary, nil
}

// buildListQuery builds the SELECT query for listing balances
func (r *BalanceRepository) buildListQuery(filter repositories.BalanceFilter) (string, []interface{}, error) {
	query := `
		SELECT id, portfolio_id, security_id, quantity_long, quantity_short,
			   last_updated, version, created_at
		FROM balances`

	whereClause, args := r.buildWhereClause(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	// Add sorting
	if orderBy := r.buildOrderBy(filter); orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	// Add pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	return query, args, nil
}

// buildCountQuery builds the COUNT query for balances
func (r *BalanceRepository) buildCountQuery(filter repositories.BalanceFilter) (string, []interface{}, error) {
	query := "SELECT COUNT(*) FROM balances"

	whereClause, args := r.buildWhereClause(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query, args, nil
}

// buildWhereClause builds the WHERE clause for balance queries
func (r *BalanceRepository) buildWhereClause(filter repositories.BalanceFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// ID filters
	if filter.ID != nil {
		conditions = append(conditions, fmt.Sprintf("id = $%d", argIndex))
		args = append(args, *filter.ID)
		argIndex++
	}

	if filter.PortfolioID != nil {
		conditions = append(conditions, fmt.Sprintf("portfolio_id = $%d", argIndex))
		args = append(args, *filter.PortfolioID)
		argIndex++
	}

	if filter.SecurityID != nil {
		conditions = append(conditions, fmt.Sprintf("security_id = $%d", argIndex))
		args = append(args, *filter.SecurityID)
		argIndex++
	}

	// Cash vs security filters
	if filter.CashOnly {
		conditions = append(conditions, "security_id IS NULL")
	}

	if filter.SecuritiesOnly {
		conditions = append(conditions, "security_id IS NOT NULL")
	}

	if filter.IncludeCash != nil && !*filter.IncludeCash {
		conditions = append(conditions, "security_id IS NOT NULL")
	}

	if filter.OnlyCash != nil && *filter.OnlyCash {
		conditions = append(conditions, "security_id IS NULL")
	}

	// Date filters
	if filter.LastUpdatedFrom != nil {
		conditions = append(conditions, fmt.Sprintf("last_updated >= $%d", argIndex))
		args = append(args, *filter.LastUpdatedFrom)
		argIndex++
	}

	if filter.LastUpdatedTo != nil {
		conditions = append(conditions, fmt.Sprintf("last_updated <= $%d", argIndex))
		args = append(args, *filter.LastUpdatedTo)
		argIndex++
	}

	// Quantity filters
	if filter.QuantityLongMin != nil {
		conditions = append(conditions, fmt.Sprintf("quantity_long >= $%d", argIndex))
		args = append(args, *filter.QuantityLongMin)
		argIndex++
	}

	if filter.QuantityLongMax != nil {
		conditions = append(conditions, fmt.Sprintf("quantity_long <= $%d", argIndex))
		args = append(args, *filter.QuantityLongMax)
		argIndex++
	}

	if filter.QuantityShortMin != nil {
		conditions = append(conditions, fmt.Sprintf("quantity_short >= $%d", argIndex))
		args = append(args, *filter.QuantityShortMin)
		argIndex++
	}

	if filter.QuantityShortMax != nil {
		conditions = append(conditions, fmt.Sprintf("quantity_short <= $%d", argIndex))
		args = append(args, *filter.QuantityShortMax)
		argIndex++
	}

	// Zero balance filters
	if filter.ExcludeZeroBalances {
		conditions = append(conditions, "(quantity_long != 0 OR quantity_short != 0)")
	}

	if filter.OnlyZeroBalances {
		conditions = append(conditions, "quantity_long = 0 AND quantity_short = 0")
	}

	// Collection filters (IN clauses)
	if len(filter.IDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("id = ANY($%d)", argIndex))
		args = append(args, pq.Array(filter.IDs))
		argIndex++
	}

	if len(filter.PortfolioIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("portfolio_id = ANY($%d)", argIndex))
		args = append(args, pq.Array(filter.PortfolioIDs))
		argIndex++
	}

	if len(filter.SecurityIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("security_id = ANY($%d)", argIndex))
		args = append(args, pq.Array(filter.SecurityIDs))
		argIndex++
	}

	return strings.Join(conditions, " AND "), args
}

// buildOrderBy builds the ORDER BY clause
func (r *BalanceRepository) buildOrderBy(filter repositories.BalanceFilter) string {
	if len(filter.SortBy) > 0 {
		return strings.Join(filter.SortBy, ", ")
	}

	// Default sorting: cash first, then by security ID, then by creation date
	return "security_id NULLS FIRST, created_at DESC"
}
