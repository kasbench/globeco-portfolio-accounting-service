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
)

// TransactionService interface defines transaction application service operations
type TransactionService interface {
	// Transaction CRUD operations
	CreateTransaction(ctx context.Context, transactionDTO dto.TransactionPostDTO) (*dto.TransactionResponseDTO, error)
	CreateTransactions(ctx context.Context, transactionDTOs []dto.TransactionPostDTO) (*dto.TransactionBatchResponse, error)
	GetTransaction(ctx context.Context, id int64) (*dto.TransactionResponseDTO, error)
	GetTransactions(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionListResponse, error)

	// Transaction processing operations
	ProcessTransaction(ctx context.Context, id int64) (*dto.TransactionProcessingResult, error)
	ReprocessFailedTransactions(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionBatchResponse, error)

	// Statistics and reporting
	GetTransactionStats(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionStatsDTO, error)

	// Health and monitoring
	GetServiceHealth(ctx context.Context) error
}

// transactionService implements TransactionService interface
type transactionService struct {
	transactionRepo      repositories.TransactionRepository
	balanceRepo          repositories.BalanceRepository
	transactionProcessor services.TransactionProcessor
	validator            services.TransactionValidator
	transactionMapper    *mappers.TransactionMapper
	config               TransactionServiceConfig
	logger               logger.Logger
}

// TransactionServiceConfig holds configuration for transaction service
type TransactionServiceConfig struct {
	MaxBatchSize          int
	ProcessingTimeout     time.Duration
	EnableAsyncProcessing bool
}

// NewTransactionService creates a new transaction application service
func NewTransactionService(
	transactionRepo repositories.TransactionRepository,
	balanceRepo repositories.BalanceRepository,
	transactionProcessor services.TransactionProcessor,
	validator services.TransactionValidator,
	transactionMapper *mappers.TransactionMapper,
	config TransactionServiceConfig,
	lg logger.Logger,
) TransactionService {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Set default configuration
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 1000
	}
	if config.ProcessingTimeout == 0 {
		config.ProcessingTimeout = 30 * time.Second
	}

	return &transactionService{
		transactionRepo:      transactionRepo,
		balanceRepo:          balanceRepo,
		transactionProcessor: transactionProcessor,
		validator:            validator,
		transactionMapper:    transactionMapper,
		config:               config,
		logger:               lg,
	}
}

