package models

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceBuilder(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *BalanceBuilder
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid security balance",
			setupFunc: func() *BalanceBuilder {
				return NewBalanceBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithQuantityLong(decimal.NewFromInt(100)).
					WithQuantityShort(decimal.NewFromInt(0))
			},
			expectError: false,
		},
		{
			name: "Valid cash balance",
			setupFunc: func() *BalanceBuilder {
				return NewBalanceBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithQuantityLong(decimal.NewFromFloat(1000.50)).
					WithQuantityShort(decimal.NewFromInt(0))
			},
			expectError: false,
		},
		{
			name: "Valid balance with both long and short positions",
			setupFunc: func() *BalanceBuilder {
				return NewBalanceBuilder().
					WithPortfolioID("PORTFOLIO123456789012345").
					WithSecurityIDFromString("SECURITY1234567890123456").
					WithQuantityLong(decimal.NewFromInt(100)).
					WithQuantityShort(decimal.NewFromInt(50))
			},
			expectError: false,
		},
		{
			name: "Invalid - empty portfolio ID",
			setupFunc: func() *BalanceBuilder {
				return NewBalanceBuilder().
					WithPortfolioID("").
					WithQuantityLong(decimal.NewFromInt(100)).
					WithQuantityShort(decimal.NewFromInt(0))
			},
			expectError: true,
			errorMsg:    "portfolio ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			balance, err := builder.Build()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, balance)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, balance)
				if balance != nil {
					assert.Equal(t, 1, balance.Version())
					assert.False(t, balance.LastUpdated().IsZero())
				}
			}
		})
	}
}

func TestBalanceBusinessRules(t *testing.T) {
	t.Run("Cash balance should have nil security ID", func(t *testing.T) {
		balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(1000.50)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, balance)
		if balance != nil {
			assert.True(t, balance.IsCashBalance())
			assert.True(t, balance.SecurityID().IsCash())
		}
	})

	t.Run("Security balance should have valid security ID", func(t *testing.T) {
		balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.NoError(t, err)
		assert.NotNil(t, balance)
		if balance != nil {
			assert.False(t, balance.IsCashBalance())
			assert.False(t, balance.SecurityID().IsCash())
			assert.Equal(t, "SECURITY1234567890123456", balance.SecurityID().String())
		}
	})
}

func TestBalanceValueObjects(t *testing.T) {
	t.Run("PortfolioID validation", func(t *testing.T) {
		validID := "PORTFOLIO123456789012345"
		balance, err := NewBalanceBuilder().
			WithPortfolioID(validID).
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.NoError(t, err)
		if balance != nil {
			assert.Equal(t, validID, balance.PortfolioID().String())
		}

		// Test invalid length
		invalidID := "SHORT"
		balance, err = NewBalanceBuilder().
			WithPortfolioID(invalidID).
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "portfolio ID")
	})

	t.Run("SecurityID validation", func(t *testing.T) {
		validID := "SECURITY1234567890123456"
		balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString(validID).
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.NoError(t, err)
		if balance != nil {
			assert.False(t, balance.SecurityID().IsCash())
			assert.Equal(t, validID, balance.SecurityID().String())
		}

		// Test invalid length
		invalidID := "SHORT"
		balance, err = NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString(invalidID).
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security ID")
	})

	t.Run("Quantity validation", func(t *testing.T) {
		// Test positive quantities for security balance
		balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(50)).
			Build()

		assert.NoError(t, err)
		if balance != nil {
			assert.True(t, balance.QuantityLong().Value().Equal(decimal.NewFromInt(100)))
			assert.True(t, balance.QuantityShort().Value().Equal(decimal.NewFromInt(50)))
		}

		// Test zero quantities for security balance
		balance, err = NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.Zero).
			WithQuantityShort(decimal.Zero).
			Build()

		assert.NoError(t, err)
		if balance != nil {
			assert.True(t, balance.QuantityLong().IsZero())
			assert.True(t, balance.QuantityShort().IsZero())
		}

		// Test negative quantities for security balance (should be allowed)
		balance, err = NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(-100)).
			WithQuantityShort(decimal.NewFromInt(-50)).
			Build()

		assert.NoError(t, err)
		if balance != nil {
			assert.True(t, balance.QuantityLong().IsNegative())
			assert.True(t, balance.QuantityShort().IsNegative())
		}

		// Test cash balance with positive long quantity and zero short
		cashBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(1000.50)).
			WithQuantityShort(decimal.Zero).
			Build()

		assert.NoError(t, err)
		if cashBalance != nil {
			assert.True(t, cashBalance.IsCashBalance())
			assert.True(t, cashBalance.QuantityLong().IsPositive())
			assert.True(t, cashBalance.QuantityShort().IsZero())
		}

		// Test that cash balance cannot have short quantity (business rule)
		_, err = NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(1000.50)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cash positions cannot have short quantities")
	})
}

