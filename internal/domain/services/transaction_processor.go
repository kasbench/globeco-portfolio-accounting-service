package services

import (
	"context"
	"fmt"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// ProcessingResult represents the result of transaction processing
type ProcessingResult struct {
	TransactionID    int64                     `json:"transactionId"`
	Status           models.TransactionStatus  `json:"status"`
	Success          bool                      `json:"success"`
	ErrorMessage     string                    `json:"errorMessage,omitempty"`
	BalanceChanges   *BalanceCalculationResult `json:"balanceChanges,omitempty"`
	ProcessingTime   time.Duration             `json:"processingTime"`
	ValidationErrors []ValidationError         `json:"validationErrors,omitempty"`
}

// BatchProcessingResult represents the result of batch processing
type BatchProcessingResult struct {
	TotalTransactions   int                         `json:"totalTransactions"`
	SuccessfulProcessed int                         `json:"successfulProcessed"`
	Failed              int                         `json:"failed"`
	Results             map[int64]*ProcessingResult `json:"results"`
	ProcessingTime      time.Duration               `json:"processingTime"`
	Summary             *ProcessingSummary          `json:"summary"`
}

// ProcessingSummary provides a summary of batch processing
type ProcessingSummary struct {
	ByStatus          map[string]int    `json:"byStatus"`
	ByTransactionType map[string]int    `json:"byTransactionType"`
	TotalAmount       map[string]string `json:"totalAmount"` // Currency -> Amount
	ErrorCategories   map[string]int    `json:"errorCategories"`
}

// TransactionProcessor orchestrates transaction processing
type TransactionProcessor struct {
	transactionRepo repositories.TransactionRepository
	balanceRepo     repositories.BalanceRepository
	validator       *TransactionValidator
	calculator      *BalanceCalculator
	logger          logger.Logger
}

// NewTransactionProcessor creates a new transaction processor
func NewTransactionProcessor(
	transactionRepo repositories.TransactionRepository,
	balanceRepo repositories.BalanceRepository,
	validator *TransactionValidator,
	calculator *BalanceCalculator,
	logger logger.Logger,
) *TransactionProcessor {
	return &TransactionProcessor{
		transactionRepo: transactionRepo,
		balanceRepo:     balanceRepo,
		validator:       validator,
		calculator:      calculator,
		logger:          logger,
	}
}

// ProcessTransaction processes a single transaction through the complete workflow
func (p *TransactionProcessor) ProcessTransaction(ctx context.Context, transaction *models.Transaction) (*ProcessingResult, error) {
	startTime := time.Now()

	result := &ProcessingResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Success:       false,
	}

	p.logger.Info("Starting transaction processing",
		logger.Int64("transactionId", transaction.ID()),
		logger.String("sourceId", transaction.SourceID().String()),
		logger.String("type", transaction.TransactionType().String()))

	// Step 1: Validate transaction for processing
	validationResult := p.validator.ValidateTransactionForProcessing(ctx, transaction)
	if !validationResult.IsValid() {
		result.ValidationErrors = validationResult.Errors
		result.ErrorMessage = "Transaction validation failed"
		result.Status = models.TransactionStatusError
		result.ProcessingTime = time.Since(startTime)

		p.logger.Warn("Transaction validation failed",
			logger.Int64("transactionId", transaction.ID()),
			logger.Int("errorCount", len(validationResult.Errors)))

		return result, p.updateTransactionStatus(ctx, transaction, models.TransactionStatusError, &result.ErrorMessage)
	}

	// Step 2: Calculate balance impacts
	balanceResult, err := p.calculator.ApplyTransactionToBalances(ctx, transaction)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to calculate balance impacts: %v", err)
		result.Status = models.TransactionStatusError
		result.ProcessingTime = time.Since(startTime)

		p.logger.Error("Failed to calculate balance impacts",
			logger.Int64("transactionId", transaction.ID()),
			logger.Err(err))

		return result, p.updateTransactionStatus(ctx, transaction, models.TransactionStatusError, &result.ErrorMessage)
	}

	// Step 3: Validate balance constraints
	if err := p.calculator.ValidateBalanceConstraints(ctx, transaction, balanceResult); err != nil {
		result.ErrorMessage = fmt.Sprintf("Balance constraint violation: %v", err)
		result.Status = models.TransactionStatusFatal // Fatal because constraint violations can't be retried
		result.ProcessingTime = time.Since(startTime)

		p.logger.Error("Balance constraint violation",
			logger.Int64("transactionId", transaction.ID()),
			logger.Err(err))

		return result, p.updateTransactionStatus(ctx, transaction, models.TransactionStatusFatal, &result.ErrorMessage)
	}

	// Step 4: Persist balance changes (within a transaction)
	if err := p.persistBalanceChanges(ctx, transaction, balanceResult); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to persist balance changes: %v", err)
		result.Status = models.TransactionStatusError
		result.ProcessingTime = time.Since(startTime)

		p.logger.Error("Failed to persist balance changes",
			logger.Int64("transactionId", transaction.ID()),
			logger.Err(err))

		return result, p.updateTransactionStatus(ctx, transaction, models.TransactionStatusError, &result.ErrorMessage)
	}

	// Step 5: Update transaction status to processed
	if err := p.updateTransactionStatus(ctx, transaction, models.TransactionStatusProc, nil); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to update transaction status: %v", err)
		result.Status = models.TransactionStatusError
		result.ProcessingTime = time.Since(startTime)

		p.logger.Error("Failed to update transaction status",
			logger.Int64("transactionId", transaction.ID()),
			logger.Err(err))

		return result, err
	}

	// Success!
	result.Success = true
	result.Status = models.TransactionStatusProc
	result.BalanceChanges = balanceResult
	result.ProcessingTime = time.Since(startTime)

	p.logger.Info("Transaction processed successfully",
		logger.Int64("transactionId", transaction.ID()),
		logger.String("duration", result.ProcessingTime.String()))

	return result, nil
}