// CreateTransaction creates a single transaction
func (s *transactionService) CreateTransaction(ctx context.Context, transactionDTO dto.TransactionPostDTO) (*dto.TransactionResponseDTO, error) {
	s.logger.Info("Creating single transaction",
		logger.String("sourceId", transactionDTO.SourceID))

	// Validate DTO
	validationErrors := s.transactionMapper.ValidatePostDTO(&transactionDTO)
	if len(validationErrors) > 0 {
		s.logger.Warn("Transaction DTO validation failed",
			logger.Int("errorCount", len(validationErrors)),
			logger.String("sourceId", transactionDTO.SourceID))
		return nil, fmt.Errorf("validation failed: %s", validationErrors[0].Message)
	}

	// Convert DTO to domain model
	domainTransaction, err := s.transactionMapper.FromPostDTO(&transactionDTO)
	if err != nil {
		s.logger.Error("Failed to convert DTO to domain model",
			logger.Err(err),
			logger.String("sourceId", transactionDTO.SourceID))
		return nil, fmt.Errorf("failed to convert transaction: %w", err)
	}

	// Validate business rules
	validationResult := s.validator.ValidateTransaction(ctx, domainTransaction)
	if !validationResult.IsValid() {
		s.logger.Warn("Transaction business validation failed",
			logger.Int("errorCount", len(validationResult.Errors)),
			logger.String("sourceId", transactionDTO.SourceID))
		return nil, fmt.Errorf("business validation failed: %s", validationResult.Errors[0].Message)
	}

	// Convert domain transaction to repository transaction for persistence
	repoTransaction := s.convertDomainToRepo(domainTransaction)

	// Create transaction in repository with status NEW
	err = s.transactionRepo.Create(ctx, repoTransaction)
	if err != nil {
		s.logger.Error("Failed to create transaction in repository",
			logger.Err(err),
			logger.String("sourceId", transactionDTO.SourceID))
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	s.logger.Info("Transaction created successfully",
		logger.Int64("transactionId", repoTransaction.ID),
		logger.String("sourceId", transactionDTO.SourceID))

	// Convert back to domain transaction with ID for processing
	domainTransactionWithID := s.convertRepoToDomain(repoTransaction)

	// STEP 2: Process transaction to update balances and set status to PROC
	// This implements the required business workflow from requirements
	processingResult, err := s.transactionProcessor.ProcessTransaction(ctx, domainTransactionWithID)
	if err != nil {
		s.logger.Error("Failed to process transaction after creation",
			logger.Err(err),
			logger.Int64("transactionId", repoTransaction.ID),
			logger.String("sourceId", transactionDTO.SourceID))
		return nil, fmt.Errorf("transaction created but processing failed: %w", err)
	}

	// Check if processing was successful
	if processingResult != nil && !processingResult.Success {
		s.logger.Warn("Transaction processing failed after creation",
			logger.Int64("transactionId", repoTransaction.ID),
			logger.String("sourceId", transactionDTO.SourceID),
			logger.String("error", processingResult.ErrorMessage))
		return nil, fmt.Errorf("transaction processing failed: %s", processingResult.ErrorMessage)
	}

	// Get the updated transaction with PROC status
	updatedRepoTransaction, err := s.transactionRepo.GetByID(ctx, repoTransaction.ID)
	if err != nil {
		s.logger.Warn("Failed to retrieve processed transaction, using original",
			logger.Err(err),
			logger.Int64("transactionId", repoTransaction.ID))
		// Return original transaction even if we can't retrieve updated version
		return s.transactionMapper.ToResponseDTO(domainTransactionWithID), nil
	}

	// Use the updated transaction with PROC status
	processedDomainTransaction := s.convertRepoToDomain(updatedRepoTransaction)

	s.logger.Info("Transaction created and processed successfully",
		logger.Int64("transactionId", repoTransaction.ID),
		logger.String("sourceId", transactionDTO.SourceID),
		logger.String("status", "PROC"))

	return s.transactionMapper.ToResponseDTO(processedDomainTransaction), nil
}

// CreateTransactions creates multiple transactions in a batch
func (s *transactionService) CreateTransactions(ctx context.Context, transactionDTOs []dto.TransactionPostDTO) (*dto.TransactionBatchResponse, error) {
	s.logger.Info("Creating batch of transactions",
		logger.Int("count", len(transactionDTOs)))

	if len(transactionDTOs) == 0 {
		return &dto.TransactionBatchResponse{
			Successful: []dto.TransactionResponseDTO{},
			Failed:     []dto.TransactionErrorDTO{},
			Summary: dto.BatchSummaryDTO{
				TotalRequested: 0,
				Successful:     0,
				Failed:         0,
				SuccessRate:    0,
			},
		}, nil
	}

	var successful []*models.Transaction
	var failed []dto.TransactionErrorDTO

	// Process each transaction
	for i, transactionDTO := range transactionDTOs {
		// Validate DTO
		validationErrors := s.transactionMapper.ValidatePostDTO(&transactionDTO)
		if len(validationErrors) > 0 {
			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors:      validationErrors,
			})
			continue
		}

		// Convert DTO to domain model
		domainTransaction, err := s.transactionMapper.FromPostDTO(&transactionDTO)
		if err != nil {
			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors: []dto.ValidationError{{
					Field:   "transaction",
					Message: err.Error(),
					Value:   fmt.Sprintf("index_%d", i),
				}},
			})
			continue
		}

		// Validate business rules
		validationResult := s.validator.ValidateTransaction(ctx, domainTransaction)
		if !validationResult.IsValid() {
			var errors []dto.ValidationError
			for _, validationError := range validationResult.Errors {
				errors = append(errors, dto.ValidationError{
					Field:   validationError.Field,
					Message: validationError.Message,
					Value:   fmt.Sprintf("%v", validationError.Value),
				})
			}
			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors:      errors,
			})
			continue
		}

		// Convert domain transaction to repository transaction
		repoTransaction := s.convertDomainToRepo(domainTransaction)

		// Create transaction in repository with status NEW
		err = s.transactionRepo.Create(ctx, repoTransaction)
		if err != nil {
			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors: []dto.ValidationError{{
					Field:   "repository",
					Message: err.Error(),
					Value:   fmt.Sprintf("index_%d", i),
				}},
			})
			continue
		}

		// Convert back to domain transaction with ID for processing
		domainTransactionWithID := s.convertRepoToDomain(repoTransaction)

		// STEP 2: Process transaction to update balances and set status to PROC
		// This implements the required business workflow from requirements
		processingResult, err := s.transactionProcessor.ProcessTransaction(ctx, domainTransactionWithID)
		if err != nil {
			s.logger.Error("Failed to process transaction after creation",
				logger.Err(err),
				logger.Int64("transactionId", repoTransaction.ID),
				logger.String("sourceId", transactionDTO.SourceID))

			// Transaction was created but processing failed - mark as ERROR
			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors: []dto.ValidationError{{
					Field:   "processing",
					Message: fmt.Sprintf("balance processing failed: %v", err),
					Value:   fmt.Sprintf("index_%d", i),
				}},
			})
			continue
		}

		// Check if processing was successful
		if processingResult != nil && !processingResult.Success {
			s.logger.Warn("Transaction processing failed after creation",
				logger.Int64("transactionId", repoTransaction.ID),
				logger.String("sourceId", transactionDTO.SourceID),
				logger.String("error", processingResult.ErrorMessage))

			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: transactionDTO,
				Errors: []dto.ValidationError{{
					Field:   "processing",
					Message: processingResult.ErrorMessage,
					Value:   fmt.Sprintf("index_%d", i),
				}},
			})
			continue
		}

		// Get the updated transaction with PROC status
		updatedRepoTransaction, err := s.transactionRepo.GetByID(ctx, repoTransaction.ID)
		if err != nil {
			s.logger.Error("Failed to retrieve processed transaction",
				logger.Err(err),
				logger.Int64("transactionId", repoTransaction.ID))
			// Continue with original transaction even if we can't retrieve updated version
			successful = append(successful, domainTransactionWithID)
		} else {
			// Use the updated transaction with PROC status
			successful = append(successful, s.convertRepoToDomain(updatedRepoTransaction))
		}

		s.logger.Info("Transaction created and processed successfully",
			logger.Int64("transactionId", repoTransaction.ID),
			logger.String("sourceId", transactionDTO.SourceID),
			logger.String("status", "PROC"))
	}

	s.logger.Info("Batch transaction creation and processing completed",
		logger.Int("successful", len(successful)),
		logger.Int("failed", len(failed)),
		logger.Int("total", len(transactionDTOs)))

	batchResponse := s.transactionMapper.ToBatchResponse(successful, failed)
	return &batchResponse, nil
}

