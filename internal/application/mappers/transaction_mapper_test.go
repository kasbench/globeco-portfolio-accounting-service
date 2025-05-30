package mappers

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
)

func TestTransactionMapper_FromPostDTO(t *testing.T) {
	mapper := NewTransactionMapper()

	t.Run("Valid BUY transaction DTO", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      stringPtr("SECURITY1234567890123456"),
			SourceID:        "SOURCE001",
			TransactionType: "BUY",
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "20240101",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.NoError(t, err)
		assert.NotNil(t, transaction)
		assert.Equal(t, "PORTFOLIO123456789012345", transaction.PortfolioID().String())
		assert.Equal(t, "SECURITY1234567890123456", transaction.SecurityID().String())
		assert.Equal(t, "SOURCE001", transaction.SourceID().String())
		assert.Equal(t, models.TransactionTypeBuy, transaction.TransactionType())
		assert.True(t, decimal.NewFromInt(100).Equal(transaction.Quantity().Value()))
		assert.True(t, decimal.NewFromFloat(50.25).Equal(transaction.Price().Value()))
		assert.Equal(t, models.TransactionStatusNew, transaction.Status())
	})

	t.Run("Valid DEP (cash) transaction DTO", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      nil, // Cash transaction
			SourceID:        "SOURCE002",
			TransactionType: "DEP",
			Quantity:        decimal.NewFromFloat(1000.00),
			Price:           decimal.NewFromInt(1),
			TransactionDate: "20240115",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.NoError(t, err)
		assert.NotNil(t, transaction)
		assert.Equal(t, "PORTFOLIO123456789012345", transaction.PortfolioID().String())
		assert.True(t, transaction.SecurityID().IsCash())
		assert.Equal(t, "SOURCE002", transaction.SourceID().String())
		assert.Equal(t, models.TransactionTypeDep, transaction.TransactionType())
		assert.True(t, decimal.NewFromFloat(1000.00).Equal(transaction.Quantity().Value()))
		assert.True(t, decimal.NewFromInt(1).Equal(transaction.Price().Value()))
		assert.True(t, transaction.IsCashTransaction())
	})

	t.Run("Invalid portfolio ID length", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "SHORT", // Too short
			SecurityID:      stringPtr("SECURITY1234567890123456"),
			SourceID:        "SOURCE001",
			TransactionType: "BUY",
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "20240101",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.Error(t, err)
		assert.Nil(t, transaction)
		assert.Contains(t, err.Error(), "portfolio ID")
	})

	t.Run("Invalid transaction date format", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      stringPtr("SECURITY1234567890123456"),
			SourceID:        "SOURCE001",
			TransactionType: "BUY",
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "invalid-date",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.Error(t, err)
		assert.Nil(t, transaction)
		assert.Contains(t, err.Error(), "date")
	})

	t.Run("Security transaction without security ID", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      nil, // Missing for security transaction
			SourceID:        "SOURCE001",
			TransactionType: "BUY", // Requires security ID
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "20240101",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.Error(t, err)
		assert.Nil(t, transaction)
		assert.Contains(t, err.Error(), "security ID")
	})

	t.Run("Cash transaction with security ID", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      stringPtr("SECURITY1234567890123456"), // Should not have security ID for cash
			SourceID:        "SOURCE002",
			TransactionType: "DEP", // Cash transaction
			Quantity:        decimal.NewFromFloat(1000.00),
			Price:           decimal.NewFromInt(1),
			TransactionDate: "20240115",
		}

		transaction, err := mapper.FromPostDTO(&postDTO)

		assert.Error(t, err)
		assert.Nil(t, transaction)
		assert.Contains(t, err.Error(), "cash transactions")
	})
}

