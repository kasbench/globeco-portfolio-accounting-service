package services

import (
	"context"
	"fmt"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/mappers"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/services"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/shopspring/decimal"
)

// BalanceService interface defines balance application service operations
type BalanceService interface {
	// Balance query operations
	GetBalance(ctx context.Context, id int64) (*dto.BalanceDTO, error)
	GetBalances(ctx context.Context, filter dto.BalanceFilter) (*dto.BalanceListResponse, error)
	GetBalancesByPortfolio(ctx context.Context, portfolioID string, pagination dto.PaginationRequest) (*dto.BalanceListResponse, error)

	// Portfolio summary operations
	GetPortfolioSummary(ctx context.Context, portfolioID string) (*dto.PortfolioSummaryDTO, error)
	GetPortfolioSummaries(ctx context.Context, filter dto.PortfolioSummaryFilter) ([]dto.PortfolioSummaryDTO, error)

	// Balance statistics
	GetBalanceStats(ctx context.Context, filter dto.BalanceFilter) (*dto.BalanceStatsDTO, error)

	// Balance update operations
	UpdateBalance(ctx context.Context, id int64, updateRequest dto.BalanceUpdateRequest) (*dto.BalanceUpdateResponse, error)
	BulkUpdateBalances(ctx context.Context, bulkRequest dto.BulkBalanceUpdateRequest) (*dto.BulkBalanceUpdateResponse, error)

	// Health and monitoring
	GetServiceHealth(ctx context.Context) error
}

// balanceService implements BalanceService interface
type balanceService struct {
	balanceRepo       repositories.BalanceRepository
	transactionRepo   repositories.TransactionRepository
	balanceCalculator services.BalanceCalculator
	balanceMapper     *mappers.BalanceMapper
	config            BalanceServiceConfig
	logger            logger.Logger
}

// BalanceServiceConfig holds configuration for balance service
type BalanceServiceConfig struct {
	MaxBulkUpdateSize    int
	HistoryRetentionDays int
	CacheTimeout         time.Duration
}

// NewBalanceService creates a new balance application service
func NewBalanceService(
	balanceRepo repositories.BalanceRepository,
	transactionRepo repositories.TransactionRepository,
	balanceCalculator services.BalanceCalculator,
	balanceMapper *mappers.BalanceMapper,
	config BalanceServiceConfig,
	lg logger.Logger,
) BalanceService {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set default configuration
	if config.MaxBulkUpdateSize == 0 {
		config.MaxBulkUpdateSize = 1000
	}
	if config.HistoryRetentionDays == 0 {
		config.HistoryRetentionDays = 90
	}
	if config.CacheTimeout == 0 {
		config.CacheTimeout = 15 * time.Minute
	}

	return &balanceService{
		balanceRepo:       balanceRepo,
		transactionRepo:   transactionRepo,
		balanceCalculator: balanceCalculator,
		balanceMapper:     balanceMapper,
		config:            config,
		logger:            lg,
	}
}

// GetBalance retrieves a balance by ID
func (s *balanceService) GetBalance(ctx context.Context, id int64) (*dto.BalanceDTO, error) {
	s.logger.Debug("Retrieving balance",
		logger.Int64("balanceId", id))

	repoBalance, err := s.balanceRepo.GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			s.logger.Warn("Balance not found",
				logger.Int64("balanceId", id))
			return nil, fmt.Errorf("balance not found: %d", id)
		}
		s.logger.Error("Failed to retrieve balance",
			logger.Err(err),
			logger.Int64("balanceId", id))
		return nil, fmt.Errorf("failed to retrieve balance: %w", err)
	}

	domainBalance := s.convertRepoToDomain(repoBalance)
	return s.balanceMapper.ToDTO(domainBalance), nil
}

// GetBalances retrieves balances with filtering and pagination
func (s *balanceService) GetBalances(ctx context.Context, filter dto.BalanceFilter) (*dto.BalanceListResponse, error) {
	s.logger.Debug("Retrieving balances with filter",
		logger.Int("limit", filter.Pagination.Limit),
		logger.Int("offset", filter.Pagination.Offset))

	// Validate filter
	if !filter.IsValid() {
		return nil, fmt.Errorf("invalid filter parameters")
	}

	// Convert DTO filter to repository filter
	repoFilter := s.convertDTOFilterToRepo(filter)

	// Set default pagination
	if repoFilter.Limit == 0 {
		repoFilter.Limit = 50
	}
	if repoFilter.Limit > 1000 {
		repoFilter.Limit = 1000
	}

	// Get balances from repository
	repoBalances, err := s.balanceRepo.List(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to retrieve balances",
			logger.Err(err))
		return nil, fmt.Errorf("failed to retrieve balances: %w", err)
	}

	// Get total count for pagination
	totalCount, err := s.balanceRepo.Count(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count balances",
			logger.Err(err))
		return nil, fmt.Errorf("failed to count balances: %w", err)
	}

	s.logger.Debug("Retrieved balances",
		logger.Int("count", len(repoBalances)),
		logger.Int64("total", totalCount))

	// Convert repository balances to domain balances
	domainBalances := make([]*models.Balance, len(repoBalances))
	for i, repoBalance := range repoBalances {
		domainBalances[i] = s.convertRepoToDomain(repoBalance)
	}

	return &dto.BalanceListResponse{
		Balances: s.balanceMapper.ToDTOs(domainBalances),
		Pagination: dto.NewPaginationResponse(
			repoFilter.Limit,
			repoFilter.Offset,
			totalCount,
		),
	}, nil
}