// GetTransaction retrieves a transaction by ID
func (s *transactionService) GetTransaction(ctx context.Context, id int64) (*dto.TransactionResponseDTO, error) {
	s.logger.Debug("Retrieving transaction",
		logger.Int64("transactionId", id))

	repoTransaction, err := s.transactionRepo.GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			s.logger.Warn("Transaction not found",
				logger.Int64("transactionId", id))
			return nil, fmt.Errorf("transaction not found: %d", id)
		}
		s.logger.Error("Failed to retrieve transaction",
			logger.Err(err),
			logger.Int64("transactionId", id))
		return nil, fmt.Errorf("failed to retrieve transaction: %w", err)
	}

	domainTransaction := s.convertRepoToDomain(repoTransaction)
	return s.transactionMapper.ToResponseDTO(domainTransaction), nil
}

// GetTransactions retrieves transactions with filtering and pagination
func (s *transactionService) GetTransactions(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionListResponse, error) {
	s.logger.Debug("Retrieving transactions with filter",
		logger.Int("limit", filter.Pagination.Limit),
		logger.Int("offset", filter.Pagination.Offset))

	// Validate filter
	if !filter.IsValid() {
		return nil, fmt.Errorf("invalid filter parameters")
	}

	// Convert DTO filter to repository filter
	repoFilter := s.convertDTOFilterToRepo(filter)

	// Set default pagination if not provided
	if repoFilter.Limit == 0 {
		repoFilter.Limit = 50
	}
	if repoFilter.Limit > 1000 {
		repoFilter.Limit = 1000
	}

	// Get transactions from repository
	repoTransactions, err := s.transactionRepo.List(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to retrieve transactions",
			logger.Err(err))
		return nil, fmt.Errorf("failed to retrieve transactions: %w", err)
	}

	// Get total count for pagination
	totalCount, err := s.transactionRepo.Count(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count transactions",
			logger.Err(err))
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	s.logger.Debug("Retrieved transactions",
		logger.Int("count", len(repoTransactions)),
		logger.Int64("total", totalCount))

	// Convert repository transactions to domain transactions
	domainTransactions := make([]*models.Transaction, len(repoTransactions))
	for i, repoTxn := range repoTransactions {
		domainTransactions[i] = s.convertRepoToDomain(repoTxn)
	}

	return &dto.TransactionListResponse{
		Transactions: s.transactionMapper.ToResponseDTOs(domainTransactions),
		Pagination: dto.NewPaginationResponse(
			repoFilter.Limit,
			repoFilter.Offset,
			totalCount,
		),
	}, nil
}