func TestTransactionMapper_ToResponseDTO(t *testing.T) {
	mapper := NewTransactionMapper()

	t.Run("Domain transaction to response DTO", func(t *testing.T) {
		// Create domain transaction
		transaction, err := models.NewTransactionBuilder().
			WithID(123).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithSourceID("SOURCE001").
			WithTransactionType("BUY").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromFloat(50.25)).
			WithTransactionDate(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)).
			WithVersion(2).
			WithReprocessingAttempts(0).
			Build()

		require.NoError(t, err)

		// Set status after creation
		transactionWithStatus := transaction.SetStatus(models.TransactionStatusProc, nil)

		responseDTO := mapper.ToResponseDTO(transactionWithStatus)

		assert.NotNil(t, responseDTO)
		assert.Equal(t, int64(123), responseDTO.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", responseDTO.PortfolioID)
		assert.Equal(t, "SECURITY1234567890123456", *responseDTO.SecurityID)
		assert.Equal(t, "SOURCE001", responseDTO.SourceID)
		assert.Equal(t, "BUY", responseDTO.TransactionType)
		assert.Equal(t, "PROC", responseDTO.Status)
		assert.True(t, decimal.NewFromInt(100).Equal(responseDTO.Quantity))
		assert.True(t, decimal.NewFromFloat(50.25).Equal(responseDTO.Price))
		assert.Equal(t, "20240115", responseDTO.TransactionDate)
		assert.Equal(t, 2, responseDTO.Version)
		assert.Equal(t, 0, responseDTO.ReprocessingAttempts)
		assert.Nil(t, responseDTO.ErrorMessage)
	})

	t.Run("Cash transaction to response DTO", func(t *testing.T) {
		// Create cash transaction
		transaction, err := models.NewTransactionBuilder().
			WithID(456).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID("SOURCE002").
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromFloat(1000.00)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)).
			WithVersion(1).
			WithReprocessingAttempts(0).
			Build()

		require.NoError(t, err)

		responseDTO := mapper.ToResponseDTO(transaction)

		assert.NotNil(t, responseDTO)
		assert.Equal(t, int64(456), responseDTO.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", responseDTO.PortfolioID)
		assert.Nil(t, responseDTO.SecurityID) // Cash transaction
		assert.Equal(t, "SOURCE002", responseDTO.SourceID)
		assert.Equal(t, "DEP", responseDTO.TransactionType)
		assert.Equal(t, "NEW", responseDTO.Status)
		assert.True(t, decimal.NewFromFloat(1000.00).Equal(responseDTO.Quantity))
		assert.True(t, decimal.NewFromInt(1).Equal(responseDTO.Price))
		assert.Equal(t, "20240201", responseDTO.TransactionDate)
	})

	t.Run("Transaction with error message", func(t *testing.T) {
		errorMsg := "Validation error: Invalid quantity"

		// Create transaction with error
		transaction, err := models.NewTransactionBuilder().
			WithID(789).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithSourceID("SOURCE003").
			WithTransactionType("SELL").
			WithQuantity(decimal.NewFromInt(50)).
			WithPrice(decimal.NewFromFloat(25.50)).
			WithTransactionDate(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)).
			WithVersion(1).
			WithReprocessingAttempts(2).
			Build()

		require.NoError(t, err)

		// Set error message
		transactionWithError := transaction.SetStatus(models.TransactionStatusError, &errorMsg)

		responseDTO := mapper.ToResponseDTO(transactionWithError)

		assert.NotNil(t, responseDTO)
		assert.Equal(t, "ERROR", responseDTO.Status)
		assert.Equal(t, 2, responseDTO.ReprocessingAttempts)
		assert.NotNil(t, responseDTO.ErrorMessage)
		assert.Equal(t, errorMsg, *responseDTO.ErrorMessage)
	})
}

