package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// TransactionRepository implements the repositories.TransactionRepository interface for PostgreSQL
type TransactionRepository struct {
	db     *database.DB
	logger logger.Logger
}

// NewTransactionRepository creates a new PostgreSQL transaction repository
func NewTransactionRepository(db *database.DB, logger logger.Logger) *TransactionRepository {
	return &TransactionRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new transaction
func (r *TransactionRepository) Create(ctx context.Context, transaction *repositories.Transaction) error {
	query := `
		INSERT INTO transactions (
			portfolio_id, security_id, source_id, status, transaction_type,
			quantity, price, transaction_date, reprocessing_attempts, version
		) VALUES (
			:portfolio_id, :security_id, :source_id, :status, :transaction_type,
			:quantity, :price, :transaction_date, :reprocessing_attempts, :version
		) RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, query, transaction)
	if err != nil {
		if isDuplicateKeyError(err) {
			return repositories.NewDuplicateKeyError("transaction", "source_id", transaction.SourceID)
		}
		return repositories.NewRepositoryError("create", "transaction", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&transaction.ID, &transaction.CreatedAt, &transaction.UpdatedAt); err != nil {
			return repositories.NewRepositoryError("scan", "transaction", err)
		}
	}

	r.logger.Info("Transaction created",
		logger.Int64("id", transaction.ID),
		logger.String("sourceId", transaction.SourceID),
		logger.String("portfolioId", transaction.PortfolioID))

	return nil
}

// CreateBatch creates multiple transactions in a single transaction
func (r *TransactionRepository) CreateBatch(ctx context.Context, transactions []*repositories.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	return r.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			INSERT INTO transactions (
				portfolio_id, security_id, source_id, status, transaction_type,
				quantity, price, transaction_date, reprocessing_attempts, version
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			) RETURNING id, created_at, updated_at`

		for _, transaction := range transactions {
			err := tx.QueryRowxContext(ctx, query,
				transaction.PortfolioID, transaction.SecurityID, transaction.SourceID,
				transaction.Status, transaction.TransactionType, transaction.Quantity,
				transaction.Price, transaction.TransactionDate, transaction.ReprocessingAttempts,
				transaction.Version,
			).Scan(&transaction.ID, &transaction.CreatedAt, &transaction.UpdatedAt)

			if err != nil {
				if isDuplicateKeyError(err) {
					return repositories.NewDuplicateKeyError("transaction", "source_id", transaction.SourceID)
				}
				return repositories.NewRepositoryError("create_batch", "transaction", err)
			}
		}

		r.logger.Info("Transaction batch created",
			logger.Int("count", len(transactions)))

		return nil
	})
}

// GetByID retrieves a transaction by ID
func (r *TransactionRepository) GetByID(ctx context.Context, id int64) (*repositories.Transaction, error) {
	query := `
		SELECT id, portfolio_id, security_id, source_id, status, transaction_type,
			   quantity, price, transaction_date, reprocessing_attempts, version,
			   created_at, updated_at
		FROM transactions
		WHERE id = $1`

	var transaction repositories.Transaction
	err := r.db.GetContext(ctx, &transaction, query, id)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.NewNotFoundError("transaction", id)
		}
		return nil, repositories.NewRepositoryError("get", "transaction", err)
	}

	return &transaction, nil
}

// GetBySourceID retrieves a transaction by source ID
func (r *TransactionRepository) GetBySourceID(ctx context.Context, sourceID string) (*repositories.Transaction, error) {
	query := `
		SELECT id, portfolio_id, security_id, source_id, status, transaction_type,
			   quantity, price, transaction_date, reprocessing_attempts, version,
			   created_at, updated_at
		FROM transactions
		WHERE source_id = $1`

	var transaction repositories.Transaction
	err := r.db.GetContext(ctx, &transaction, query, sourceID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.NewNotFoundError("transaction", sourceID)
		}
		return nil, repositories.NewRepositoryError("get", "transaction", err)
	}

	return &transaction, nil
}

// List retrieves transactions based on filter criteria
func (r *TransactionRepository) List(ctx context.Context, filter repositories.TransactionFilter) ([]*repositories.Transaction, error) {
	query, args, err := r.buildListQuery(filter)
	if err != nil {
		return nil, repositories.NewRepositoryError("build_query", "transaction", err)
	}

	var transactions []*repositories.Transaction
	err = r.db.SelectContext(ctx, &transactions, query, args...)

	if err != nil {
		return nil, repositories.NewRepositoryError("list", "transaction", err)
	}

	return transactions, nil
}

// Count counts transactions based on filter criteria
func (r *TransactionRepository) Count(ctx context.Context, filter repositories.TransactionFilter) (int64, error) {
	query, args, err := r.buildCountQuery(filter)
	if err != nil {
		return 0, repositories.NewRepositoryError("build_query", "transaction", err)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, query, args...)

	if err != nil {
		return 0, repositories.NewRepositoryError("count", "transaction", err)
	}

	return count, nil
}