// ProcessTransaction processes a single transaction
func (s *transactionService) ProcessTransaction(ctx context.Context, id int64) (*dto.TransactionProcessingResult, error) {
	s.logger.Info("Processing transaction",
		logger.Int64("transactionId", id))

	// Get transaction from repository
	repoTransaction, err := s.transactionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Convert to domain transaction
	domainTransaction := s.convertRepoToDomain(repoTransaction)

	// Check if transaction can be processed
	if !domainTransaction.CanBeProcessed() {
		s.logger.Warn("Transaction cannot be processed",
			logger.Int64("transactionId", id),
			logger.String("status", string(domainTransaction.Status())))
		return &dto.TransactionProcessingResult{
			TransactionID:  id,
			Status:         string(domainTransaction.Status()),
			ProcessedAt:    time.Now(),
			BalanceUpdated: false,
			ErrorMessage:   stringPtr("transaction cannot be processed in current status"),
		}, nil
	}

	// Process transaction
	processingResult, err := s.transactionProcessor.ProcessTransaction(ctx, domainTransaction)
	if err != nil {
		s.logger.Error("Failed to process transaction",
			logger.Err(err),
			logger.Int64("transactionId", id))

		errorMsg := err.Error()
		return &dto.TransactionProcessingResult{
			TransactionID:  id,
			Status:         "ERROR",
			ProcessedAt:    time.Now(),
			BalanceUpdated: false,
			ErrorMessage:   &errorMsg,
		}, nil
	}

	// Check processing result
	if processingResult != nil && !processingResult.Success {
		s.logger.Warn("Transaction processing failed",
			logger.Int64("transactionId", id),
			logger.String("error", processingResult.ErrorMessage))

		return &dto.TransactionProcessingResult{
			TransactionID:  id,
			Status:         string(processingResult.Status),
			ProcessedAt:    time.Now(),
			BalanceUpdated: false,
			ErrorMessage:   &processingResult.ErrorMessage,
		}, nil
	}

	s.logger.Info("Transaction processed successfully",
		logger.Int64("transactionId", id))

	return &dto.TransactionProcessingResult{
		TransactionID:  id,
		Status:         "PROC",
		ProcessedAt:    time.Now(),
		BalanceUpdated: true,
		ErrorMessage:   nil,
	}, nil
}