func TestBalanceMethods(t *testing.T) {
	// Create a test balance
	balance, err := NewBalanceBuilder().
		WithPortfolioID("PORTFOLIO123456789012345").
		WithSecurityIDFromString("SECURITY1234567890123456").
		WithQuantityLong(decimal.NewFromInt(100)).
		WithQuantityShort(decimal.NewFromInt(50)).
		Build()

	require.NoError(t, err)
	require.NotNil(t, balance)

	t.Run("IsCash", func(t *testing.T) {
		// Security balance
		assert.False(t, balance.IsCashBalance())

		// Cash balance
		cashBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(1000.50)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()

		require.NoError(t, err)
		assert.True(t, cashBalance.IsCashBalance())
	})

	t.Run("NetQuantity", func(t *testing.T) {
		// Net = Long - Short = 100 - 50 = 50
		expectedNet := decimal.NewFromInt(50)
		actualNet := balance.NetQuantity()
		assert.True(t, expectedNet.Equal(actualNet.Value()))

		// Test zero net position
		zeroNetBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		require.NoError(t, err)
		assert.True(t, zeroNetBalance.NetQuantity().IsZero())
	})

	t.Run("IsLongPosition", func(t *testing.T) {
		// Long > Short
		assert.True(t, balance.IsLongPosition())

		// Long < Short
		shortBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(50)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		require.NoError(t, err)
		assert.False(t, shortBalance.IsLongPosition())

		// Long = Short
		neutralBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		require.NoError(t, err)
		assert.False(t, neutralBalance.IsLongPosition())
	})

	t.Run("IsShortPosition", func(t *testing.T) {
		// Long > Short
		assert.False(t, balance.IsShortPosition())

		// Long < Short
		shortBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(50)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		require.NoError(t, err)
		assert.True(t, shortBalance.IsShortPosition())
	})

	t.Run("IsFlat", func(t *testing.T) {
		// Long > Short
		assert.False(t, balance.IsFlat())

		// Long = Short
		flatBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(100)).
			Build()

		require.NoError(t, err)
		assert.True(t, flatBalance.IsFlat())
	})

	t.Run("UpdateQuantities", func(t *testing.T) {
		// Update quantities to new values
		newLongQuantity := NewQuantity(decimal.NewFromInt(125))
		newShortQuantity := NewQuantity(decimal.NewFromInt(25))

		updatedBalance := balance.UpdateQuantities(newLongQuantity, newShortQuantity)

		// Long should be 125
		expectedLong := decimal.NewFromInt(125)
		assert.True(t, expectedLong.Equal(updatedBalance.QuantityLong().Value()))

		// Short should be 25
		expectedShort := decimal.NewFromInt(25)
		assert.True(t, expectedShort.Equal(updatedBalance.QuantityShort().Value()))

		assert.NotEqual(t, balance.LastUpdated(), updatedBalance.LastUpdated())
		assert.Equal(t, balance.Version()+1, updatedBalance.Version())
	})

	t.Run("IncrementVersion", func(t *testing.T) {
		originalVersion := balance.Version()
		newBalance := balance.IncrementVersion()

		assert.Equal(t, originalVersion+1, newBalance.Version())
		assert.NotEqual(t, balance.LastUpdated(), newBalance.LastUpdated())
	})
}

func TestBalanceAggregation(t *testing.T) {
	t.Run("Calculate portfolio total values", func(t *testing.T) {
		// Create multiple balances for the same portfolio
		balances := []*Balance{}

		// Cash balance: $10,000
		cashBalance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(10000.00)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()
		require.NoError(t, err)
		balances = append(balances, cashBalance)

		// Security 1: 100 long, 0 short
		security1Balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(0)).
			Build()
		require.NoError(t, err)
		balances = append(balances, security1Balance)

		// Security 2: 50 long, 25 short (net 25 long)
		security2Balance, err := NewBalanceBuilder().
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY2345678901234567").
			WithQuantityLong(decimal.NewFromInt(50)).
			WithQuantityShort(decimal.NewFromInt(25)).
			Build()
		require.NoError(t, err)
		balances = append(balances, security2Balance)

		// Verify we have the expected number of balances
		assert.Len(t, balances, 3)

		// Count positions
		var cashBalances, securityBalances, longPositions, shortPositions int
		for _, balance := range balances {
			if balance.IsCashBalance() {
				cashBalances++
			} else {
				securityBalances++
				if balance.IsLongPosition() {
					longPositions++
				}
				if balance.IsShortPosition() {
					shortPositions++
				}
			}
		}

		assert.Equal(t, 1, cashBalances)
		assert.Equal(t, 2, securityBalances)
		assert.Equal(t, 2, longPositions)
		assert.Equal(t, 0, shortPositions)
	})
}
