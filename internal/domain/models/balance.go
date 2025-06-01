package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Balance represents a portfolio balance domain entity
type Balance struct {
	id            int64
	portfolioID   PortfolioID
	securityID    SecurityID
	quantityLong  Quantity
	quantityShort Quantity
	lastUpdated   time.Time
	version       int
	createdAt     time.Time
}

// BalanceBuilder helps build Balance entities with validation
type BalanceBuilder struct {
	balance Balance
	errors  []error
}

// NewBalanceBuilder creates a new balance builder
func NewBalanceBuilder() *BalanceBuilder {
	return &BalanceBuilder{
		balance: Balance{
			quantityLong:  ZeroQuantity(),
			quantityShort: ZeroQuantity(),
			lastUpdated:   time.Now().UTC(),
			version:       1,
			createdAt:     time.Now().UTC(),
		},
	}
}

// WithID sets the balance ID
func (b *BalanceBuilder) WithID(id int64) *BalanceBuilder {
	b.balance.id = id
	return b
}

// WithPortfolioID sets the portfolio ID
func (b *BalanceBuilder) WithPortfolioID(portfolioID string) *BalanceBuilder {
	pid, err := NewPortfolioID(portfolioID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid portfolio ID: %w", err))
		return b
	}
	b.balance.portfolioID = pid
	return b
}

// WithSecurityID sets the security ID
func (b *BalanceBuilder) WithSecurityID(securityID *string) *BalanceBuilder {
	sid, err := NewSecurityID(securityID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid security ID: %w", err))
		return b
	}
	b.balance.securityID = sid
	return b
}

// WithSecurityIDFromString sets the security ID from a string
func (b *BalanceBuilder) WithSecurityIDFromString(securityID string) *BalanceBuilder {
	sid, err := NewSecurityIDFromString(securityID)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid security ID: %w", err))
		return b
	}
	b.balance.securityID = sid
	return b
}

// WithQuantityLong sets the long quantity
func (b *BalanceBuilder) WithQuantityLong(quantity decimal.Decimal) *BalanceBuilder {
	b.balance.quantityLong = NewQuantity(quantity)
	return b
}

// WithQuantityLongFromString sets the long quantity from a string
func (b *BalanceBuilder) WithQuantityLongFromString(quantity string) *BalanceBuilder {
	q, err := NewQuantityFromString(quantity)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid long quantity: %w", err))
		return b
	}
	b.balance.quantityLong = q
	return b
}

// WithQuantityShort sets the short quantity
func (b *BalanceBuilder) WithQuantityShort(quantity decimal.Decimal) *BalanceBuilder {
	b.balance.quantityShort = NewQuantity(quantity)
	return b
}

// WithQuantityShortFromString sets the short quantity from a string
func (b *BalanceBuilder) WithQuantityShortFromString(quantity string) *BalanceBuilder {
	q, err := NewQuantityFromString(quantity)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid short quantity: %w", err))
		return b
	}
	b.balance.quantityShort = q
	return b
}

// WithVersion sets the version for optimistic locking
func (b *BalanceBuilder) WithVersion(version int) *BalanceBuilder {
	if version < 1 {
		b.errors = append(b.errors, errors.New("version must be positive"))
		return b
	}
	b.balance.version = version
	return b
}

// WithTimestamps sets the timestamps
func (b *BalanceBuilder) WithTimestamps(lastUpdated, createdAt time.Time) *BalanceBuilder {
	b.balance.lastUpdated = lastUpdated.UTC()
	b.balance.createdAt = createdAt.UTC()
	return b
}

// Build creates the balance with validation
func (b *BalanceBuilder) Build() (*Balance, error) {
	// Validate business rules
	if err := b.validateBusinessRules(); err != nil {
		b.errors = append(b.errors, err)
	}

	// Check for any accumulated errors
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("balance validation failed: %v", b.errors)
	}

	// Create a copy to ensure immutability
	balance := b.balance
	return &balance, nil
}

