package models

import (
	"errors"
	"strings"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeBuy   TransactionType = "BUY"   // Buy security
	TransactionTypeSell  TransactionType = "SELL"  // Sell security
	TransactionTypeShort TransactionType = "SHORT" // Short sell security
	TransactionTypeCover TransactionType = "COVER" // Cover short position
	TransactionTypeDep   TransactionType = "DEP"   // Cash deposit
	TransactionTypeWd    TransactionType = "WD"    // Cash withdrawal
	TransactionTypeIn    TransactionType = "IN"    // Transfer in (securities)
	TransactionTypeOut   TransactionType = "OUT"   // Transfer out (securities)
)

// AllTransactionTypes returns all valid transaction types
func AllTransactionTypes() []TransactionType {
	return []TransactionType{
		TransactionTypeBuy,
		TransactionTypeSell,
		TransactionTypeShort,
		TransactionTypeCover,
		TransactionTypeDep,
		TransactionTypeWd,
		TransactionTypeIn,
		TransactionTypeOut,
	}
}

// String returns the string representation of the transaction type
func (t TransactionType) String() string {
	return string(t)
}

// IsValid checks if the transaction type is valid
func (t TransactionType) IsValid() bool {
	for _, validType := range AllTransactionTypes() {
		if t == validType {
			return true
		}
	}
	return false
}

// IsCashTransaction returns true if this transaction type represents a cash transaction
func (t TransactionType) IsCashTransaction() bool {
	return t == TransactionTypeDep || t == TransactionTypeWd
}

// IsSecurityTransaction returns true if this transaction type represents a security transaction
func (t TransactionType) IsSecurityTransaction() bool {
	return !t.IsCashTransaction()
}

// ParseTransactionType parses a string into a TransactionType
func ParseTransactionType(s string) (TransactionType, error) {
	t := TransactionType(strings.ToUpper(strings.TrimSpace(s)))
	if !t.IsValid() {
		return "", errors.New("invalid transaction type")
	}
	return t, nil
}

// TransactionStatus represents the processing status of a transaction
type TransactionStatus string

const (
	TransactionStatusNew   TransactionStatus = "NEW"   // Initial status
	TransactionStatusProc  TransactionStatus = "PROC"  // Processed successfully
	TransactionStatusError TransactionStatus = "ERROR" // Non-fatal error, can be reprocessed
	TransactionStatusFatal TransactionStatus = "FATAL" // Fatal error, cannot be reprocessed
)

// AllTransactionStatuses returns all valid transaction statuses
func AllTransactionStatuses() []TransactionStatus {
	return []TransactionStatus{
		TransactionStatusNew,
		TransactionStatusProc,
		TransactionStatusError,
		TransactionStatusFatal,
	}
}

// String returns the string representation of the transaction status
func (s TransactionStatus) String() string {
	return string(s)
}

// IsValid checks if the transaction status is valid
func (s TransactionStatus) IsValid() bool {
	for _, validStatus := range AllTransactionStatuses() {
		if s == validStatus {
			return true
		}
	}
	return false
}

// IsProcessed returns true if the transaction has been successfully processed
func (s TransactionStatus) IsProcessed() bool {
	return s == TransactionStatusProc
}

// CanBeReprocessed returns true if the transaction can be reprocessed
func (s TransactionStatus) CanBeReprocessed() bool {
	return s == TransactionStatusNew || s == TransactionStatusError
}

// IsFinalState returns true if the status represents a final state
func (s TransactionStatus) IsFinalState() bool {
	return s == TransactionStatusProc || s == TransactionStatusFatal
}

// ParseTransactionStatus parses a string into a TransactionStatus
func ParseTransactionStatus(s string) (TransactionStatus, error) {
	status := TransactionStatus(strings.ToUpper(strings.TrimSpace(s)))
	if !status.IsValid() {
		return "", errors.New("invalid transaction status")
	}
	return status, nil
}

// BalanceImpact represents how a transaction type affects balances
type BalanceImpact struct {
	LongUnits  ImpactDirection // Impact on long position
	ShortUnits ImpactDirection // Impact on short position
	Cash       ImpactDirection // Impact on cash balance
}

// ImpactDirection represents the direction of balance impact
type ImpactDirection int

const (
	ImpactNone     ImpactDirection = 0  // No impact
	ImpactIncrease ImpactDirection = 1  // Increase balance
	ImpactDecrease ImpactDirection = -1 // Decrease balance
)

// String returns the string representation of the impact direction
func (i ImpactDirection) String() string {
	switch i {
	case ImpactNone:
		return "NONE"
	case ImpactIncrease:
		return "INCREASE"
	case ImpactDecrease:
		return "DECREASE"
	default:
		return "UNKNOWN"
	}
}

// GetBalanceImpact returns the balance impact for a given transaction type
func (t TransactionType) GetBalanceImpact() BalanceImpact {
	switch t {
	case TransactionTypeBuy:
		return BalanceImpact{
			LongUnits:  ImpactIncrease,
			ShortUnits: ImpactNone,
			Cash:       ImpactDecrease,
		}
	case TransactionTypeSell:
		return BalanceImpact{
			LongUnits:  ImpactDecrease,
			ShortUnits: ImpactNone,
			Cash:       ImpactIncrease,
		}
	case TransactionTypeShort:
		return BalanceImpact{
			LongUnits:  ImpactNone,
			ShortUnits: ImpactIncrease,
			Cash:       ImpactIncrease,
		}
	case TransactionTypeCover:
		return BalanceImpact{
			LongUnits:  ImpactNone,
			ShortUnits: ImpactDecrease,
			Cash:       ImpactDecrease,
		}
	case TransactionTypeDep:
		return BalanceImpact{
			LongUnits:  ImpactNone,
			ShortUnits: ImpactNone,
			Cash:       ImpactIncrease,
		}
	case TransactionTypeWd:
		return BalanceImpact{
			LongUnits:  ImpactNone,
			ShortUnits: ImpactNone,
			Cash:       ImpactDecrease,
		}
	case TransactionTypeIn:
		return BalanceImpact{
			LongUnits:  ImpactIncrease,
			ShortUnits: ImpactNone,
			Cash:       ImpactNone,
		}
	case TransactionTypeOut:
		return BalanceImpact{
			LongUnits:  ImpactDecrease,
			ShortUnits: ImpactNone,
			Cash:       ImpactNone,
		}
	default:
		return BalanceImpact{
			LongUnits:  ImpactNone,
			ShortUnits: ImpactNone,
			Cash:       ImpactNone,
		}
	}
}
