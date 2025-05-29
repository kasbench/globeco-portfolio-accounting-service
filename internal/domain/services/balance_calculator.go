package services

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/repositories"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// BalanceCalculationResult represents the result of a balance calculation
type BalanceCalculationResult struct {
	SecurityBalance *models.Balance `json:"securityBalance,omitempty"`
	CashBalance     *models.Balance `json:"cashBalance,omitempty"`
	Success         bool            `json:"success"`
	ErrorMessage    string          `json:"errorMessage,omitempty"`
}

// BalanceImpactSummary provides a summary of how a transaction affects balances
type BalanceImpactSummary struct {
	TransactionID      int64           `json:"transactionId"`
	PortfolioID        string          `json:"portfolioId"`
	SecurityID         *string         `json:"securityId,omitempty"`
	TransactionType    string          `json:"transactionType"`
	Quantity           decimal.Decimal `json:"quantity"`
	Price              decimal.Decimal `json:"price"`
	NotionalAmount     decimal.Decimal `json:"notionalAmount"`
	SecurityImpact     *BalanceChange  `json:"securityImpact,omitempty"`
	CashImpact         *BalanceChange  `json:"cashImpact,omitempty"`
	RequiresNewBalance bool            `json:"requiresNewBalance"`
}

// BalanceChange represents a change to a balance
type BalanceChange struct {
	BalanceType    string          `json:"balanceType"` // "SECURITY" or "CASH"
	LongChange     decimal.Decimal `json:"longChange"`
	ShortChange    decimal.Decimal `json:"shortChange"`
	NetChange      decimal.Decimal `json:"netChange"`
	ResultingLong  decimal.Decimal `json:"resultingLong"`
	ResultingShort decimal.Decimal `json:"resultingShort"`
}

// BalanceCalculator provides balance calculation services
type BalanceCalculator struct {
	balanceRepo repositories.BalanceRepository
	logger      logger.Logger
}

// NewBalanceCalculator creates a new balance calculator
func NewBalanceCalculator(
	balanceRepo repositories.BalanceRepository,
	logger logger.Logger,
) *BalanceCalculator {
	return &BalanceCalculator{
		balanceRepo: balanceRepo,
		logger:      logger,
	}
}

// CalculateBalanceImpact calculates how a transaction will impact balances
func (c *BalanceCalculator) CalculateBalanceImpact(ctx context.Context, transaction *models.Transaction) (*BalanceImpactSummary, error) {
	summary := &BalanceImpactSummary{
		TransactionID:   transaction.ID(),
		PortfolioID:     transaction.PortfolioID().String(),
		TransactionType: transaction.TransactionType().String(),
		Quantity:        transaction.Quantity().Value(),
		Price:           transaction.Price().Value(),
		NotionalAmount:  transaction.CalculateNotionalAmount().Value(),
	}

	if !transaction.SecurityID().IsCash() {
		summary.SecurityID = transaction.SecurityID().Value()
	}

	impact := transaction.GetBalanceImpact()

	// Calculate security balance impact (if applicable)
	if transaction.IsSecurityTransaction() {
		securityImpact, err := c.calculateSecurityImpact(ctx, transaction, impact)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate security impact: %w", err)
		}
		summary.SecurityImpact = securityImpact
	}

	// Calculate cash balance impact (if applicable)
	if impact.Cash != models.ImpactNone {
		cashImpact, err := c.calculateCashImpact(ctx, transaction, impact)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate cash impact: %w", err)
		}
		summary.CashImpact = cashImpact
	}

	return summary, nil
}

// calculateSecurityImpact calculates the impact on security balances
func (c *BalanceCalculator) calculateSecurityImpact(ctx context.Context, transaction *models.Transaction, impact models.BalanceImpact) (*BalanceChange, error) {
	portfolioID := transaction.PortfolioID().String()
	securityID := transaction.SecurityID().Value()

	// Get current security balance
	currentBalance, err := c.balanceRepo.GetByPortfolioAndSecurity(ctx, portfolioID, securityID)
	if err != nil && !repositories.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to get current security balance: %w", err)
	}

	// Calculate changes
	quantity := transaction.Quantity().Value()
	longChange := decimal.Zero
	shortChange := decimal.Zero

	switch impact.LongUnits {
	case models.ImpactIncrease:
		longChange = quantity
	case models.ImpactDecrease:
		longChange = quantity.Neg()
	}

	switch impact.ShortUnits {
	case models.ImpactIncrease:
		shortChange = quantity
	case models.ImpactDecrease:
		shortChange = quantity.Neg()
	}

	// Calculate resulting balances
	resultingLong := decimal.Zero
	resultingShort := decimal.Zero

	if currentBalance != nil {
		resultingLong = currentBalance.QuantityLong.Add(longChange)
		resultingShort = currentBalance.QuantityShort.Add(shortChange)
	} else {
		resultingLong = longChange
		resultingShort = shortChange
	}

	return &BalanceChange{
		BalanceType:    "SECURITY",
		LongChange:     longChange,
		ShortChange:    shortChange,
		NetChange:      longChange.Add(shortChange),
		ResultingLong:  resultingLong,
		ResultingShort: resultingShort,
	}, nil
}