// ReprocessFailedTransactions reprocesses failed transactions
func (s *transactionService) ReprocessFailedTransactions(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionBatchResponse, error) {
	s.logger.Info("Reprocessing failed transactions")

	// Create filter for failed transactions
	repoFilter := repositories.TransactionFilter{
		Statuses: []string{"ERROR"},
		Limit:    filter.Pagination.Limit,
		Offset:   filter.Pagination.Offset,
	}

	if repoFilter.Limit == 0 {
		repoFilter.Limit = 100
	}

	repoTransactions, err := s.transactionRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to find failed transactions: %w", err)
	}

	var successful []*models.Transaction
	var failed []dto.TransactionErrorDTO

	// Process each failed transaction
	for _, repoTransaction := range repoTransactions {
		domainTransaction := s.convertRepoToDomain(repoTransaction)

		processingResult, err := s.transactionProcessor.ProcessTransaction(ctx, domainTransaction)
		if err != nil || (processingResult != nil && !processingResult.Success) {
			// Convert transaction to DTO for error response
			postDTO := dto.TransactionPostDTO{
				PortfolioID:     repoTransaction.PortfolioID,
				SourceID:        repoTransaction.SourceID,
				TransactionType: repoTransaction.TransactionType,
				Quantity:        repoTransaction.Quantity,
				Price:           repoTransaction.Price,
				TransactionDate: repoTransaction.TransactionDate.Format("20060102"),
			}
			if repoTransaction.SecurityID != nil {
				postDTO.SecurityID = repoTransaction.SecurityID
			}

			errorMessage := "processing failed"
			if err != nil {
				errorMessage = err.Error()
			} else if processingResult != nil {
				errorMessage = processingResult.ErrorMessage
			}

			failed = append(failed, dto.TransactionErrorDTO{
				Transaction: postDTO,
				Errors: []dto.ValidationError{{
					Field:   "processing",
					Message: errorMessage,
					Value:   fmt.Sprintf("transaction_%d", repoTransaction.ID),
				}},
			})
		} else {
			successful = append(successful, domainTransaction)
		}
	}

	s.logger.Info("Reprocessing completed",
		logger.Int("successful", len(successful)),
		logger.Int("failed", len(failed)))

	batchResponse := s.transactionMapper.ToBatchResponse(successful, failed)
	return &batchResponse, nil
}

// GetTransactionStats retrieves transaction statistics
func (s *transactionService) GetTransactionStats(ctx context.Context, filter dto.TransactionFilter) (*dto.TransactionStatsDTO, error) {
	s.logger.Debug("Retrieving transaction statistics")

	stats, err := s.transactionRepo.GetTransactionStats(ctx)
	if err != nil {
		s.logger.Error("Failed to retrieve transaction statistics",
			logger.Err(err))
		return nil, fmt.Errorf("failed to retrieve statistics: %w", err)
	}

	// Convert to DTO
	statsDTO := &dto.TransactionStatsDTO{
		TotalCount:     stats.TotalCount,
		StatusCounts:   stats.StatusCounts,
		TypeCounts:     stats.TypeCounts,
		PortfolioCount: 0, // Would need additional query to calculate unique portfolios
	}

	return statsDTO, nil
}

// GetServiceHealth checks the health of the transaction service
func (s *transactionService) GetServiceHealth(ctx context.Context) error {
	s.logger.Debug("Checking transaction service health")

	// For now, we'll do a simple count query to check database connectivity
	_, err := s.transactionRepo.Count(ctx, repositories.TransactionFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("transaction repository health check failed: %w", err)
	}

	// Check balance repository as well
	_, err = s.balanceRepo.Count(ctx, repositories.BalanceFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("balance repository health check failed: %w", err)
	}

	return nil
}

// Helper functions

// convertDomainToRepo converts a domain transaction to repository transaction
func (s *transactionService) convertDomainToRepo(domainTxn *models.Transaction) *repositories.Transaction {
	repoTxn := &repositories.Transaction{
		ID:                   domainTxn.ID(),
		PortfolioID:          domainTxn.PortfolioID().String(),
		SourceID:             domainTxn.SourceID().String(),
		Status:               string(domainTxn.Status()),
		TransactionType:      string(domainTxn.TransactionType()),
		Quantity:             domainTxn.Quantity().Value(),
		Price:                domainTxn.Price().Value(),
		TransactionDate:      domainTxn.TransactionDate(),
		ReprocessingAttempts: domainTxn.ReprocessingAttempts(),
		Version:              domainTxn.Version(),
		CreatedAt:            domainTxn.CreatedAt(),
		UpdatedAt:            domainTxn.UpdatedAt(),
		ErrorMessage:         domainTxn.ErrorMessage(),
	}

	// Handle optional security ID
	if !domainTxn.SecurityID().IsEmpty() {
		securityID := domainTxn.SecurityID().String()
		repoTxn.SecurityID = &securityID
	}

	return repoTxn
}

