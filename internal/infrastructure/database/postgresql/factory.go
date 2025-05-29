package postgresql

import (
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/infrastructure/database"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// RepositoryFactory provides factory methods for creating PostgreSQL repositories
type RepositoryFactory struct {
	db     *database.DB
	logger logger.Logger
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(db *database.DB, logger logger.Logger) *RepositoryFactory {
	return &RepositoryFactory{
		db:     db,
		logger: logger,
	}
}

// TransactionRepository creates a new PostgreSQL transaction repository
func (f *RepositoryFactory) TransactionRepository() repositories.TransactionRepository {
	return NewTransactionRepository(f.db, f.logger)
}

// BalanceRepository creates a new PostgreSQL balance repository
func (f *RepositoryFactory) BalanceRepository() repositories.BalanceRepository {
	return NewBalanceRepository(f.db, f.logger)
}

// CreateAllRepositories creates all repository instances
func (f *RepositoryFactory) CreateAllRepositories() (repositories.TransactionRepository, repositories.BalanceRepository) {
	return f.TransactionRepository(), f.BalanceRepository()
}

// RepositoryContainer holds all repository instances
type RepositoryContainer struct {
	TransactionRepo repositories.TransactionRepository
	BalanceRepo     repositories.BalanceRepository
}

// NewRepositoryContainer creates a new repository container with all repositories
func NewRepositoryContainer(db *database.DB, logger logger.Logger) *RepositoryContainer {
	factory := NewRepositoryFactory(db, logger)

	return &RepositoryContainer{
		TransactionRepo: factory.TransactionRepository(),
		BalanceRepo:     factory.BalanceRepository(),
	}
}