// GetBalancesByPortfolio retrieves all balances for a specific portfolio
func (s *balanceService) GetBalancesByPortfolio(ctx context.Context, portfolioID string, pagination dto.PaginationRequest) (*dto.BalanceListResponse, error) {
	s.logger.Debug("Retrieving balances for portfolio",
		logger.String("portfolioId", portfolioID))

	// Create filter for portfolio
	filter := dto.BalanceFilter{
		PortfolioID: &portfolioID,
		Pagination:  pagination,
	}

	return s.GetBalances(ctx, filter)
}

// GetPortfolioSummary retrieves a summary of balances for a portfolio
func (s *balanceService) GetPortfolioSummary(ctx context.Context, portfolioID string) (*dto.PortfolioSummaryDTO, error) {
	s.logger.Debug("Retrieving portfolio summary",
		logger.String("portfolioId", portfolioID))

	// Get all balances for the portfolio
	repoFilter := repositories.BalanceFilter{
		PortfolioID: &portfolioID,
		Limit:       1000, // Get all balances for summary
	}

	repoBalances, err := s.balanceRepo.List(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to retrieve portfolio balances",
			logger.Err(err),
			logger.String("portfolioId", portfolioID))
		return nil, fmt.Errorf("failed to retrieve portfolio balances: %w", err)
	}

	if len(repoBalances) == 0 {
		s.logger.Warn("No balances found for portfolio",
			logger.String("portfolioId", portfolioID))
		return nil, fmt.Errorf("no balances found for portfolio: %s", portfolioID)
	}

	// Convert to domain balances
	domainBalances := make([]*models.Balance, len(repoBalances))
	for i, repoBalance := range repoBalances {
		domainBalances[i] = s.convertRepoToDomain(repoBalance)
	}

	// Create portfolio summary
	summary := s.balanceMapper.ToPortfolioSummaryDTO(portfolioID, domainBalances)

	s.logger.Debug("Portfolio summary created",
		logger.String("portfolioId", portfolioID),
		logger.Int("securityCount", summary.SecurityCount))

	return summary, nil
}

// GetPortfolioSummaries retrieves summaries for multiple portfolios
func (s *balanceService) GetPortfolioSummaries(ctx context.Context, filter dto.PortfolioSummaryFilter) ([]dto.PortfolioSummaryDTO, error) {
	s.logger.Debug("Retrieving portfolio summaries with filter")

	var portfolioIDs []string
	if len(filter.PortfolioIDs) > 0 {
		portfolioIDs = filter.PortfolioIDs
	} else {
		// Get distinct portfolio IDs if none specified
		repoFilter := repositories.BalanceFilter{
			Limit: 10000, // Large limit to get all portfolios
		}
		repoBalances, err := s.balanceRepo.List(ctx, repoFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve balances: %w", err)
		}

		// Extract unique portfolio IDs
		portfolioMap := make(map[string]bool)
		for _, balance := range repoBalances {
			portfolioMap[balance.PortfolioID] = true
		}

		portfolioIDs = make([]string, 0, len(portfolioMap))
		for portfolioID := range portfolioMap {
			portfolioIDs = append(portfolioIDs, portfolioID)
		}
	}

	// Get summaries for each portfolio
	summaries := make([]dto.PortfolioSummaryDTO, 0, len(portfolioIDs))
	for _, portfolioID := range portfolioIDs {
		summary, err := s.GetPortfolioSummary(ctx, portfolioID)
		if err != nil {
			s.logger.Warn("Failed to get portfolio summary",
				logger.String("portfolioId", portfolioID),
				logger.Err(err))
			continue
		}
		summaries = append(summaries, *summary)
	}

	s.logger.Debug("Retrieved portfolio summaries",
		logger.Int("count", len(summaries)))

	return summaries, nil
}