// ProcessTransactionBatch processes multiple transactions
func (p *TransactionProcessor) ProcessTransactionBatch(ctx context.Context, transactions []*models.Transaction) (*BatchProcessingResult, error) {
	startTime := time.Now()

	result := &BatchProcessingResult{
		TotalTransactions: len(transactions),
		Results:           make(map[int64]*ProcessingResult),
		Summary: &ProcessingSummary{
			ByStatus:          make(map[string]int),
			ByTransactionType: make(map[string]int),
			TotalAmount:       make(map[string]string),
			ErrorCategories:   make(map[string]int),
		},
	}

	p.logger.Info("Starting batch processing",
		logger.Int("transactionCount", len(transactions)))

	// Process each transaction
	for _, transaction := range transactions {
		processingResult, err := p.ProcessTransaction(ctx, transaction)
		if err != nil {
			p.logger.Error("Error processing transaction in batch",
				logger.Int64("transactionId", transaction.ID()),
				logger.Err(err))
		}

		if processingResult != nil {
			result.Results[transaction.ID()] = processingResult

			// Update counters
			if processingResult.Success {
				result.SuccessfulProcessed++
			} else {
				result.Failed++
			}

			// Update summary
			p.updateBatchSummary(result.Summary, transaction, processingResult)
		}
	}

	result.ProcessingTime = time.Since(startTime)

	p.logger.Info("Batch processing completed",
		logger.Int("total", result.TotalTransactions),
		logger.Int("successful", result.SuccessfulProcessed),
		logger.Int("failed", result.Failed),
		logger.String("duration", result.ProcessingTime.String()))

	return result, nil
}

// ReprocessFailedTransactions reprocesses transactions in ERROR status
func (p *TransactionProcessor) ReprocessFailedTransactions(ctx context.Context, limit int) (*BatchProcessingResult, error) {
	// Get transactions that can be reprocessed
	filter := repositories.TransactionFilter{
		Statuses: []string{models.TransactionStatusError.String()},
		Limit:    limit,
	}

	transactions, err := p.transactionRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed transactions: %w", err)
	}

	if len(transactions) == 0 {
		p.logger.Info("No failed transactions to reprocess")
		return &BatchProcessingResult{}, nil
	}

	p.logger.Info("Reprocessing failed transactions",
		logger.Int("count", len(transactions)))

	// Convert repository transactions to domain transactions
	domainTransactions := make([]*models.Transaction, 0, len(transactions))
	for _, repoTxn := range transactions {
		domainTxn, err := p.convertToDomainTransaction(repoTxn)
		if err != nil {
			p.logger.Error("Failed to convert transaction",
				logger.Int64("transactionId", repoTxn.ID),
				logger.Err(err))
			continue
		}
		domainTransactions = append(domainTransactions, domainTxn)
	}

	return p.ProcessTransactionBatch(ctx, domainTransactions)
}