// validateBusinessRules validates business rules for the balance
func (b *BalanceBuilder) validateBusinessRules() error {
	bal := &b.balance

	// Rule: Portfolio ID is always required
	if bal.portfolioID.IsEmpty() {
		return errors.New("portfolio ID is required")
	}

	// Rule: Cash positions (nil security ID) can only have long quantities
	if bal.securityID.IsCash() {
		if !bal.quantityShort.IsZero() {
			return errors.New("cash positions cannot have short quantities")
		}
	}

	return nil
}

// Getters for accessing balance data

// ID returns the balance ID
func (b *Balance) ID() int64 {
	return b.id
}

// PortfolioID returns the portfolio ID
func (b *Balance) PortfolioID() PortfolioID {
	return b.portfolioID
}

// SecurityID returns the security ID
func (b *Balance) SecurityID() SecurityID {
	return b.securityID
}

// QuantityLong returns the long quantity
func (b *Balance) QuantityLong() Quantity {
	return b.quantityLong
}

// QuantityShort returns the short quantity
func (b *Balance) QuantityShort() Quantity {
	return b.quantityShort
}

// LastUpdated returns the last updated timestamp
func (b *Balance) LastUpdated() time.Time {
	return b.lastUpdated
}

// Version returns the version for optimistic locking
func (b *Balance) Version() int {
	return b.version
}

// CreatedAt returns the creation timestamp
func (b *Balance) CreatedAt() time.Time {
	return b.createdAt
}

// Business methods

// IsCashBalance returns true if this is a cash balance
func (b *Balance) IsCashBalance() bool {
	return b.securityID.IsCash()
}

// IsSecurityBalance returns true if this is a security balance
func (b *Balance) IsSecurityBalance() bool {
	return !b.securityID.IsCash()
}

// NetQuantity returns the net quantity (long - short)
func (b *Balance) NetQuantity() Quantity {
	return Quantity{Amount: b.quantityLong.Amount.Sub(b.quantityShort.Amount)}
}

// IsZero returns true if both long and short quantities are zero
func (b *Balance) IsZero() bool {
	return b.quantityLong.IsZero() && b.quantityShort.IsZero()
}

// HasLongPosition returns true if there is a long position
func (b *Balance) HasLongPosition() bool {
	return !b.quantityLong.IsZero()
}

// HasShortPosition returns true if there is a short position
func (b *Balance) HasShortPosition() bool {
	return !b.quantityShort.IsZero()
}

// IsLongPosition returns true if net position is long
func (b *Balance) IsLongPosition() bool {
	net := b.NetQuantity()
	return net.IsPositive()
}

// IsShortPosition returns true if net position is short
func (b *Balance) IsShortPosition() bool {
	net := b.NetQuantity()
	return net.IsNegative()
}

// IsFlat returns true if net position is zero
func (b *Balance) IsFlat() bool {
	net := b.NetQuantity()
	return net.IsZero()
}

// ApplyTransactionImpact applies a transaction's impact to the balance
func (b *Balance) ApplyTransactionImpact(transaction *Transaction) (*Balance, error) {
	// Validate that this balance applies to the transaction
	if !b.portfolioID.Equals(transaction.PortfolioID()) {
		return nil, errors.New("transaction portfolio ID does not match balance portfolio ID")
	}

	// For cash transactions, only apply to cash balances
	if transaction.IsCashTransaction() && !b.IsCashBalance() {
		return nil, errors.New("cash transaction can only be applied to cash balance")
	}

	// For security transactions, only apply to matching security balances
	if transaction.IsSecurityTransaction() {
		if b.IsCashBalance() || !b.securityID.Equals(transaction.SecurityID()) {
			return nil, errors.New("security transaction can only be applied to matching security balance")
		}
	}

	// Get the balance impact
	impact := transaction.GetBalanceImpact()
	quantity := transaction.Quantity()

	// Calculate new quantities
	newLongQuantity := b.quantityLong
	newShortQuantity := b.quantityShort

	// For cash transactions, apply cash impact to long quantity
	if transaction.IsCashTransaction() && b.IsCashBalance() {
		switch impact.Cash {
		case ImpactIncrease:
			newLongQuantity = Quantity{Amount: newLongQuantity.Amount.Add(quantity.Amount)}
		case ImpactDecrease:
			newLongQuantity = Quantity{Amount: newLongQuantity.Amount.Sub(quantity.Amount)}
		}
	} else {
		// For security transactions, apply long/short units impacts
		// Apply long units impact
		switch impact.LongUnits {
		case ImpactIncrease:
			newLongQuantity = Quantity{Amount: newLongQuantity.Amount.Add(quantity.Amount)}
		case ImpactDecrease:
			newLongQuantity = Quantity{Amount: newLongQuantity.Amount.Sub(quantity.Amount)}
		}

		// Apply short units impact
		switch impact.ShortUnits {
		case ImpactIncrease:
			newShortQuantity = Quantity{Amount: newShortQuantity.Amount.Add(quantity.Amount)}
		case ImpactDecrease:
			newShortQuantity = Quantity{Amount: newShortQuantity.Amount.Sub(quantity.Amount)}
		}
	}

	// Create new balance with updated quantities
	return NewBalanceBuilder().
		WithID(b.id).
		WithPortfolioID(b.portfolioID.String()).
		WithSecurityID(b.securityID.Value()).
		WithQuantityLong(newLongQuantity.Value()).
		WithQuantityShort(newShortQuantity.Value()).
		WithVersion(b.version + 1).
		Build()
}