// Update updates an existing transaction with optimistic locking
func (r *TransactionRepository) Update(ctx context.Context, transaction *repositories.Transaction) error {
	query := `
		UPDATE transactions SET
			portfolio_id = :portfolio_id,
			security_id = :security_id,
			source_id = :source_id,
			status = :status,
			transaction_type = :transaction_type,
			quantity = :quantity,
			price = :price,
			transaction_date = :transaction_date,
			reprocessing_attempts = :reprocessing_attempts,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = :id AND version = :version
		RETURNING version, updated_at`

	originalVersion := transaction.Version
	transaction.Version++ // Optimistic increment

	rows, err := r.db.NamedQueryContext(ctx, query, transaction)
	if err != nil {
		transaction.Version = originalVersion // Restore on error
		if isDuplicateKeyError(err) {
			return repositories.NewDuplicateKeyError("transaction", "source_id", transaction.SourceID)
		}
		return repositories.NewRepositoryError("update", "transaction", err)
	}
	defer rows.Close()

	if !rows.Next() {
		transaction.Version = originalVersion // Restore on error
		return repositories.NewOptimisticLockError("transaction", transaction.ID, originalVersion, transaction.Version)
	}

	if err := rows.Scan(&transaction.Version, &transaction.UpdatedAt); err != nil {
		return repositories.NewRepositoryError("scan", "transaction", err)
	}

	r.logger.Info("Transaction updated",
		logger.Int64("id", transaction.ID),
		logger.Int("version", transaction.Version))

	return nil
}

