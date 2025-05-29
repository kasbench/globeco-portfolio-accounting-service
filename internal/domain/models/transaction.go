package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Transaction represents a portfolio transaction domain entity
type Transaction struct {
	id                   int64
	portfolioID          PortfolioID
	securityID           SecurityID
	sourceID             SourceID
	status               TransactionStatus
	transactionType      TransactionType
	quantity             Quantity
	price                Price
	transactionDate      time.Time
	reprocessingAttempts int
	version              int
	createdAt            time.Time
	updatedAt            time.Time
	errorMessage         *string
}

// TransactionBuilder helps build Transaction entities with validation
type TransactionBuilder struct {
	transaction Transaction
	errors      []error
}

// NewTransactionBuilder creates a new transaction builder
func NewTransactionBuilder() *TransactionBuilder {
	return &TransactionBuilder{
		transaction: Transaction{
			status:          TransactionStatusNew,
			transactionDate: time.Now().UTC().Truncate(24 * time.Hour), // Default to today
			version:         1,
			createdAt:       time.Now().UTC(),
			updatedAt:       time.Now().UTC(),
		},
	}
}

// WithID sets the transaction ID
func (b *TransactionBuilder) WithID(id int64) *TransactionBuilder {
	b.transaction.id = id
	return b
}

// WithPortfolioID sets the portfolio ID
func (b *TransactionBuilder) WithPortfolioID(portfolioID string) *TransactionBuilder {
	pid, err := NewPortfolioID(portfolioID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid portfolio ID: %w", err))
		return b
	}
	b.transaction.portfolioID = pid
	return b
}

// WithSecurityID sets the security ID
func (b *TransactionBuilder) WithSecurityID(securityID *string) *TransactionBuilder {
	sid, err := NewSecurityID(securityID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid security ID: %w", err))
		return b
	}
	b.transaction.securityID = sid
	return b
}

// WithSecurityIDFromString sets the security ID from a string
func (b *TransactionBuilder) WithSecurityIDFromString(securityID string) *TransactionBuilder {
	sid, err := NewSecurityIDFromString(securityID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid security ID: %w", err))
		return b
	}
	b.transaction.securityID = sid
	return b
}

// WithSourceID sets the source ID
func (b *TransactionBuilder) WithSourceID(sourceID string) *TransactionBuilder {
	sid, err := NewSourceID(sourceID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid source ID: %w", err))
		return b
	}
	b.transaction.sourceID = sid
	return b
}

// WithTransactionType sets the transaction type
func (b *TransactionBuilder) WithTransactionType(transactionType string) *TransactionBuilder {
	tt, err := ParseTransactionType(transactionType)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid transaction type: %w", err))
		return b
	}
	b.transaction.transactionType = tt
	return b
}

// WithStatus sets the transaction status
func (b *TransactionBuilder) WithStatus(status string) *TransactionBuilder {
	s, err := ParseTransactionStatus(status)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid status: %w", err))
		return b
	}
	b.transaction.status = s
	return b
}

// WithQuantity sets the quantity
func (b *TransactionBuilder) WithQuantity(quantity decimal.Decimal) *TransactionBuilder {
	b.transaction.quantity = NewQuantity(quantity)
	return b
}

// WithQuantityFromString sets the quantity from a string
func (b *TransactionBuilder) WithQuantityFromString(quantity string) *TransactionBuilder {
	q, err := NewQuantityFromString(quantity)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid quantity: %w", err))
		return b
	}
	b.transaction.quantity = q
	return b
}

// WithPrice sets the price
func (b *TransactionBuilder) WithPrice(price decimal.Decimal) *TransactionBuilder {
	p, err := NewPrice(price)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid price: %w", err))
		return b
	}
	b.transaction.price = p
	return b
}

// WithPriceFromString sets the price from a string
func (b *TransactionBuilder) WithPriceFromString(price string) *TransactionBuilder {
	p, err := NewPriceFromString(price)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid price: %w", err))
		return b
	}
	b.transaction.price = p
	return b
}

// WithTransactionDate sets the transaction date
func (b *TransactionBuilder) WithTransactionDate(date time.Time) *TransactionBuilder {
	b.transaction.transactionDate = date.UTC().Truncate(24 * time.Hour)
	return b
}

// WithTransactionDateFromString sets the transaction date from YYYYMMDD string
func (b *TransactionBuilder) WithTransactionDateFromString(dateStr string) *TransactionBuilder {
	date, err := time.Parse("20060102", dateStr)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid transaction date format (expected YYYYMMDD): %w", err))
		return b
	}
	b.transaction.transactionDate = date.UTC()
	return b
}

// WithVersion sets the version for optimistic locking
func (b *TransactionBuilder) WithVersion(version int) *TransactionBuilder {
	if version < 1 {
		b.errors = append(b.errors, errors.New("version must be positive"))
		return b
	}
	b.transaction.version = version
	return b
}

// WithReprocessingAttempts sets the reprocessing attempts
func (b *TransactionBuilder) WithReprocessingAttempts(attempts int) *TransactionBuilder {
	if attempts < 0 {
		b.errors = append(b.errors, errors.New("reprocessing attempts cannot be negative"))
		return b
	}
	b.transaction.reprocessingAttempts = attempts
	return b
}

// WithErrorMessage sets the error message
func (b *TransactionBuilder) WithErrorMessage(message string) *TransactionBuilder {
	if message == "" {
		b.transaction.errorMessage = nil
	} else {
		b.transaction.errorMessage = &message
	}
	return b
}

// WithTimestamps sets created and updated timestamps
func (b *TransactionBuilder) WithTimestamps(createdAt, updatedAt time.Time) *TransactionBuilder {
	b.transaction.createdAt = createdAt.UTC()
	b.transaction.updatedAt = updatedAt.UTC()
	return b
}

