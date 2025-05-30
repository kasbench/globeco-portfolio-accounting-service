package models

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionBuilder(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *TransactionBuilder
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid BUY transaction",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithSourceID("SOURCE001").
					WithTransactionType("BUY").
					WithQuantity(decimal.NewFromInt(100)).
					WithPrice(decimal.NewFromFloat(50.25)).
					WithTransactionDate(time.Now())
			},
			expectError: false,
		},
		{
			name: "Valid DEPOSIT transaction (no security ID)",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSourceID("SOURCE001").
					WithTransactionType("DEP").
					WithQuantity(decimal.NewFromInt(1000)).
					WithPrice(decimal.NewFromInt(1)).
					WithTransactionDate(time.Now())
			},
			expectError: false,
		},
		{
			name: "Invalid - BUY transaction without security ID",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSourceID("SOURCE001").
					WithTransactionType("BUY").
					WithQuantity(decimal.NewFromInt(100)).
					WithPrice(decimal.NewFromFloat(50.25)).
					WithTransactionDate(time.Now())
			},
			expectError: true,
			errorMsg:    "security transactions require a valid security ID",
		},
		{
			name: "Invalid - empty portfolio ID",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithSourceID("SOURCE001").
					WithTransactionType("BUY").
					WithQuantity(decimal.NewFromInt(100)).
					WithPrice(decimal.NewFromFloat(50.25)).
					WithTransactionDate(time.Now())
			},
			expectError: true,
			errorMsg:    "portfolio ID is required",
		},
		{
			name: "Invalid - zero quantity",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithSourceID("SOURCE001").
					WithTransactionType("BUY").
					WithQuantity(decimal.Zero).
					WithPrice(decimal.NewFromFloat(50.25)).
					WithTransactionDate(time.Now())
			},
			expectError: true,
			errorMsg:    "quantity cannot be zero",
		},
		{
			name: "Invalid - negative price",
			setupFunc: func() *TransactionBuilder {
				return NewTransactionBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithSourceID("SOURCE001").
					WithTransactionType("BUY").
					WithQuantity(decimal.NewFromInt(100)).
					WithPrice(decimal.NewFromInt(-10)).
					WithTransactionDate(time.Now())
			},
			expectError: true,
			errorMsg:    "price cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			transaction, err := builder.Build()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, transaction)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transaction)
				if transaction != nil {
					assert.Equal(t, TransactionStatusNew, transaction.Status())
					assert.Equal(t, 1, transaction.Version())
					assert.Equal(t, 0, transaction.ReprocessingAttempts())
				}
			}
		})
	}
}

func TestTransactionBusinessRules(t *testing.T) {
	baseBuilder := func() *TransactionBuilder {
		return NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID("SOURCE001").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromFloat(50.25)).
			WithTransactionDate(time.Now())
	}

	cashTransactionTypes := []string{
		"DEP",
		"WD",
	}

	securityTransactionTypes := []string{
		"BUY",
		"SELL",
		"SHORT",
		"COVER",
		"IN",
		"OUT",
	}

	t.Run("Cash transactions should not have security ID", func(t *testing.T) {
		for _, txType := range cashTransactionTypes {
			t.Run(txType, func(t *testing.T) {
				// Valid - no security ID
				transaction, err := baseBuilder().
					WithTransactionType(txType).
					WithPrice(decimal.NewFromInt(1)). // Cash transactions need price 1.0
					Build()
				assert.NoError(t, err)
				assert.NotNil(t, transaction)

				// Invalid - with security ID
				transaction, err = baseBuilder().
					WithTransactionType(txType).
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithPrice(decimal.NewFromInt(1)).
					Build()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cash transactions (DEP/WD) must have empty security ID")
			})
		}
	})

	t.Run("Security transactions should have security ID", func(t *testing.T) {
		for _, txType := range securityTransactionTypes {
			t.Run(txType, func(t *testing.T) {
				// Valid - with security ID
				transaction, err := baseBuilder().
					WithTransactionType(txType).
					WithSecurityIDFromString("SECURITY1234567890123456").
					Build()
				assert.NoError(t, err)
				assert.NotNil(t, transaction)

				// Invalid - no security ID
				transaction, err = baseBuilder().
					WithTransactionType(txType).
					Build()
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "security transactions require a valid security ID")
			})
		}
	})
}

