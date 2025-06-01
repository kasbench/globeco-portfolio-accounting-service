package services

import (
	"context"
	"fmt"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value"`
	Message string      `json:"message"`
	Code    string      `json:"code"`
}

// Error implements the error interface
func (v ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", v.Field, v.Message)
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// IsValid returns true if the validation passed
func (r ValidationResult) IsValid() bool {
	return r.Valid && len(r.Errors) == 0
}

// HasErrors returns true if there are validation errors
func (r ValidationResult) HasErrors() bool {
	return !r.Valid || len(r.Errors) > 0
}

// FirstError returns the first validation error, if any
func (r ValidationResult) FirstError() *ValidationError {
	if len(r.Errors) > 0 {
		return &r.Errors[0]
	}
	return nil
}

// TransactionValidator provides validation services for transactions
type TransactionValidator struct {
	transactionRepo repositories.TransactionRepository
	balanceRepo     repositories.BalanceRepository
	logger          logger.Logger
}

// NewTransactionValidator creates a new transaction validator
func NewTransactionValidator(
	transactionRepo repositories.TransactionRepository,
	balanceRepo repositories.BalanceRepository,
	logger logger.Logger,
) *TransactionValidator {
	return &TransactionValidator{
		transactionRepo: transactionRepo,
		balanceRepo:     balanceRepo,
		logger:          logger,
	}
}

// ValidateTransaction performs comprehensive validation of a transaction
func (v *TransactionValidator) ValidateTransaction(ctx context.Context, transaction *models.Transaction) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Basic field validation
	if errs := v.validateBasicFields(transaction); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	// Business rule validation
	if errs := v.validateBusinessRules(ctx, transaction); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	// Portfolio validation
	if errs := v.validatePortfolio(ctx, transaction); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	// Security validation for non-cash transactions
	if !transaction.IsCashTransaction() {
		if errs := v.validateSecurity(ctx, transaction); len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
		}
	}

	// Source ID uniqueness validation
	if errs := v.validateSourceIDUniqueness(ctx, transaction); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	if !result.Valid {
		v.logger.Warn("Transaction validation failed",
			logger.String("transactionType", transaction.TransactionType().String()),
			logger.String("portfolioId", transaction.PortfolioID().String()),
			logger.Int("errorCount", len(result.Errors)))
	}

	return result
}

// ValidateTransactionBatch validates multiple transactions
func (v *TransactionValidator) ValidateTransactionBatch(ctx context.Context, transactions []*models.Transaction) map[int]ValidationResult {
	results := make(map[int]ValidationResult)

	for i, transaction := range transactions {
		results[i] = v.ValidateTransaction(ctx, transaction)
	}

	return results
}

// validateBasicFields validates basic transaction fields
func (v *TransactionValidator) validateBasicFields(transaction *models.Transaction) []ValidationError {
	var errors []ValidationError

	// Portfolio ID validation
	if transaction.PortfolioID().IsEmpty() {
		errors = append(errors, ValidationError{
			Field:   "portfolioId",
			Value:   transaction.PortfolioID().String(),
			Message: "portfolio ID is required",
			Code:    "REQUIRED",
		})
	}

	// Source ID validation
	if transaction.SourceID().IsEmpty() {
		errors = append(errors, ValidationError{
			Field:   "sourceId",
			Value:   transaction.SourceID().String(),
			Message: "source ID is required",
			Code:    "REQUIRED",
		})
	}

	// Transaction type validation
	if !transaction.TransactionType().IsValid() {
		errors = append(errors, ValidationError{
			Field:   "transactionType",
			Value:   transaction.TransactionType().String(),
			Message: "invalid transaction type",
			Code:    "INVALID_TYPE",
		})
	}

	// Quantity validation
	if transaction.Quantity().IsZero() {
		errors = append(errors, ValidationError{
			Field:   "quantity",
			Value:   transaction.Quantity().String(),
			Message: "quantity cannot be zero",
			Code:    "INVALID_VALUE",
		})
	}

	// Price validation
	if transaction.Price().IsNegative() {
		errors = append(errors, ValidationError{
			Field:   "price",
			Value:   transaction.Price().String(),
			Message: "price cannot be negative",
			Code:    "INVALID_VALUE",
		})
	}

	return errors
}

// validateBusinessRules validates transaction-specific business rules
func (v *TransactionValidator) validateBusinessRules(ctx context.Context, transaction *models.Transaction) []ValidationError {
	var errors []ValidationError

	transactionType := transaction.TransactionType()

	// Cash transaction rules
	if transactionType.IsCashTransaction() {
		// Cash transactions must have nil security ID
		if !transaction.SecurityID().IsCash() {
			errors = append(errors, ValidationError{
				Field:   "securityId",
				Value:   transaction.SecurityID().String(),
				Message: "cash transactions must have empty security ID",
				Code:    "INVALID_CASH_TRANSACTION",
			})
		}

		// Cash transactions must have price of 1.0
		if !transaction.Price().Equals(models.CashPrice().Amount) {
			errors = append(errors, ValidationError{
				Field:   "price",
				Value:   transaction.Price().String(),
				Message: "cash transactions must have price of 1.0",
				Code:    "INVALID_CASH_PRICE",
			})
		}
	}

	// Security transaction rules
	if transactionType.IsSecurityTransaction() {
		// Security transactions must have a valid security ID
		if transaction.SecurityID().IsCash() {
			errors = append(errors, ValidationError{
				Field:   "securityId",
				Value:   "null",
				Message: "security transactions require a valid security ID",
				Code:    "MISSING_SECURITY_ID",
			})
		}

		// Security transactions must have positive price
		if !transaction.Price().IsPositive() {
			errors = append(errors, ValidationError{
				Field:   "price",
				Value:   transaction.Price().String(),
				Message: "security transactions must have positive price",
				Code:    "INVALID_SECURITY_PRICE",
			})
		}
	}

	return errors
}