// Build creates the transaction with validation
func (b *TransactionBuilder) Build() (*Transaction, error) {
	// Validate business rules
	if err := b.validateBusinessRules(); err != nil {
		b.errors = append(b.errors, err)
	}

	// Check for any accumulated errors
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("transaction validation failed: %v", b.errors)
	}

	// Create a copy to ensure immutability
	transaction := b.transaction
	return &transaction, nil
}

// validateBusinessRules validates business rules for the transaction
func (b *TransactionBuilder) validateBusinessRules() error {
	t := &b.transaction

	// Rule: Cash transactions (DEP/WD) must have nil security ID
	if t.transactionType.IsCashTransaction() {
		if !t.securityID.IsCash() {
			return errors.New("cash transactions (DEP/WD) must have empty security ID")
		}
		// Cash transactions should have price of 1.0
		if !t.price.Equals(CashPrice().Amount) {
			return errors.New("cash transactions must have price of 1.0")
		}
	}

	// Rule: Security transactions must have non-nil security ID
	if t.transactionType.IsSecurityTransaction() {
		if t.securityID.IsCash() {
			return errors.New("security transactions require a valid security ID")
		}
	}

	// Rule: Portfolio ID is always required
	if t.portfolioID.IsEmpty() {
		return errors.New("portfolio ID is required")
	}

	// Rule: Source ID is always required
	if t.sourceID.IsEmpty() {
		return errors.New("source ID is required")
	}

	// Rule: Quantity cannot be zero
	if t.quantity.IsZero() {
		return errors.New("quantity cannot be zero")
	}

	// Rule: Price must be positive
	if !t.price.IsPositive() && !t.price.IsZero() {
		return errors.New("price must be positive")
	}

	return nil
}

// Getters for accessing transaction data

// ID returns the transaction ID
func (t *Transaction) ID() int64 {
	return t.id
}

// PortfolioID returns the portfolio ID
func (t *Transaction) PortfolioID() PortfolioID {
	return t.portfolioID
}

// SecurityID returns the security ID
func (t *Transaction) SecurityID() SecurityID {
	return t.securityID
}

// SourceID returns the source ID
func (t *Transaction) SourceID() SourceID {
	return t.sourceID
}

// Status returns the transaction status
func (t *Transaction) Status() TransactionStatus {
	return t.status
}

// TransactionType returns the transaction type
func (t *Transaction) TransactionType() TransactionType {
	return t.transactionType
}

// Quantity returns the transaction quantity
func (t *Transaction) Quantity() Quantity {
	return t.quantity
}

// Price returns the transaction price
func (t *Transaction) Price() Price {
	return t.price
}

// TransactionDate returns the transaction date
func (t *Transaction) TransactionDate() time.Time {
	return t.transactionDate
}

// ReprocessingAttempts returns the number of reprocessing attempts
func (t *Transaction) ReprocessingAttempts() int {
	return t.reprocessingAttempts
}

// Version returns the version for optimistic locking
func (t *Transaction) Version() int {
	return t.version
}

// CreatedAt returns the creation timestamp
func (t *Transaction) CreatedAt() time.Time {
	return t.createdAt
}

// UpdatedAt returns the last update timestamp
func (t *Transaction) UpdatedAt() time.Time {
	return t.updatedAt
}

// ErrorMessage returns the error message if any
func (t *Transaction) ErrorMessage() *string {
	return t.errorMessage
}

// Business methods

// GetBalanceImpact returns the balance impact for this transaction
func (t *Transaction) GetBalanceImpact() BalanceImpact {
	return t.transactionType.GetBalanceImpact()
}

// IsCashTransaction returns true if this is a cash transaction
func (t *Transaction) IsCashTransaction() bool {
	return t.transactionType.IsCashTransaction()
}

// IsSecurityTransaction returns true if this is a security transaction
func (t *Transaction) IsSecurityTransaction() bool {
	return t.transactionType.IsSecurityTransaction()
}

// CanBeProcessed returns true if the transaction can be processed
func (t *Transaction) CanBeProcessed() bool {
	return t.status.CanBeReprocessed()
}

// IsProcessed returns true if the transaction has been processed
func (t *Transaction) IsProcessed() bool {
	return t.status.IsProcessed()
}

// CalculateNotionalAmount calculates the notional amount (quantity * price)
func (t *Transaction) CalculateNotionalAmount() Amount {
	return t.quantity.Amount.Mul(t.price.Amount)
}

// SetStatus updates the transaction status (creates new instance for immutability)
func (t *Transaction) SetStatus(status TransactionStatus, errorMessage *string) *Transaction {
	newTransaction := *t
	newTransaction.status = status
	newTransaction.errorMessage = errorMessage
	newTransaction.updatedAt = time.Now().UTC()
	return &newTransaction
}

// IncrementReprocessingAttempts increments the reprocessing attempts
func (t *Transaction) IncrementReprocessingAttempts() *Transaction {
	newTransaction := *t
	newTransaction.reprocessingAttempts++
	newTransaction.updatedAt = time.Now().UTC()
	return &newTransaction
}

// IncrementVersion increments the version for optimistic locking
func (t *Transaction) IncrementVersion() *Transaction {
	newTransaction := *t
	newTransaction.version++
	newTransaction.updatedAt = time.Now().UTC()
	return &newTransaction
}

// String returns a string representation of the transaction
func (t *Transaction) String() string {
	return fmt.Sprintf("Transaction{ID: %d, Portfolio: %s, Security: %s, Type: %s, Status: %s, Quantity: %s, Price: %s}",
		t.id, t.portfolioID.String(), t.securityID.String(), t.transactionType, t.status, t.quantity.String(), t.price.String())
}