// calculateCashImpact calculates the impact on cash balances
func (c *BalanceCalculator) calculateCashImpact(ctx context.Context, transaction *models.Transaction, impact models.BalanceImpact) (*BalanceChange, error) {
	portfolioID := transaction.PortfolioID().String()

	// Get current cash balance
	currentBalance, err := c.balanceRepo.GetCashBalance(ctx, portfolioID)
	if err != nil && !repositories.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to get current cash balance: %w", err)
	}

	// Calculate cash change based on transaction type
	var cashChange decimal.Decimal

	if transaction.IsCashTransaction() {
		// For cash transactions, use the quantity directly
		cashChange = transaction.Quantity().Value()
		if transaction.TransactionType() == models.TransactionTypeWd {
			cashChange = cashChange.Neg()
		}
	} else {
		// For security transactions, calculate based on notional amount
		notionalAmount := transaction.CalculateNotionalAmount().Value()
		switch impact.Cash {
		case models.ImpactIncrease:
			cashChange = notionalAmount
		case models.ImpactDecrease:
			cashChange = notionalAmount.Neg()
		}
	}

	// Calculate resulting balance
	resultingLong := decimal.Zero
	if currentBalance != nil {
		resultingLong = currentBalance.QuantityLong.Add(cashChange)
	} else {
		resultingLong = cashChange
	}

	return &BalanceChange{
		BalanceType:    "CASH",
		LongChange:     cashChange,
		ShortChange:    decimal.Zero, // Cash never has short positions
		NetChange:      cashChange,
		ResultingLong:  resultingLong,
		ResultingShort: decimal.Zero,
	}, nil
}

// ApplyTransactionToBalances applies a transaction to the relevant balances
func (c *BalanceCalculator) ApplyTransactionToBalances(ctx context.Context, transaction *models.Transaction) (*BalanceCalculationResult, error) {
	result := &BalanceCalculationResult{
		Success: false,
	}

	// Get balance impact
	impact := transaction.GetBalanceImpact()
	portfolioID := transaction.PortfolioID()

	// Handle security balance update
	if transaction.IsSecurityTransaction() {
		securityBalance, err := c.applyToSecurityBalance(ctx, transaction, impact)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("failed to apply to security balance: %v", err)
			return result, err
		}
		result.SecurityBalance = securityBalance
	}

	// Handle cash balance update
	if impact.Cash != models.ImpactNone {
		cashBalance, err := c.applyToCashBalance(ctx, transaction, impact, portfolioID)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("failed to apply to cash balance: %v", err)
			return result, err
		}
		result.CashBalance = cashBalance
	}

	result.Success = true
	return result, nil
}

// applyToSecurityBalance applies transaction impact to security balance
func (c *BalanceCalculator) applyToSecurityBalance(ctx context.Context, transaction *models.Transaction, impact models.BalanceImpact) (*models.Balance, error) {
	portfolioID := transaction.PortfolioID().String()
	securityID := transaction.SecurityID().Value()

	// Get current balance
	currentBalance, err := c.balanceRepo.GetByPortfolioAndSecurity(ctx, portfolioID, securityID)
	if err != nil && !repositories.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to get security balance: %w", err)
	}

	if currentBalance == nil {
		// Create new balance
		return c.createNewSecurityBalance(transaction, impact)
	}

	// Convert repository balance to domain balance for calculation
	domainBalance, err := c.convertToDomainBalance(currentBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to convert balance: %w", err)
	}

	// Apply transaction impact
	updatedBalance, err := domainBalance.ApplyTransactionImpact(transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to apply transaction impact: %w", err)
	}

	return updatedBalance, nil
}

// applyToCashBalance applies transaction impact to cash balance
func (c *BalanceCalculator) applyToCashBalance(ctx context.Context, transaction *models.Transaction, impact models.BalanceImpact, portfolioID models.PortfolioID) (*models.Balance, error) {
	// Get current cash balance
	currentBalance, err := c.balanceRepo.GetCashBalance(ctx, portfolioID.String())
	if err != nil && !repositories.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to get cash balance: %w", err)
	}

	if currentBalance == nil {
		// Create new cash balance
		return c.createNewCashBalance(transaction, impact, portfolioID)
	}

	// Convert repository balance to domain balance for calculation
	domainBalance, err := c.convertToDomainBalance(currentBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cash balance: %w", err)
	}

	// For cash transactions, apply directly to cash balance
	if transaction.IsCashTransaction() {
		return domainBalance.ApplyTransactionImpact(transaction)
	}

	// For security transactions, calculate cash impact
	return c.applyCashImpactFromSecurityTransaction(domainBalance, transaction, impact)
}

