package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

// PortfolioID represents a portfolio identifier value object
type PortfolioID struct {
	value string
}

// NewPortfolioID creates a new PortfolioID with validation
func NewPortfolioID(value string) (PortfolioID, error) {
	if err := validatePortfolioID(value); err != nil {
		return PortfolioID{}, err
	}
	return PortfolioID{value: value}, nil
}

// String returns the string representation of the portfolio ID
func (p PortfolioID) String() string {
	return p.value
}

// Value returns the underlying string value
func (p PortfolioID) Value() string {
	return p.value
}

// IsEmpty returns true if the portfolio ID is empty
func (p PortfolioID) IsEmpty() bool {
	return p.value == ""
}

// Equals checks if two portfolio IDs are equal
func (p PortfolioID) Equals(other PortfolioID) bool {
	return p.value == other.value
}

// validatePortfolioID validates a portfolio ID string
func validatePortfolioID(value string) error {
	if len(value) != 24 {
		return fmt.Errorf("portfolio ID must be exactly 24 characters, got %d", len(value))
	}

	// Check for alphanumeric characters only
	if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(value) {
		return errors.New("portfolio ID must contain only alphanumeric characters")
	}

	return nil
}

// SecurityID represents a security identifier value object
type SecurityID struct {
	value *string
}

// NewSecurityID creates a new SecurityID with validation
func NewSecurityID(value *string) (SecurityID, error) {
	if value == nil {
		return SecurityID{value: nil}, nil // Allow nil for cash transactions
	}

	if err := validateSecurityID(*value); err != nil {
		return SecurityID{}, err
	}

	return SecurityID{value: value}, nil
}

// NewSecurityIDFromString creates a new SecurityID from a string
func NewSecurityIDFromString(value string) (SecurityID, error) {
	if value == "" {
		return SecurityID{value: nil}, nil
	}
	return NewSecurityID(&value)
}

// String returns the string representation of the security ID
func (s SecurityID) String() string {
	if s.value == nil {
		return ""
	}
	return *s.value
}

// Value returns the underlying string pointer
func (s SecurityID) Value() *string {
	return s.value
}

// IsEmpty returns true if the security ID is nil or empty
func (s SecurityID) IsEmpty() bool {
	return s.value == nil || *s.value == ""
}

// IsCash returns true if this represents a cash position (nil security ID)
func (s SecurityID) IsCash() bool {
	return s.value == nil
}

// Equals checks if two security IDs are equal
func (s SecurityID) Equals(other SecurityID) bool {
	if s.value == nil && other.value == nil {
		return true
	}
	if s.value == nil || other.value == nil {
		return false
	}
	return *s.value == *other.value
}

// validateSecurityID validates a security ID string
func validateSecurityID(value string) error {
	if len(value) != 24 {
		return fmt.Errorf("security ID must be exactly 24 characters, got %d", len(value))
	}

	// Check for alphanumeric characters only
	if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(value) {
		return errors.New("security ID must contain only alphanumeric characters")
	}

	return nil
}

// SourceID represents a source system identifier value object
type SourceID struct {
	value string
}

// NewSourceID creates a new SourceID with validation
func NewSourceID(value string) (SourceID, error) {
	if err := validateSourceID(value); err != nil {
		return SourceID{}, err
	}
	return SourceID{value: value}, nil
}

// String returns the string representation of the source ID
func (s SourceID) String() string {
	return s.value
}

// Value returns the underlying string value
func (s SourceID) Value() string {
	return s.value
}

// IsEmpty returns true if the source ID is empty
func (s SourceID) IsEmpty() bool {
	return s.value == ""
}

// Equals checks if two source IDs are equal
func (s SourceID) Equals(other SourceID) bool {
	return s.value == other.value
}

// validateSourceID validates a source ID string
func validateSourceID(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("source ID cannot be empty")
	}

	if len(trimmed) > 50 {
		return fmt.Errorf("source ID cannot exceed 50 characters, got %d", len(trimmed))
	}

	return nil
}

// Amount represents a monetary or quantity amount value object
type Amount struct {
	value decimal.Decimal
}

// NewAmount creates a new Amount from a decimal
func NewAmount(value decimal.Decimal) Amount {
	return Amount{value: value}
}

// NewAmountFromString creates a new Amount from a string
func NewAmountFromString(value string) (Amount, error) {
	dec, err := decimal.NewFromString(value)
	if err != nil {
		return Amount{}, fmt.Errorf("invalid amount format: %w", err)
	}
	return Amount{value: dec}, nil
}

