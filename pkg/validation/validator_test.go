package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_Required(t *testing.T) {
	v := New()

	t.Run("Valid non-empty string", func(t *testing.T) {
		v.Clear()
		v.Required("field", "value")
		assert.False(t, v.HasErrors())
	})

	t.Run("Empty string produces error", func(t *testing.T) {
		v.Clear()
		v.Required("field", "")
		assert.True(t, v.HasErrors())
		assert.Len(t, v.Errors(), 1)
		assert.Equal(t, "field", v.Errors()[0].Field)
		assert.Contains(t, v.Errors()[0].Message, "required")
	})

	t.Run("Whitespace only string produces error", func(t *testing.T) {
		v.Clear()
		v.Required("field", "   ")
		assert.True(t, v.HasErrors())
	})
}

func TestValidator_ExactLength(t *testing.T) {
	v := New()

	t.Run("Correct length passes", func(t *testing.T) {
		v.Clear()
		v.ExactLength("field", "12345", 5)
		assert.False(t, v.HasErrors())
	})

	t.Run("Too short produces error", func(t *testing.T) {
		v.Clear()
		v.ExactLength("field", "123", 5)
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 5 characters")
	})

	t.Run("Too long produces error", func(t *testing.T) {
		v.Clear()
		v.ExactLength("field", "1234567", 5)
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 5 characters")
	})
}

func TestValidator_PortfolioID(t *testing.T) {
	v := New()

	t.Run("Valid portfolio ID", func(t *testing.T) {
		v.Clear()
		v.PortfolioID("portfolioId", "PORTFOLIO123456789012345")
		assert.False(t, v.HasErrors())
	})

	t.Run("Empty portfolio ID is allowed", func(t *testing.T) {
		v.Clear()
		v.PortfolioID("portfolioId", "")
		assert.False(t, v.HasErrors())
	})

	t.Run("Portfolio ID too short", func(t *testing.T) {
		v.Clear()
		v.PortfolioID("portfolioId", "SHORT")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 24 characters")
	})

	t.Run("Portfolio ID too long", func(t *testing.T) {
		v.Clear()
		v.PortfolioID("portfolioId", "PORTFOLIO1234567890123456789")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 24 characters")
	})
}

func TestValidator_SecurityID(t *testing.T) {
	v := New()

	t.Run("Valid security ID", func(t *testing.T) {
		v.Clear()
		v.SecurityID("securityId", "SECURITY1234567890123456")
		assert.False(t, v.HasErrors())
	})

	t.Run("Empty security ID is allowed", func(t *testing.T) {
		v.Clear()
		v.SecurityID("securityId", "")
		assert.False(t, v.HasErrors())
	})

	t.Run("Security ID too short", func(t *testing.T) {
		v.Clear()
		v.SecurityID("securityId", "SHORT")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 24 characters")
	})

	t.Run("Security ID too long", func(t *testing.T) {
		v.Clear()
		v.SecurityID("securityId", "SECURITY12345678901234567890")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "exactly 24 characters")
	})
}

func TestValidator_SourceID(t *testing.T) {
	v := New()

	t.Run("Valid source ID", func(t *testing.T) {
		v.Clear()
		v.SourceID("sourceId", "SOURCE12345678901234567890123456789012345678901")
		if v.HasErrors() {
			t.Logf("Validation errors: %v", v.Errors())
		}
		assert.False(t, v.HasErrors())
	})

	t.Run("Source ID too long", func(t *testing.T) {
		v.Clear()
		v.SourceID("sourceId", "SOURCE123456789012345678901234567890123456789012345678901")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "at most 50 characters")
	})

	t.Run("Empty source ID produces error", func(t *testing.T) {
		v.Clear()
		v.SourceID("sourceId", "")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "required")
	})

	t.Run("Minimum length source ID", func(t *testing.T) {
		v.Clear()
		v.SourceID("sourceId", "S")
		assert.False(t, v.HasErrors())
	})
}

func TestValidator_TransactionType(t *testing.T) {
	v := New()
	validTypes := []string{"BUY", "SELL", "SHORT", "COVER", "DEP", "WD", "IN", "OUT"}

	for _, validType := range validTypes {
		t.Run("Valid type: "+validType, func(t *testing.T) {
			v.Clear()
			v.TransactionType("transactionType", validType)
			assert.False(t, v.HasErrors())
		})
	}

	t.Run("Invalid transaction type", func(t *testing.T) {
		v.Clear()
		v.TransactionType("transactionType", "INVALID")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be one of")
	})

	t.Run("Empty transaction type", func(t *testing.T) {
		v.Clear()
		v.TransactionType("transactionType", "")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "required")
	})

	t.Run("Lowercase transaction type", func(t *testing.T) {
		v.Clear()
		v.TransactionType("transactionType", "buy")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be one of")
	})
}