// GetBalanceStats retrieves balance statistics
func (s *balanceService) GetBalanceStats(ctx context.Context, filter dto.BalanceFilter) (*dto.BalanceStatsDTO, error) {
	s.logger.Debug("Retrieving balance statistics")

	// Convert filter
	repoFilter := s.convertDTOFilterToRepo(filter)

	// Get total count
	totalCount, err := s.balanceRepo.Count(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count balances",
			logger.Err(err))
		return nil, fmt.Errorf("failed to count balances: %w", err)
	}

	// Get distinct portfolio count
	repoFilter.Limit = 10000 // Large limit to get all balances
	repoBalances, err := s.balanceRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve balances for stats: %w", err)
	}

	// Calculate statistics
	portfolioMap := make(map[string]bool)
	securityMap := make(map[string]bool)
	cashBalanceCount := int64(0)
	zeroBalanceCount := int64(0)
	var lastUpdated time.Time

	for _, balance := range repoBalances {
		portfolioMap[balance.PortfolioID] = true

		if balance.SecurityID == nil {
			cashBalanceCount++
		} else {
			securityMap[*balance.SecurityID] = true
		}

		if balance.QuantityLong.IsZero() && balance.QuantityShort.IsZero() {
			zeroBalanceCount++
		}

		if balance.LastUpdated.After(lastUpdated) {
			lastUpdated = balance.LastUpdated
		}
	}

	statsDTO := &dto.BalanceStatsDTO{
		TotalBalances:    totalCount,
		PortfolioCount:   int64(len(portfolioMap)),
		SecurityCount:    int64(len(securityMap)),
		CashBalanceCount: cashBalanceCount,
		ZeroBalanceCount: zeroBalanceCount,
		LastUpdated:      lastUpdated,
	}

	return statsDTO, nil
}

// UpdateBalance updates a single balance
func (s *balanceService) UpdateBalance(ctx context.Context, id int64, updateRequest dto.BalanceUpdateRequest) (*dto.BalanceUpdateResponse, error) {
	s.logger.Info("Updating balance",
		logger.Int64("balanceId", id))

	// Validate update request
	validationErrors := s.balanceMapper.ValidateBalanceUpdateRequest(&updateRequest)
	if len(validationErrors) > 0 {
		s.logger.Warn("Balance update validation failed",
			logger.Int("errorCount", len(validationErrors)),
			logger.Int64("balanceId", id))
		return nil, fmt.Errorf("validation failed: %d errors", len(validationErrors))
	}

	// Get current balance
	currentRepoBalance, err := s.balanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get current balance: %w", err)
	}

	// Keep copy of previous balance for response
	previousBalance := s.convertRepoToDomain(currentRepoBalance)

	// Update balance fields
	wasUpdated := false
	if updateRequest.QuantityLong != nil {
		currentRepoBalance.QuantityLong = *updateRequest.QuantityLong
		wasUpdated = true
	}
	if updateRequest.QuantityShort != nil {
		currentRepoBalance.QuantityShort = *updateRequest.QuantityShort
		wasUpdated = true
	}

	if !wasUpdated {
		return &dto.BalanceUpdateResponse{
			Balance:       *s.balanceMapper.ToDTO(previousBalance),
			Updated:       false,
			PreviousValue: *s.balanceMapper.ToDTO(previousBalance),
		}, nil
	}

	// Update in repository
	err = s.balanceRepo.Update(ctx, currentRepoBalance)
	if err != nil {
		s.logger.Error("Failed to update balance",
			logger.Err(err),
			logger.Int64("balanceId", id))
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	s.logger.Info("Balance updated successfully",
		logger.Int64("balanceId", id))

	updatedBalance := s.convertRepoToDomain(currentRepoBalance)
	return &dto.BalanceUpdateResponse{
		Balance:       *s.balanceMapper.ToDTO(updatedBalance),
		Updated:       true,
		PreviousValue: *s.balanceMapper.ToDTO(previousBalance),
	}, nil
}

// BulkUpdateBalances updates multiple balances
func (s *balanceService) BulkUpdateBalances(ctx context.Context, bulkRequest dto.BulkBalanceUpdateRequest) (*dto.BulkBalanceUpdateResponse, error) {
	s.logger.Info("Bulk updating balances",
		logger.Int("count", len(bulkRequest.Updates)))

	// Validate bulk request
	validationErrors := s.balanceMapper.ValidateBulkBalanceUpdateRequest(&bulkRequest)
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("bulk validation failed: %d errors", len(validationErrors))
	}

	if len(bulkRequest.Updates) > s.config.MaxBulkUpdateSize {
		return nil, fmt.Errorf("bulk update size exceeds limit: %d > %d", len(bulkRequest.Updates), s.config.MaxBulkUpdateSize)
	}

	var successful []dto.BalanceUpdateResponse
	var failed []dto.BalanceUpdateError

	// Process each update
	for _, updateItem := range bulkRequest.Updates {
		updateRequest := dto.BalanceUpdateRequest{
			QuantityLong:  updateItem.QuantityLong,
			QuantityShort: updateItem.QuantityShort,
			Version:       updateItem.Version,
		}

		updateResponse, err := s.UpdateBalance(ctx, updateItem.BalanceID, updateRequest)
		if err != nil {
			failed = append(failed, dto.BalanceUpdateError{
				BalanceID: updateItem.BalanceID,
				Errors: []dto.ValidationError{{
					Field:   "update",
					Message: err.Error(),
					Value:   fmt.Sprintf("balance_%d", updateItem.BalanceID),
				}},
			})
		} else {
			successful = append(successful, *updateResponse)
		}
	}

	s.logger.Info("Bulk balance update completed",
		logger.Int("successful", len(successful)),
		logger.Int("failed", len(failed)))

	batchResponse := s.balanceMapper.ToBatchUpdateResponse(successful, failed)
	return &batchResponse, nil
}