func TestTransactionValueObjects(t *testing.T) {
	t.Run("PortfolioID validation", func(t *testing.T) {
		validID := "PORTFOLIO123456789012345"
		assert.Equal(t, 24, len(validID))

		transaction, err := NewTransactionBuilder().
			WithPortfolioID(validID).
			WithSourceID("SOURCE001").
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		assert.NoError(t, err)
		assert.Equal(t, validID, transaction.PortfolioID().String())

		// Test invalid length
		invalidID := "SHORT"
		transaction, err = NewTransactionBuilder().
			WithPortfolioID(invalidID).
			WithSourceID("SOURCE001").
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "portfolio ID")
	})

	t.Run("SecurityID validation", func(t *testing.T) {
		validID := "SECURITY1234567890123456"
		assert.Equal(t, 24, len(validID))

		transaction, err := NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString(validID).
			WithSourceID("SOURCE001").
			WithTransactionType("BUY").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromFloat(50.25)).
			WithTransactionDate(time.Now()).
			Build()

		assert.NoError(t, err)
		assert.False(t, transaction.SecurityID().IsCash())
		assert.Equal(t, validID, transaction.SecurityID().String())

		// Test invalid length
		invalidID := "SHORT"
		transaction, err = NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString(invalidID).
			WithSourceID("SOURCE001").
			WithTransactionType("BUY").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromFloat(50.25)).
			WithTransactionDate(time.Now()).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security ID")
	})

	t.Run("SourceID validation", func(t *testing.T) {
		validID := "SOURCE001"
		transaction, err := NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID(validID).
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		assert.NoError(t, err)
		if transaction != nil {
			assert.Equal(t, validID, transaction.SourceID().String())
		}

		// Test max length (50 characters)
		longValidID := "SOURCE12345678901234567890123456789012345678901234"
		assert.Equal(t, 50, len(longValidID))

		transaction, err = NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID(longValidID).
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		assert.NoError(t, err)
		if transaction != nil {
			assert.Equal(t, longValidID, transaction.SourceID().String())
		}

		// Test too long (51 characters)
		tooLongID := "SOURCE123456789012345678901234567890123456789012345"
		assert.Equal(t, 51, len(tooLongID))

		transaction, err = NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID(tooLongID).
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source ID")
	})
}

func TestTransactionMethods(t *testing.T) {
	transaction, err := NewTransactionBuilder().
		WithPortfolioID("PORTFOLIO123456789012345").
		WithSecurityIDFromString("SECURITY1234567890123456").
		WithSourceID("SOURCE001").
		WithTransactionType("BUY").
		WithQuantity(decimal.NewFromInt(100)).
		WithPrice(decimal.NewFromFloat(50.25)).
		WithTransactionDate(time.Now()).
		Build()

	require.NoError(t, err)

	t.Run("IsProcessed", func(t *testing.T) {
		assert.False(t, transaction.IsProcessed())

		processedTransaction := transaction.SetStatus(TransactionStatusProc, nil)
		assert.True(t, processedTransaction.IsProcessed())

		errorTransaction := transaction.SetStatus(TransactionStatusError, nil)
		assert.False(t, errorTransaction.IsProcessed())
	})

	t.Run("IsCashTransaction", func(t *testing.T) {
		// Security transaction
		assert.False(t, transaction.IsCashTransaction())

		// Cash transaction
		cashTransaction, err := NewTransactionBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID("SOURCE001").
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromInt(1000)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()

		require.NoError(t, err)
		assert.True(t, cashTransaction.IsCashTransaction())
	})

	t.Run("CalculateNotionalAmount", func(t *testing.T) {
		// quantity: 100, price: 50.25, total: 5025.00
		expectedTotal := decimal.NewFromFloat(5025.00)
		actualTotal := transaction.CalculateNotionalAmount()
		assert.True(t, expectedTotal.Equal(actualTotal.Value()))
	})

	t.Run("CanBeProcessed", func(t *testing.T) {
		assert.True(t, transaction.CanBeProcessed())

		processedTransaction := transaction.SetStatus(TransactionStatusProc, nil)
		assert.False(t, processedTransaction.CanBeProcessed())

		errorTransaction := transaction.SetStatus(TransactionStatusError, nil)
		assert.True(t, errorTransaction.CanBeProcessed())
	})

	t.Run("SetStatus", func(t *testing.T) {
		errorMsg := "Test error message"
		errorTransaction := transaction.SetStatus(TransactionStatusError, &errorMsg)

		assert.Equal(t, TransactionStatusError, errorTransaction.Status())
		assert.NotNil(t, errorTransaction.ErrorMessage())
		assert.Equal(t, errorMsg, *errorTransaction.ErrorMessage())
		assert.NotEqual(t, transaction.UpdatedAt(), errorTransaction.UpdatedAt())
	})

	t.Run("IncrementVersion", func(t *testing.T) {
		originalVersion := transaction.Version()
		newTransaction := transaction.IncrementVersion()
		assert.Equal(t, originalVersion+1, newTransaction.Version())
		assert.NotEqual(t, transaction.UpdatedAt(), newTransaction.UpdatedAt())
	})
}