func TestTransactionMapper_ValidatePostDTO(t *testing.T) {
	mapper := NewTransactionMapper()

	t.Run("Valid DTO has no validation errors", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "PORTFOLIO123456789012345",
			SecurityID:      stringPtr("SECURITY1234567890123456"),
			SourceID:        "SOURCE001",
			TransactionType: "BUY",
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "20240101",
		}

		errors := mapper.ValidatePostDTO(&postDTO)

		assert.Len(t, errors, 0)
	})

	t.Run("Invalid DTO has validation errors", func(t *testing.T) {
		postDTO := dto.TransactionPostDTO{
			PortfolioID:     "",                 // Required field missing
			SecurityID:      stringPtr("SHORT"), // Invalid length
			SourceID:        "SOURCE001",
			TransactionType: "INVALID",               // Invalid type
			Quantity:        decimal.Zero,            // Zero quantity not allowed
			Price:           decimal.NewFromInt(-10), // Negative price not allowed
			TransactionDate: "invalid-date",          // Invalid date format
		}

		errors := mapper.ValidatePostDTO(&postDTO)

		assert.Greater(t, len(errors), 0)

		// Debug: Print actual error messages
		t.Logf("Validation errors found: %d", len(errors))
		for i, err := range errors {
			t.Logf("Error %d: Field=%s, Message=%s", i, err.Field, err.Message)
		}

		// Check specific validation errors that should be present
		fieldErrors := make(map[string]bool)
		for _, err := range errors {
			fieldErrors[err.Field] = true
		}

		// Should have at least one of these errors
		hasPortfolioError := fieldErrors["portfolioId"]
		hasSecurityError := fieldErrors["securityId"]
		hasTypeError := fieldErrors["transactionType"]
		hasPriceError := fieldErrors["price"]
		hasDateError := fieldErrors["transactionDate"]

		assert.True(t, hasPortfolioError || hasSecurityError || hasTypeError || hasPriceError || hasDateError,
			"Should have validation errors for invalid fields")
	})
}

func TestTransactionMapper_ToBatchResponse(t *testing.T) {
	mapper := NewTransactionMapper()

	t.Run("Create batch response from mixed results", func(t *testing.T) {
		// Create successful transactions
		successfulTxn1, err := models.NewTransactionBuilder().
			WithID(1).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithSourceID("SOURCE001").
			WithTransactionType("BUY").
			WithQuantity(decimal.NewFromInt(100)).
			WithPrice(decimal.NewFromFloat(50.25)).
			WithTransactionDate(time.Now()).
			Build()
		require.NoError(t, err)

		successfulTxn2, err := models.NewTransactionBuilder().
			WithID(2).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSourceID("SOURCE002").
			WithTransactionType("DEP").
			WithQuantity(decimal.NewFromFloat(1000.00)).
			WithPrice(decimal.NewFromInt(1)).
			WithTransactionDate(time.Now()).
			Build()
		require.NoError(t, err)

		successful := []*models.Transaction{successfulTxn1, successfulTxn2}

		// Create failed transaction DTOs
		failedDTO := dto.TransactionPostDTO{
			PortfolioID:     "INVALID", // Too short
			SecurityID:      stringPtr("SECURITY1234567890123456"),
			SourceID:        "SOURCE003",
			TransactionType: "BUY",
			Quantity:        decimal.NewFromInt(100),
			Price:           decimal.NewFromFloat(50.25),
			TransactionDate: "20240101",
		}

		failed := []dto.TransactionErrorDTO{
			{
				Transaction: failedDTO,
				Errors: []dto.ValidationError{
					{
						Field:   "portfolioId",
						Message: "Portfolio ID must be 24 characters",
						Value:   "INVALID",
					},
				},
			},
		}

		batchResponse := mapper.ToBatchResponse(successful, failed)

		assert.NotNil(t, batchResponse)
		assert.Len(t, batchResponse.Successful, 2)
		assert.Len(t, batchResponse.Failed, 1)
		assert.Equal(t, 3, batchResponse.Summary.TotalRequested)
		assert.Equal(t, 2, batchResponse.Summary.Successful)
		assert.Equal(t, 1, batchResponse.Summary.Failed)
		assert.InDelta(t, 66.67, batchResponse.Summary.SuccessRate, 0.01) // 2/3 * 100

		// Verify successful transaction mapping
		assert.Equal(t, int64(1), batchResponse.Successful[0].ID)
		assert.Equal(t, "BUY", batchResponse.Successful[0].TransactionType)
		assert.Equal(t, int64(2), batchResponse.Successful[1].ID)
		assert.Equal(t, "DEP", batchResponse.Successful[1].TransactionType)

		// Verify failed transaction
		assert.Equal(t, "INVALID", batchResponse.Failed[0].Transaction.PortfolioID)
		assert.Len(t, batchResponse.Failed[0].Errors, 1)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