// createNewSecurityBalance creates a new security balance from a transaction
func (c *BalanceCalculator) createNewSecurityBalance(transaction *models.Transaction, impact models.BalanceImpact) (*models.Balance, error) {
	quantity := transaction.Quantity().Value()

	longQuantity := decimal.Zero
	shortQuantity := decimal.Zero

	switch impact.LongUnits {
	case models.ImpactIncrease:
		longQuantity = quantity
	case models.ImpactDecrease:
		longQuantity = quantity.Neg()
	}

	switch impact.ShortUnits {
	case models.ImpactIncrease:
		shortQuantity = quantity
	case models.ImpactDecrease:
		shortQuantity = quantity.Neg()
	}

	return models.NewBalanceBuilder().
		WithPortfolioID(transaction.PortfolioID().String()).
		WithSecurityID(transaction.SecurityID().Value()).
		WithQuantityLong(longQuantity).
		WithQuantityShort(shortQuantity).
		Build()
}

// createNewCashBalance creates a new cash balance from a transaction
func (c *BalanceCalculator) createNewCashBalance(transaction *models.Transaction, impact models.BalanceImpact, portfolioID models.PortfolioID) (*models.Balance, error) {
	var cashAmount decimal.Decimal

	if transaction.IsCashTransaction() {
		cashAmount = transaction.Quantity().Value()
		if transaction.TransactionType() == models.TransactionTypeWd {
			cashAmount = cashAmount.Neg()
		}
	} else {
		// Security transaction cash impact
		notionalAmount := transaction.CalculateNotionalAmount().Value()
		switch impact.Cash {
		case models.ImpactIncrease:
			cashAmount = notionalAmount
		case models.ImpactDecrease:
			cashAmount = notionalAmount.Neg()
		}
	}

	return models.NewBalanceBuilder().
		WithPortfolioID(portfolioID.String()).
		WithSecurityID(nil). // Cash balance
		WithQuantityLong(cashAmount).
		WithQuantityShort(decimal.Zero).
		Build()
}

// applyCashImpactFromSecurityTransaction applies cash impact from security transactions
func (c *BalanceCalculator) applyCashImpactFromSecurityTransaction(cashBalance *models.Balance, transaction *models.Transaction, impact models.BalanceImpact) (*models.Balance, error) {
	notionalAmount := transaction.CalculateNotionalAmount().Value()

	var cashChange decimal.Decimal
	switch impact.Cash {
	case models.ImpactIncrease:
		cashChange = notionalAmount
	case models.ImpactDecrease:
		cashChange = notionalAmount.Neg()
	default:
		return cashBalance, nil // No change
	}

	newLongQuantity := models.NewQuantity(cashBalance.QuantityLong().Value().Add(cashChange))

	return cashBalance.UpdateQuantities(newLongQuantity, cashBalance.QuantityShort()), nil
}

// convertToDomainBalance converts repository balance to domain balance
func (c *BalanceCalculator) convertToDomainBalance(repoBalance *repositories.Balance) (*models.Balance, error) {
	builder := models.NewBalanceBuilder().
		WithID(repoBalance.ID).
		WithPortfolioID(repoBalance.PortfolioID).
		WithSecurityID(repoBalance.SecurityID).
		WithQuantityLong(repoBalance.QuantityLong).
		WithQuantityShort(repoBalance.QuantityShort).
		WithVersion(repoBalance.Version).
		WithTimestamps(repoBalance.LastUpdated, repoBalance.CreatedAt)

	return builder.Build()
}

// ValidateBalanceConstraints validates that balance operations don't violate constraints
func (c *BalanceCalculator) ValidateBalanceConstraints(ctx context.Context, transaction *models.Transaction, balanceResult *BalanceCalculationResult) error {
	// Validate security balance constraints
	if balanceResult.SecurityBalance != nil {
		if err := c.validateSecurityBalanceConstraints(balanceResult.SecurityBalance); err != nil {
			return fmt.Errorf("security balance constraint violation: %w", err)
		}
	}

	// Validate cash balance constraints
	if balanceResult.CashBalance != nil {
		if err := c.validateCashBalanceConstraints(balanceResult.CashBalance); err != nil {
			return fmt.Errorf("cash balance constraint violation: %w", err)
		}
	}

	return nil
}

// validateSecurityBalanceConstraints validates security balance business rules
func (c *BalanceCalculator) validateSecurityBalanceConstraints(balance *models.Balance) error {
	// Security balances can have negative positions (short positions)
	// This is generally allowed in portfolio accounting
	return nil
}

// validateCashBalanceConstraints validates cash balance business rules
func (c *BalanceCalculator) validateCashBalanceConstraints(balance *models.Balance) error {
	// Cash balances should not have short positions
	if !balance.QuantityShort().IsZero() {
		return fmt.Errorf("cash balances cannot have short positions")
	}

	// Additional constraint: could check for negative cash if overdrafts are not allowed
	// if balance.QuantityLong().IsNegative() {
	//     return fmt.Errorf("cash balance cannot be negative")
	// }

	return nil
}