// CreateCashBalanceUpdate creates a cash balance update for a transaction
func CreateCashBalanceUpdate(portfolioID PortfolioID, transaction *Transaction) (*Balance, error) {
	if !transaction.IsCashTransaction() {
		// For security transactions, we need to update cash based on the cash impact
		impact := transaction.GetBalanceImpact()
		if impact.Cash == ImpactNone {
			return nil, nil // No cash impact
		}

		// Calculate cash amount (quantity * price)
		cashAmount := transaction.Quantity().Amount.Mul(transaction.Price().Amount)
		if impact.Cash == ImpactDecrease {
			cashAmount = cashAmount.Neg()
		}

		return NewBalanceBuilder().
			WithPortfolioID(portfolioID.String()).
			WithSecurityID(nil). // Cash balance
			WithQuantityLong(cashAmount.Value()).
			WithQuantityShort(decimal.Zero).
			Build()
	}

	// For cash transactions, apply directly
	quantity := transaction.Quantity()
	if transaction.TransactionType() == TransactionTypeWd {
		quantity = Quantity{Amount: quantity.Amount.Neg()}
	}

	return NewBalanceBuilder().
		WithPortfolioID(portfolioID.String()).
		WithSecurityID(nil). // Cash balance
		WithQuantityLong(quantity.Value()).
		WithQuantityShort(decimal.Zero).
		Build()
}

// IncrementVersion increments the version for optimistic locking
func (b *Balance) IncrementVersion() *Balance {
	newBalance := *b
	newBalance.version++
	newBalance.lastUpdated = time.Now().UTC()
	return &newBalance
}

// UpdateQuantities updates the quantities and increments version
func (b *Balance) UpdateQuantities(longQuantity, shortQuantity Quantity) *Balance {
	newBalance := *b
	newBalance.quantityLong = longQuantity
	newBalance.quantityShort = shortQuantity
	newBalance.version++
	newBalance.lastUpdated = time.Now().UTC()
	return &newBalance
}

// CanCoverShortSale checks if the balance can cover a short sale
func (b *Balance) CanCoverShortSale(quantity Quantity) bool {
	if b.IsCashBalance() {
		return false // Cannot short sell cash
	}

	// For now, allow any short sale (margin requirements would be handled elsewhere)
	return true
}

// CanCoverPurchase checks if there's enough cash for a purchase
func (b *Balance) CanCoverPurchase(amount Amount) bool {
	if !b.IsCashBalance() {
		return false // This method only applies to cash balances
	}

	return b.quantityLong.GreaterThan(amount) || b.quantityLong.Equals(amount)
}

// String returns a string representation of the balance
func (b *Balance) String() string {
	securityStr := "CASH"
	if !b.securityID.IsCash() {
		securityStr = b.securityID.String()
	}

	return fmt.Sprintf("Balance{ID: %d, Portfolio: %s, Security: %s, Long: %s, Short: %s, Net: %s}",
		b.id, b.portfolioID.String(), securityStr,
		b.quantityLong.String(), b.quantityShort.String(), b.NetQuantity().String())
}