// persistBalanceChanges persists the calculated balance changes
func (p *TransactionProcessor) persistBalanceChanges(ctx context.Context, transaction *models.Transaction, balanceResult *BalanceCalculationResult) error {
	// This would ideally be done in a database transaction for atomicity
	// For now, we'll persist changes individually

	// Persist security balance changes
	if balanceResult.SecurityBalance != nil {
		repoBalance := p.convertToRepositoryBalance(balanceResult.SecurityBalance)

		// Check if balance exists
		existing, err := p.balanceRepo.GetByPortfolioAndSecurity(ctx,
			repoBalance.PortfolioID, repoBalance.SecurityID)
		if err != nil && !repositories.IsNotFoundError(err) {
			return fmt.Errorf("failed to check existing security balance: %w", err)
		}

		if existing != nil {
			repoBalance.ID = existing.ID
			repoBalance.Version = existing.Version
			if err := p.balanceRepo.Update(ctx, repoBalance); err != nil {
				return fmt.Errorf("failed to update security balance: %w", err)
			}
		} else {
			if err := p.balanceRepo.Create(ctx, repoBalance); err != nil {
				return fmt.Errorf("failed to create security balance: %w", err)
			}
		}
	}

	// Persist cash balance changes
	if balanceResult.CashBalance != nil {
		repoBalance := p.convertToRepositoryBalance(balanceResult.CashBalance)

		// Check if cash balance exists
		existing, err := p.balanceRepo.GetCashBalance(ctx, repoBalance.PortfolioID)
		if err != nil && !repositories.IsNotFoundError(err) {
			return fmt.Errorf("failed to check existing cash balance: %w", err)
		}

		if existing != nil {
			repoBalance.ID = existing.ID
			repoBalance.Version = existing.Version
			if err := p.balanceRepo.Update(ctx, repoBalance); err != nil {
				return fmt.Errorf("failed to update cash balance: %w", err)
			}
		} else {
			if err := p.balanceRepo.Create(ctx, repoBalance); err != nil {
				return fmt.Errorf("failed to create cash balance: %w", err)
			}
		}
	}

	return nil
}

// updateTransactionStatus updates the transaction status
func (p *TransactionProcessor) updateTransactionStatus(ctx context.Context, transaction *models.Transaction, status models.TransactionStatus, errorMessage *string) error {
	return p.transactionRepo.UpdateStatus(ctx, transaction.ID(), status.String(), errorMessage, transaction.Version())
}

// updateBatchSummary updates the batch processing summary
func (p *TransactionProcessor) updateBatchSummary(summary *ProcessingSummary, transaction *models.Transaction, result *ProcessingResult) {
	// Update status counts
	summary.ByStatus[result.Status.String()]++

	// Update transaction type counts
	summary.ByTransactionType[transaction.TransactionType().String()]++

	// Update error categories if there was an error
	if !result.Success && len(result.ValidationErrors) > 0 {
		for _, validationError := range result.ValidationErrors {
			summary.ErrorCategories[validationError.Code]++
		}
	}

	// TODO: Update total amounts by currency
	// This would require currency information from the transaction or portfolio
}

// convertToDomainTransaction converts repository transaction to domain transaction
func (p *TransactionProcessor) convertToDomainTransaction(repoTxn *repositories.Transaction) (*models.Transaction, error) {
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

	return builder.Build()
}

// convertToRepositoryBalance converts domain balance to repository balance
func (p *TransactionProcessor) convertToRepositoryBalance(domainBalance *models.Balance) *repositories.Balance {
	return &repositories.Balance{
		ID:            domainBalance.ID(),
		PortfolioID:   domainBalance.PortfolioID().String(),
		SecurityID:    domainBalance.SecurityID().Value(),
		QuantityLong:  domainBalance.QuantityLong().Value(),
		QuantityShort: domainBalance.QuantityShort().Value(),
		LastUpdated:   domainBalance.LastUpdated(),
		Version:       domainBalance.Version(),
		CreatedAt:     domainBalance.CreatedAt(),
	}
}

// GetProcessingStats returns processing statistics
func (p *TransactionProcessor) GetProcessingStats(ctx context.Context) (*repositories.TransactionStats, error) {
	return p.transactionRepo.GetTransactionStats(ctx)
}