// NewAmountFromFloat creates a new Amount from a float64
func NewAmountFromFloat(value float64) Amount {
	return Amount{value: decimal.NewFromFloat(value)}
}

// NewAmountFromInt creates a new Amount from an int64
func NewAmountFromInt(value int64) Amount {
	return Amount{value: decimal.NewFromInt(value)}
}

// Zero returns a zero amount
func ZeroAmount() Amount {
	return Amount{value: decimal.Zero}
}

// Value returns the underlying decimal value
func (a Amount) Value() decimal.Decimal {
	return a.value
}

// String returns the string representation of the amount
func (a Amount) String() string {
	return a.value.String()
}

// Float64 returns the float64 representation of the amount
func (a Amount) Float64() (float64, bool) {
	return a.value.Float64()
}

// IsZero returns true if the amount is zero
func (a Amount) IsZero() bool {
	return a.value.IsZero()
}

// IsPositive returns true if the amount is positive
func (a Amount) IsPositive() bool {
	return a.value.IsPositive()
}

// IsNegative returns true if the amount is negative
func (a Amount) IsNegative() bool {
	return a.value.IsNegative()
}

// Abs returns the absolute value of the amount
func (a Amount) Abs() Amount {
	return Amount{value: a.value.Abs()}
}

// Neg returns the negated amount
func (a Amount) Neg() Amount {
	return Amount{value: a.value.Neg()}
}

// Add adds two amounts
func (a Amount) Add(other Amount) Amount {
	return Amount{value: a.value.Add(other.value)}
}

// Sub subtracts an amount from this amount
func (a Amount) Sub(other Amount) Amount {
	return Amount{value: a.value.Sub(other.value)}
}

// Mul multiplies two amounts
func (a Amount) Mul(other Amount) Amount {
	return Amount{value: a.value.Mul(other.value)}
}

// Div divides this amount by another amount
func (a Amount) Div(other Amount) Amount {
	return Amount{value: a.value.Div(other.value)}
}

// Equals checks if two amounts are equal
func (a Amount) Equals(other Amount) bool {
	return a.value.Equal(other.value)
}

// Compare compares two amounts (-1, 0, 1)
func (a Amount) Compare(other Amount) int {
	return a.value.Cmp(other.value)
}

// GreaterThan checks if this amount is greater than another
func (a Amount) GreaterThan(other Amount) bool {
	return a.value.GreaterThan(other.value)
}

// LessThan checks if this amount is less than another
func (a Amount) LessThan(other Amount) bool {
	return a.value.LessThan(other.value)
}

// Round rounds the amount to the specified number of decimal places
func (a Amount) Round(places int32) Amount {
	return Amount{value: a.value.Round(places)}
}

// RoundToDecimalPlaces rounds to standard financial decimal places (8)
func (a Amount) RoundToDecimalPlaces() Amount {
	return a.Round(8)
}

// Price represents a price amount with additional validation for financial prices
type Price struct {
	Amount
}

// NewPrice creates a new Price with validation
func NewPrice(value decimal.Decimal) (Price, error) {
	if value.IsNegative() {
		return Price{}, errors.New("price cannot be negative")
	}
	return Price{Amount: NewAmount(value)}, nil
}

// NewPriceFromString creates a new Price from a string
func NewPriceFromString(value string) (Price, error) {
	amount, err := NewAmountFromString(value)
	if err != nil {
		return Price{}, err
	}
	return NewPrice(amount.value)
}

// NewPriceFromFloat creates a new Price from a float64
func NewPriceFromFloat(value float64) (Price, error) {
	if value < 0 {
		return Price{}, errors.New("price cannot be negative")
	}
	return Price{Amount: NewAmountFromFloat(value)}, nil
}

// CashPrice returns the standard price for cash transactions (1.0)
func CashPrice() Price {
	price, _ := NewPriceFromFloat(1.0)
	return price
}

// Quantity represents a transaction quantity that can be positive or negative
type Quantity struct {
	Amount
}

// NewQuantity creates a new Quantity
func NewQuantity(value decimal.Decimal) Quantity {
	return Quantity{Amount: NewAmount(value)}
}

// NewQuantityFromString creates a new Quantity from a string
func NewQuantityFromString(value string) (Quantity, error) {
	amount, err := NewAmountFromString(value)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{Amount: amount}, nil
}

// NewQuantityFromFloat creates a new Quantity from a float64
func NewQuantityFromFloat(value float64) Quantity {
	return Quantity{Amount: NewAmountFromFloat(value)}
}

// ZeroQuantity returns a zero quantity
func ZeroQuantity() Quantity {
	return Quantity{Amount: ZeroAmount()}
}