func TestTransactionType(t *testing.T) {
	t.Run("IsValid", func(t *testing.T) {
		validTypes := []TransactionType{
			TransactionTypeBuy,
			TransactionTypeSell,
			TransactionTypeShort,
			TransactionTypeCover,
			TransactionTypeDep,
			TransactionTypeWd,
			TransactionTypeIn,
			TransactionTypeOut,
		}

		for _, txType := range validTypes {
			assert.True(t, txType.IsValid(), "Type %s should be valid", txType)
		}

		invalidType := TransactionType("INVALID")
		assert.False(t, invalidType.IsValid())
	})

	t.Run("IsCashTransaction", func(t *testing.T) {
		cashTypes := []TransactionType{
			TransactionTypeDep,
			TransactionTypeWd,
		}

		for _, txType := range cashTypes {
			assert.True(t, txType.IsCashTransaction(), "Type %s should be cash transaction", txType)
		}

		securityTypes := []TransactionType{
			TransactionTypeBuy,
			TransactionTypeSell,
			TransactionTypeShort,
			TransactionTypeCover,
			TransactionTypeIn,
			TransactionTypeOut,
		}

		for _, txType := range securityTypes {
			assert.False(t, txType.IsCashTransaction(), "Type %s should not be cash transaction", txType)
		}
	})

	t.Run("IsSecurityTransaction", func(t *testing.T) {
		securityTypes := []TransactionType{
			TransactionTypeBuy,
			TransactionTypeSell,
			TransactionTypeShort,
			TransactionTypeCover,
			TransactionTypeIn,
			TransactionTypeOut,
		}

		for _, txType := range securityTypes {
			assert.True(t, txType.IsSecurityTransaction(), "Type %s should be security transaction", txType)
		}

		cashTypes := []TransactionType{
			TransactionTypeDep,
			TransactionTypeWd,
		}

		for _, txType := range cashTypes {
			assert.False(t, txType.IsSecurityTransaction(), "Type %s should not be security transaction", txType)
		}
	})
}

func TestTransactionStatus(t *testing.T) {
	t.Run("IsValid", func(t *testing.T) {
		validStatuses := []TransactionStatus{
			TransactionStatusNew,
			TransactionStatusProc,
			TransactionStatusFatal,
			TransactionStatusError,
		}

		for _, status := range validStatuses {
			assert.True(t, status.IsValid(), "Status %s should be valid", status)
		}

		invalidStatus := TransactionStatus("INVALID")
		assert.False(t, invalidStatus.IsValid())
	})

	t.Run("CanBeReprocessed", func(t *testing.T) {
		processableStatuses := []TransactionStatus{
			TransactionStatusNew,
			TransactionStatusError,
		}

		for _, status := range processableStatuses {
			assert.True(t, status.CanBeReprocessed(), "Status %s should be processable", status)
		}

		nonProcessableStatuses := []TransactionStatus{
			TransactionStatusProc,
			TransactionStatusFatal,
		}

		for _, status := range nonProcessableStatuses {
			assert.False(t, status.CanBeReprocessed(), "Status %s should not be processable", status)
		}
	})
}