// convertRepoToDomain converts a repository transaction to domain transaction
func (s *transactionService) convertRepoToDomain(repoTxn *repositories.Transaction) *models.Transaction {
	builder := models.NewTransactionBuilder().
		WithID(repoTxn.ID).
		WithPortfolioID(repoTxn.PortfolioID).
		WithSecurityID(repoTxn.SecurityID).
		WithSourceID(repoTxn.SourceID).
		WithTransactionType(repoTxn.TransactionType).
		WithStatus(repoTxn.Status).
		WithQuantity(repoTxn.Quantity).
		WithPrice(repoTxn.Price).
		WithTransactionDate(repoTxn.TransactionDate).
		WithReprocessingAttempts(repoTxn.ReprocessingAttempts).
		WithVersion(repoTxn.Version).
		WithTimestamps(repoTxn.CreatedAt, repoTxn.UpdatedAt)

	if repoTxn.ErrorMessage != nil {
		builder.WithErrorMessage(*repoTxn.ErrorMessage)
	}

	// Build should not fail for valid repository data
	domainTxn, err := builder.Build()
	if err != nil {
		s.logger.Error("Failed to convert repository transaction to domain model",
			logger.Int64("transactionId", repoTxn.ID),
			logger.Err(err))
		// Return a minimal transaction in case of error
		return &models.Transaction{}
	}

	return domainTxn
}

// convertDTOFilterToRepo converts DTO filter to repository filter
func (s *transactionService) convertDTOFilterToRepo(dtoFilter dto.TransactionFilter) repositories.TransactionFilter {
	repoFilter := repositories.TransactionFilter{
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
	if dtoFilter.Status != nil {
		repoFilter.Status = dtoFilter.Status
	}
	if dtoFilter.TransactionType != nil {
		repoFilter.TransactionType = dtoFilter.TransactionType
	}

	// Convert slice filters
	if len(dtoFilter.PortfolioIDs) > 0 {
		repoFilter.PortfolioIDs = dtoFilter.PortfolioIDs
	}
	if len(dtoFilter.SecurityIDs) > 0 {
		repoFilter.SecurityIDs = dtoFilter.SecurityIDs
	}
	if len(dtoFilter.Statuses) > 0 {
		repoFilter.Statuses = dtoFilter.Statuses
	}
	if len(dtoFilter.TransactionTypes) > 0 {
		repoFilter.TransactionTypes = dtoFilter.TransactionTypes
	}

	// Convert date filters
	if dtoFilter.TransactionDate != nil {
		repoFilter.TransactionDate = dtoFilter.TransactionDate
	}
	if dtoFilter.TransactionDateFrom != nil {
		repoFilter.TransactionDateFrom = dtoFilter.TransactionDateFrom
	}
	if dtoFilter.TransactionDateTo != nil {
		repoFilter.TransactionDateTo = dtoFilter.TransactionDateTo
	}

	// Convert amount filters
	if dtoFilter.MinQuantity != nil {
		repoFilter.QuantityMin = dtoFilter.MinQuantity
	}
	if dtoFilter.MaxQuantity != nil {
		repoFilter.QuantityMax = dtoFilter.MaxQuantity
	}
	if dtoFilter.MinPrice != nil {
		repoFilter.PriceMin = dtoFilter.MinPrice
	}
	if dtoFilter.MaxPrice != nil {
		repoFilter.PriceMax = dtoFilter.MaxPrice
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

// stringPtr creates a string pointer
func stringPtr(s string) *string {
	return &s
}