// GetServiceHealth checks the health of the balance service
func (s *balanceService) GetServiceHealth(ctx context.Context) error {
	s.logger.Debug("Checking balance service health")

	// Simple count query to check database connectivity
	_, err := s.balanceRepo.Count(ctx, repositories.BalanceFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("balance repository health check failed: %w", err)
	}

	return nil
}

// Helper functions

// convertRepoToDomain converts a repository balance to domain balance
func (s *balanceService) convertRepoToDomain(repoBalance *repositories.Balance) *models.Balance {
	builder := models.NewBalanceBuilder().
		WithID(repoBalance.ID).
		WithPortfolioID(repoBalance.PortfolioID).
		WithSecurityID(repoBalance.SecurityID).
		WithQuantityLong(repoBalance.QuantityLong).
		WithQuantityShort(repoBalance.QuantityShort).
		WithVersion(repoBalance.Version).
		WithTimestamps(repoBalance.CreatedAt, repoBalance.LastUpdated)

	// Build should not fail for valid repository data
	domainBalance, err := builder.Build()
	if err != nil {
		s.logger.Error("Failed to convert repository balance to domain model",
			logger.Int64("balanceId", repoBalance.ID),
			logger.Err(err))
		// Return a minimal balance in case of error
		return &models.Balance{}
	}

	return domainBalance
}

// convertDTOFilterToRepo converts DTO filter to repository filter
func (s *balanceService) convertDTOFilterToRepo(dtoFilter dto.BalanceFilter) repositories.BalanceFilter {
	repoFilter := repositories.BalanceFilter{
		IDs:    dtoFilter.IDs,
		Limit:  dtoFilter.Pagination.Limit,
		Offset: dtoFilter.Pagination.Offset,
	}

	// Convert single filters
	if dtoFilter.PortfolioID != nil {
		repoFilter.PortfolioID = dtoFilter.PortfolioID
	}
	if dtoFilter.SecurityID != nil {
		repoFilter.SecurityID = dtoFilter.SecurityID
	}

	// Convert slice filters
	if len(dtoFilter.PortfolioIDs) > 0 {
		repoFilter.PortfolioIDs = dtoFilter.PortfolioIDs
	}
	if len(dtoFilter.SecurityIDs) > 0 {
		repoFilter.SecurityIDs = dtoFilter.SecurityIDs
	}

	// Convert amount filters
	if dtoFilter.MinQuantityLong != nil {
		repoFilter.QuantityLongMin = dtoFilter.MinQuantityLong
	}
	if dtoFilter.MaxQuantityLong != nil {
		repoFilter.QuantityLongMax = dtoFilter.MaxQuantityLong
	}
	if dtoFilter.MinQuantityShort != nil {
		repoFilter.QuantityShortMin = dtoFilter.MinQuantityShort
	}
	if dtoFilter.MaxQuantityShort != nil {
		repoFilter.QuantityShortMax = dtoFilter.MaxQuantityShort
	}

	// Convert date filters
	if dtoFilter.LastUpdatedFrom != nil {
		repoFilter.LastUpdatedFrom = dtoFilter.LastUpdatedFrom
	}
	if dtoFilter.LastUpdatedTo != nil {
		repoFilter.LastUpdatedTo = dtoFilter.LastUpdatedTo
	}

	// Convert boolean filters
	if dtoFilter.ZeroBalancesOnly != nil && *dtoFilter.ZeroBalancesOnly {
		zero := decimal.Zero
		repoFilter.QuantityLongMin = &zero
		repoFilter.QuantityLongMax = &zero
		repoFilter.QuantityShortMin = &zero
		repoFilter.QuantityShortMax = &zero
	}

	// Convert sorting
	if len(dtoFilter.SortBy) > 0 {
		repoFilter.SortBy = make([]string, len(dtoFilter.SortBy))
		for i, sort := range dtoFilter.SortBy {
			repoFilter.SortBy[i] = fmt.Sprintf("%s %s", sort.Field, sort.Direction)
		}
	}

	return repoFilter
}