// UpdateStatus updates the status of a transaction with optimistic locking
func (r *TransactionRepository) UpdateStatus(ctx context.Context, id int64, status string, errorMessage *string, version int) error {
	query := `
		UPDATE transactions SET
			status = $1,
			error_message = $2,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND version = $4`

	result, err := r.db.ExecContext(ctx, query, status, errorMessage, id, version)
	if err != nil {
		return repositories.NewRepositoryError("update_status", "transaction", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return repositories.NewRepositoryError("check_rows", "transaction", err)
	}

	if rowsAffected == 0 {
		return repositories.NewOptimisticLockError("transaction", id, version, version+1)
	}

	r.logger.Info("Transaction status updated",
		logger.Int64("id", id),
		logger.String("status", status))

	return nil
}

// IncrementReprocessingAttempts increments the reprocessing attempts counter
func (r *TransactionRepository) IncrementReprocessingAttempts(ctx context.Context, id int64, version int) error {
	query := `
		UPDATE transactions SET
			reprocessing_attempts = reprocessing_attempts + 1,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND version = $2`

	result, err := r.db.ExecContext(ctx, query, id, version)
	if err != nil {
		return repositories.NewRepositoryError("increment_attempts", "transaction", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return repositories.NewRepositoryError("check_rows", "transaction", err)
	}

	if rowsAffected == 0 {
		return repositories.NewOptimisticLockError("transaction", id, version, version+1)
	}

	r.logger.Info("Transaction reprocessing attempts incremented",
		logger.Int64("id", id))

	return nil
}

// GetNewTransactions retrieves transactions with NEW status
func (r *TransactionRepository) GetNewTransactions(ctx context.Context, limit int) ([]*repositories.Transaction, error) {
	filter := repositories.TransactionFilter{
		Status: stringPtr("NEW"),
		Limit:  limit,
		SortBy: []string{"created_at"},
	}

	return r.List(ctx, filter)
}

// GetTransactionsByPortfolio retrieves transactions for a specific portfolio
func (r *TransactionRepository) GetTransactionsByPortfolio(ctx context.Context, portfolioID string, limit int, offset int) ([]*repositories.Transaction, error) {
	filter := repositories.TransactionFilter{
		PortfolioID: &portfolioID,
		Limit:       limit,
		Offset:      offset,
		SortBy:      []string{"transaction_date DESC", "created_at DESC"},
	}

	return r.List(ctx, filter)
}

// GetTransactionsByStatus retrieves transactions by status
func (r *TransactionRepository) GetTransactionsByStatus(ctx context.Context, status string, limit int, offset int) ([]*repositories.Transaction, error) {
	filter := repositories.TransactionFilter{
		Status: &status,
		Limit:  limit,
		Offset: offset,
		SortBy: []string{"created_at"},
	}

	return r.List(ctx, filter)
}

// UpdateTransactionsStatus updates the status of multiple transactions
func (r *TransactionRepository) UpdateTransactionsStatus(ctx context.Context, ids []int64, status string, errorMessage *string) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			UPDATE transactions SET
				status = $1,
				error_message = $2,
				version = version + 1,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ANY($3)`

		_, err := tx.ExecContext(ctx, query, status, errorMessage, pq.Array(ids))
		if err != nil {
			return repositories.NewRepositoryError("update_status_batch", "transaction", err)
		}

		r.logger.Info("Transactions status updated",
			logger.Int("count", len(ids)),
			logger.String("status", status))

		return nil
	})
}

// GetTransactionStats retrieves transaction statistics
func (r *TransactionRepository) GetTransactionStats(ctx context.Context) (*repositories.TransactionStats, error) {
	stats := &repositories.TransactionStats{
		StatusCounts: make(map[string]int64),
		TypeCounts:   make(map[string]int64),
	}

	// Get total count
	if err := r.db.GetContext(ctx, &stats.TotalCount, "SELECT COUNT(*) FROM transactions"); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "transaction", err)
	}

	// Get status counts
	statusQuery := `
		SELECT status, COUNT(*) as count
		FROM transactions
		GROUP BY status`

	rows, err := r.db.QueryxContext(ctx, statusQuery)
	if err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "transaction", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, repositories.NewRepositoryError("scan_stats", "transaction", err)
		}
		stats.StatusCounts[status] = count
	}

	// Get type counts
	typeQuery := `
		SELECT transaction_type, COUNT(*) as count
		FROM transactions
		GROUP BY transaction_type`

	rows, err = r.db.QueryxContext(ctx, typeQuery)
	if err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "transaction", err)
	}
	defer rows.Close()

	for rows.Next() {
		var transactionType string
		var count int64
		if err := rows.Scan(&transactionType, &count); err != nil {
			return nil, repositories.NewRepositoryError("scan_stats", "transaction", err)
		}
		stats.TypeCounts[transactionType] = count
	}

	// Get recent count (24h)
	recentQuery := `
		SELECT COUNT(*)
		FROM transactions
		WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'`

	if err := r.db.GetContext(ctx, &stats.RecentCount24h, recentQuery); err != nil {
		return nil, repositories.NewRepositoryError("get_stats", "transaction", err)
	}

	return stats, nil
}

// buildListQuery builds the SELECT query for listing transactions
func (r *TransactionRepository) buildListQuery(filter repositories.TransactionFilter) (string, []interface{}, error) {
	query := `
		SELECT id, portfolio_id, security_id, source_id, status, transaction_type,
			   quantity, price, transaction_date, reprocessing_attempts, version,
			   created_at, updated_at
		FROM transactions`

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

// buildCountQuery builds the COUNT query for transactions
func (r *TransactionRepository) buildCountQuery(filter repositories.TransactionFilter) (string, []interface{}, error) {
	query := "SELECT COUNT(*) FROM transactions"

	whereClause, args := r.buildWhereClause(filter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query, args, nil
}

// buildWhereClause builds the WHERE clause for transaction queries
func (r *TransactionRepository) buildWhereClause(filter repositories.TransactionFilter) (string, []interface{}) {
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

	if filter.SourceID != nil {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, *filter.SourceID)
		argIndex++
	}

	// Status and type filters
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.TransactionType != nil {
		conditions = append(conditions, fmt.Sprintf("transaction_type = $%d", argIndex))
		args = append(args, *filter.TransactionType)
		argIndex++
	}

	// Date filters
	if filter.TransactionDate != nil {
		conditions = append(conditions, fmt.Sprintf("transaction_date = $%d", argIndex))
		args = append(args, *filter.TransactionDate)
		argIndex++
	}

	if filter.TransactionDateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("transaction_date >= $%d", argIndex))
		args = append(args, *filter.TransactionDateFrom)
		argIndex++
	}

	if filter.TransactionDateTo != nil {
		conditions = append(conditions, fmt.Sprintf("transaction_date <= $%d", argIndex))
		args = append(args, *filter.TransactionDateTo)
		argIndex++
	}

	// Amount filters
	if filter.QuantityMin != nil {
		conditions = append(conditions, fmt.Sprintf("quantity >= $%d", argIndex))
		args = append(args, *filter.QuantityMin)
		argIndex++
	}

	if filter.QuantityMax != nil {
		conditions = append(conditions, fmt.Sprintf("quantity <= $%d", argIndex))
		args = append(args, *filter.QuantityMax)
		argIndex++
	}

	if filter.PriceMin != nil {
		conditions = append(conditions, fmt.Sprintf("price >= $%d", argIndex))
		args = append(args, *filter.PriceMin)
		argIndex++
	}

	if filter.PriceMax != nil {
		conditions = append(conditions, fmt.Sprintf("price <= $%d", argIndex))
		args = append(args, *filter.PriceMax)
		argIndex++
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

	if len(filter.Statuses) > 0 {
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argIndex))
		args = append(args, pq.Array(filter.Statuses))
		argIndex++
	}

	if len(filter.TransactionTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("transaction_type = ANY($%d)", argIndex))
		args = append(args, pq.Array(filter.TransactionTypes))
		argIndex++
	}

	return strings.Join(conditions, " AND "), args
}

// buildOrderBy builds the ORDER BY clause
func (r *TransactionRepository) buildOrderBy(filter repositories.TransactionFilter) string {
	if len(filter.SortBy) > 0 {
		return strings.Join(filter.SortBy, ", ")
	}

	// Default sorting
	return "created_at DESC"
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func isDuplicateKeyError(err error) bool {
	// PostgreSQL specific error checking
	return strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "unique constraint")
}