func TestValidator_TransactionStatus(t *testing.T) {
	v := New()
	validStatuses := []string{"NEW", "PROC", "ERROR", "FATAL"}

	for _, validStatus := range validStatuses {
		t.Run("Valid status: "+validStatus, func(t *testing.T) {
			v.Clear()
			v.TransactionStatus("transactionStatus", validStatus)
			assert.False(t, v.HasErrors())
		})
	}

	t.Run("Invalid transaction status", func(t *testing.T) {
		v.Clear()
		v.TransactionStatus("transactionStatus", "INVALID")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be one of")
	})

	t.Run("Empty transaction status", func(t *testing.T) {
		v.Clear()
		v.TransactionStatus("transactionStatus", "")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "required")
	})
}

func TestValidator_YYYYMMDD(t *testing.T) {
	v := New()

	t.Run("Valid date format YYYYMMDD", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "20240315")
		assert.False(t, v.HasErrors())
	})

	t.Run("Invalid date format with dashes", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "2024-03-15")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "20060102")
	})

	t.Run("Invalid date format with slashes", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "03/15/2024")
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "20060102")
	})

	t.Run("Invalid date format too short", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "202403")
		assert.True(t, v.HasErrors())
	})

	t.Run("Invalid date format too long", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "202403155")
		assert.True(t, v.HasErrors())
	})

	t.Run("Invalid date format with letters", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "2024031A")
		assert.True(t, v.HasErrors())
	})

	t.Run("Empty date", func(t *testing.T) {
		v.Clear()
		v.YYYYMMDD("transactionDate", "")
		assert.True(t, v.HasErrors())
	})
}

func TestValidator_Positive(t *testing.T) {
	v := New()

	t.Run("Positive value passes", func(t *testing.T) {
		v.Clear()
		v.Positive("price", 10.50)
		assert.False(t, v.HasErrors())
	})

	t.Run("Zero value produces error", func(t *testing.T) {
		v.Clear()
		v.Positive("price", 0.0)
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be positive")
	})

	t.Run("Negative value produces error", func(t *testing.T) {
		v.Clear()
		v.Positive("price", -5.0)
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be positive")
	})
}

func TestValidator_OneOf(t *testing.T) {
	v := New()
	allowed := []string{"OPTION1", "OPTION2", "OPTION3"}

	t.Run("Valid option passes", func(t *testing.T) {
		v.Clear()
		v.OneOf("field", "OPTION2", allowed)
		assert.False(t, v.HasErrors())
	})

	t.Run("Invalid option produces error", func(t *testing.T) {
		v.Clear()
		v.OneOf("field", "INVALID", allowed)
		assert.True(t, v.HasErrors())
		assert.Contains(t, v.Errors()[0].Message, "must be one of")
		assert.Contains(t, v.Errors()[0].Message, "OPTION1, OPTION2, OPTION3")
	})
}

func TestValidatePortfolioTransaction(t *testing.T) {
	t.Run("Valid BUY transaction", func(t *testing.T) {
		errors := ValidatePortfolioTransaction(
			"PORTFOLIO123456789012345",
			"SECURITY1234567890123456",
			"SOURCE001",
			"BUY",
			"100.000000",
			"50.25",
			"20240315",
		)
		assert.False(t, errors.HasErrors())
	})

	t.Run("Valid cash DEP transaction", func(t *testing.T) {
		errors := ValidatePortfolioTransaction(
			"PORTFOLIO123456789012345",
			"", // Empty for cash transaction
			"SOURCE001",
			"DEP",
			"1000.00",
			"0.00",
			"20240315",
		)
		assert.False(t, errors.HasErrors())
	})

	t.Run("Invalid DEP transaction with security ID", func(t *testing.T) {
		errors := ValidatePortfolioTransaction(
			"PORTFOLIO123456789012345",
			"SECURITY1234567890123456", // Should be empty for DEP
			"SOURCE001",
			"DEP",
			"1000.00",
			"0.00",
			"20240315",
		)
		assert.True(t, errors.HasErrors())

		// Check for the specific business rule violation
		hasSecurityIDError := false
		for _, err := range errors {
			if err.Field == "securityId" && err.Message == "must be empty for cash transactions (DEP/WD)" {
				hasSecurityIDError = true
				break
			}
		}
		assert.True(t, hasSecurityIDError)
	})

	t.Run("Invalid BUY transaction without security ID", func(t *testing.T) {
		errors := ValidatePortfolioTransaction(
			"PORTFOLIO123456789012345",
			"", // Should not be empty for BUY
			"SOURCE001",
			"BUY",
			"100.00",
			"50.25",
			"20240315",
		)
		assert.True(t, errors.HasErrors())

		// Check for the specific business rule violation
		hasSecurityIDError := false
		for _, err := range errors {
			if err.Field == "securityId" && err.Message == "is required for non-cash transactions" {
				hasSecurityIDError = true
				break
			}
		}
		assert.True(t, hasSecurityIDError)
	})

	t.Run("Multiple validation errors", func(t *testing.T) {
		errors := ValidatePortfolioTransaction(
			"",             // Empty portfolio ID
			"SHORT",        // Invalid security ID length
			"",             // Empty source ID
			"INVALID",      // Invalid transaction type
			"",             // Empty quantity
			"",             // Empty price
			"INVALID-DATE", // Invalid date format
		)
		assert.True(t, errors.HasErrors())
		assert.Greater(t, len(errors), 5) // Should have multiple errors
	})
}