// validatePortfolio validates that the portfolio exists and is accessible
func (v *TransactionValidator) validatePortfolio(ctx context.Context, transaction *models.Transaction) []ValidationError {
	var errors []ValidationError

	// In a real implementation, you would call an external portfolio service
	// For now, we'll just validate the format
	portfolioID := transaction.PortfolioID().String()
	if len(portfolioID) != 24 {
		errors = append(errors, ValidationError{
			Field:   "portfolioId",
			Value:   portfolioID,
			Message: "portfolio ID must be exactly 24 characters",
			Code:    "INVALID_FORMAT",
		})
	}

	// TODO: Add call to portfolio service to verify portfolio exists
	// This would be implemented when we have the external service client

	return errors
}

// validateSecurity validates that the security exists and is valid
func (v *TransactionValidator) validateSecurity(ctx context.Context, transaction *models.Transaction) []ValidationError {
	var errors []ValidationError

	securityID := transaction.SecurityID().String()
	if len(securityID) != 24 {
		errors = append(errors, ValidationError{
			Field:   "securityId",
			Value:   securityID,
			Message: "security ID must be exactly 24 characters",
			Code:    "INVALID_FORMAT",
		})
	}

	// TODO: Add call to security service to verify security exists
	// This would be implemented when we have the external service client

	return errors
}

// validateSourceIDUniqueness ensures the source ID is unique
func (v *TransactionValidator) validateSourceIDUniqueness(ctx context.Context, transaction *models.Transaction) []ValidationError {
	var errors []ValidationError

	// Check if a transaction with this source ID already exists
	sourceID := transaction.SourceID().String()
	existingTransaction, err := v.transactionRepo.GetBySourceID(ctx, sourceID)

	if err != nil && !repositories.IsNotFoundError(err) {
		// Only log actual errors, not "not found" which is the expected case for unique source IDs
		v.logger.Error("Failed to check source ID uniqueness",
			logger.String("sourceId", sourceID),
			logger.Err(err))
		return errors
	}

	if existingTransaction != nil {
		// If this is an update (transaction has an ID), allow the same source ID
		if transaction.ID() == 0 || existingTransaction.ID != transaction.ID() {
			errors = append(errors, ValidationError{
				Field:   "sourceId",
				Value:   sourceID,
				Message: "source ID must be unique",
				Code:    "DUPLICATE_SOURCE_ID",
			})
		}
	}

	// Log successful uniqueness validation at debug level
	v.logger.Debug("Source ID uniqueness validated",
		logger.String("sourceId", sourceID),
		logger.Bool("unique", existingTransaction == nil))

	return errors
}

// ValidateTransactionUpdate validates an update to an existing transaction
func (v *TransactionValidator) ValidateTransactionUpdate(ctx context.Context, transaction *models.Transaction) ValidationResult {
	result := v.ValidateTransaction(ctx, transaction)

	// Additional validations for updates
	if transaction.ID() <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "id",
			Value:   transaction.ID(),
			Message: "transaction ID is required for updates",
			Code:    "MISSING_ID",
		})
		result.Valid = false
	}

	// Check if transaction is in a state that allows updates
	if transaction.Status().IsFinalState() && transaction.Status() != models.TransactionStatusError {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "status",
			Value:   transaction.Status().String(),
			Message: "cannot update transaction in final state",
			Code:    "INVALID_STATE",
		})
		result.Valid = false
	}

	return result
}

// ValidateTransactionForProcessing validates that a transaction can be processed
func (v *TransactionValidator) ValidateTransactionForProcessing(ctx context.Context, transaction *models.Transaction) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Transaction must be in processable state
	if !transaction.CanBeProcessed() {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "status",
			Value:   transaction.Status().String(),
			Message: "transaction cannot be processed in current state",
			Code:    "INVALID_PROCESSING_STATE",
		})
	}

	// Check if transaction has exceeded retry limits
	maxRetries := 3 // This could be configurable
	if transaction.ReprocessingAttempts() >= maxRetries {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "reprocessingAttempts",
			Value:   transaction.ReprocessingAttempts(),
			Message: fmt.Sprintf("transaction has exceeded maximum retry attempts (%d)", maxRetries),
			Code:    "MAX_RETRIES_EXCEEDED",
		})
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateBalanceOperation validates a balance operation
func (v *TransactionValidator) ValidateBalanceOperation(ctx context.Context, transaction *models.Transaction, currentBalance *models.Balance) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	if currentBalance == nil {
		// New balance - no specific validations needed
		return result
	}

	// Validate that the transaction can be applied to this balance
	if !currentBalance.PortfolioID().Equals(transaction.PortfolioID()) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "portfolioId",
			Value:   transaction.PortfolioID().String(),
			Message: "transaction portfolio does not match balance portfolio",
			Code:    "PORTFOLIO_MISMATCH",
		})
	}

	// For security transactions, ensure security IDs match
	if transaction.IsSecurityTransaction() && !currentBalance.SecurityID().Equals(transaction.SecurityID()) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "securityId",
			Value:   transaction.SecurityID().String(),
			Message: "transaction security does not match balance security",
			Code:    "SECURITY_MISMATCH",
		})
	}

	// For cash transactions, ensure we're updating a cash balance
	if transaction.IsCashTransaction() && !currentBalance.IsCashBalance() {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "securityId",
			Value:   currentBalance.SecurityID().String(),
			Message: "cash transaction cannot be applied to security balance",
			Code:    "BALANCE_TYPE_MISMATCH",
		})
	}

	result.Valid = len(result.Errors) == 0
	return result
}
