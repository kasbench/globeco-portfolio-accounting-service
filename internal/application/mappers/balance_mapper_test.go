package mappers

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
)

func TestBalanceMapper_ToDTO(t *testing.T) {
	mapper := NewBalanceMapper()

	t.Run("Security balance to DTO", func(t *testing.T) {
		// Create security balance
		balance, err := models.NewBalanceBuilder().
			WithID(123).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.NewFromInt(25)).
			WithVersion(2).
			Build()

		require.NoError(t, err)

		balanceDTO := mapper.ToDTO(balance)

		assert.NotNil(t, balanceDTO)
		assert.Equal(t, int64(123), balanceDTO.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", balanceDTO.PortfolioID)
		assert.Equal(t, "SECURITY1234567890123456", *balanceDTO.SecurityID)
		assert.True(t, decimal.NewFromInt(100).Equal(balanceDTO.QuantityLong))
		assert.True(t, decimal.NewFromInt(25).Equal(balanceDTO.QuantityShort))
		assert.Equal(t, 2, balanceDTO.Version)
		assert.NotEmpty(t, balanceDTO.LastUpdated)
	})

	t.Run("Cash balance to DTO", func(t *testing.T) {
		// Create cash balance
		balance, err := models.NewBalanceBuilder().
			WithID(456).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(10000.50)).
			WithQuantityShort(decimal.Zero).
			WithVersion(1).
			Build()

		require.NoError(t, err)

		balanceDTO := mapper.ToDTO(balance)

		assert.NotNil(t, balanceDTO)
		assert.Equal(t, int64(456), balanceDTO.ID)
		assert.Equal(t, "PORTFOLIO123456789012345", balanceDTO.PortfolioID)
		assert.Nil(t, balanceDTO.SecurityID) // Cash balance
		assert.True(t, decimal.NewFromFloat(10000.50).Equal(balanceDTO.QuantityLong))
		assert.True(t, decimal.Zero.Equal(balanceDTO.QuantityShort))
		assert.Equal(t, 1, balanceDTO.Version)
	})

	t.Run("Nil balance returns nil DTO", func(t *testing.T) {
		balanceDTO := mapper.ToDTO(nil)
		assert.Nil(t, balanceDTO)
	})
}

func TestBalanceMapper_ToDTOs(t *testing.T) {
	mapper := NewBalanceMapper()

	t.Run("Multiple balances to DTOs", func(t *testing.T) {
		// Create test balances
		balance1, err := models.NewBalanceBuilder().
			WithID(1).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		balance2, err := models.NewBalanceBuilder().
			WithID(2).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(5000.00)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		balances := []*models.Balance{balance1, balance2}

		balanceDTOs := mapper.ToDTOs(balances)

		assert.Len(t, balanceDTOs, 2)
		assert.Equal(t, int64(1), balanceDTOs[0].ID)
		assert.NotNil(t, balanceDTOs[0].SecurityID) // Security balance
		assert.Equal(t, int64(2), balanceDTOs[1].ID)
		assert.Nil(t, balanceDTOs[1].SecurityID) // Cash balance
	})

	t.Run("Empty slice returns empty slice", func(t *testing.T) {
		balanceDTOs := mapper.ToDTOs([]*models.Balance{})
		assert.NotNil(t, balanceDTOs)
		assert.Len(t, balanceDTOs, 0)
	})

	t.Run("Nil slice returns nil", func(t *testing.T) {
		balanceDTOs := mapper.ToDTOs(nil)
		assert.Nil(t, balanceDTOs)
	})
}

func TestBalanceMapper_ToPortfolioSummaryDTO(t *testing.T) {
	mapper := NewBalanceMapper()

	t.Run("Create portfolio summary from balances", func(t *testing.T) {
		portfolioID := "PORTFOLIO123456789012345"

		// Create test balances
		cashBalance, err := models.NewBalanceBuilder().
			WithID(1).
			WithPortfolioID(portfolioID).
			WithQuantityLong(decimal.NewFromFloat(10000.00)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		securityBalance1, err := models.NewBalanceBuilder().
			WithID(2).
			WithPortfolioID(portfolioID).
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(100)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		securityBalance2, err := models.NewBalanceBuilder().
			WithID(3).
			WithPortfolioID(portfolioID).
			WithSecurityIDFromString("SECURITY2345678901234567").
			WithQuantityLong(decimal.NewFromInt(50)).
			WithQuantityShort(decimal.NewFromInt(25)).
			Build()
		require.NoError(t, err)

		balances := []*models.Balance{cashBalance, securityBalance1, securityBalance2}

		summaryDTO := mapper.ToPortfolioSummaryDTO(portfolioID, balances)

		assert.NotNil(t, summaryDTO)
		assert.Equal(t, portfolioID, summaryDTO.PortfolioID)
		assert.True(t, decimal.NewFromFloat(10000.00).Equal(summaryDTO.CashBalance))
		assert.Len(t, summaryDTO.Securities, 2)
		assert.Equal(t, 2, summaryDTO.SecurityCount)

		// Check security positions
		secPosition1 := summaryDTO.Securities[0]
		assert.Equal(t, "SECURITY1234567890123456", secPosition1.SecurityID)
		assert.True(t, decimal.NewFromInt(100).Equal(secPosition1.QuantityLong))
		assert.True(t, decimal.Zero.Equal(secPosition1.QuantityShort))
		assert.True(t, decimal.NewFromInt(100).Equal(secPosition1.NetQuantity))

		secPosition2 := summaryDTO.Securities[1]
		assert.Equal(t, "SECURITY2345678901234567", secPosition2.SecurityID)
		assert.True(t, decimal.NewFromInt(50).Equal(secPosition2.QuantityLong))
		assert.True(t, decimal.NewFromInt(25).Equal(secPosition2.QuantityShort))
		assert.True(t, decimal.NewFromInt(25).Equal(secPosition2.NetQuantity))
	})

	t.Run("Portfolio with only cash balance", func(t *testing.T) {
		portfolioID := "PORTFOLIO123456789012345"

		cashBalance, err := models.NewBalanceBuilder().
			WithID(1).
			WithPortfolioID(portfolioID).
			WithQuantityLong(decimal.NewFromFloat(5000.00)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		balances := []*models.Balance{cashBalance}

		summaryDTO := mapper.ToPortfolioSummaryDTO(portfolioID, balances)

		assert.NotNil(t, summaryDTO)
		assert.Equal(t, portfolioID, summaryDTO.PortfolioID)
		assert.True(t, decimal.NewFromFloat(5000.00).Equal(summaryDTO.CashBalance))
		assert.Len(t, summaryDTO.Securities, 0)
		assert.Equal(t, 0, summaryDTO.SecurityCount)
	})

	t.Run("Empty balances returns empty summary", func(t *testing.T) {
		portfolioID := "PORTFOLIO123456789012345"

		summaryDTO := mapper.ToPortfolioSummaryDTO(portfolioID, []*models.Balance{})

		assert.NotNil(t, summaryDTO)
		assert.Equal(t, portfolioID, summaryDTO.PortfolioID)
		assert.True(t, decimal.Zero.Equal(summaryDTO.CashBalance))
		assert.Len(t, summaryDTO.Securities, 0)
		assert.Equal(t, 0, summaryDTO.SecurityCount)
	})
}

func TestBalanceMapper_ToBatchUpdateResponse(t *testing.T) {
	mapper := NewBalanceMapper()

	t.Run("Create batch response from mixed results", func(t *testing.T) {
		// Create successful balance update responses
		balance1, err := models.NewBalanceBuilder().
			WithID(1).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithSecurityIDFromString("SECURITY1234567890123456").
			WithQuantityLong(decimal.NewFromInt(150)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		balance2, err := models.NewBalanceBuilder().
			WithID(2).
			WithPortfolioID("PORTFOLIO123456789012345").
			WithQuantityLong(decimal.NewFromFloat(6000.00)).
			WithQuantityShort(decimal.Zero).
			Build()
		require.NoError(t, err)

		successful := []dto.BalanceUpdateResponse{
			mapper.ToBalanceUpdateResponse(balance1, nil, true),
			mapper.ToBalanceUpdateResponse(balance2, nil, true),
		}

		// Create failed balance update errors
		failed := []dto.BalanceUpdateError{
			{
				BalanceID: 999,
				Errors: []dto.ValidationError{
					{
						Field:   "version",
						Message: "Version mismatch",
						Value:   "1",
					},
				},
			},
		}

		batchResponse := mapper.ToBatchUpdateResponse(successful, failed)

		assert.NotNil(t, batchResponse)
		assert.Len(t, batchResponse.Successful, 2)
		assert.Len(t, batchResponse.Failed, 1)
		assert.Equal(t, 3, batchResponse.Summary.TotalRequested)
		assert.Equal(t, 2, batchResponse.Summary.Successful)
		assert.Equal(t, 1, batchResponse.Summary.Failed)
		assert.InDelta(t, 66.67, batchResponse.Summary.SuccessRate, 0.01) // 2/3 * 100

		// Verify successful balance updates
		assert.Equal(t, int64(1), batchResponse.Successful[0].Balance.ID)
		assert.True(t, batchResponse.Successful[0].Updated)
		assert.Equal(t, int64(2), batchResponse.Successful[1].Balance.ID)
		assert.True(t, batchResponse.Successful[1].Updated)

		// Verify failed balance update
		assert.Equal(t, int64(999), batchResponse.Failed[0].BalanceID)
		assert.Len(t, batchResponse.Failed[0].Errors, 1)
		assert.Equal(t, "version", batchResponse.Failed[0].Errors[0].Field)
	})
}

func TestBalanceMapper_ValidateBalanceUpdateRequest(t *testing.T) {
	mapper := NewBalanceMapper()

	t.Run("Valid update request has no errors", func(t *testing.T) {
		request := &dto.BalanceUpdateRequest{
			QuantityLong:  &decimal.Decimal{},
			QuantityShort: nil,
			Version:       1,
		}

		errors := mapper.ValidateBalanceUpdateRequest(request)
		assert.Len(t, errors, 0)
	})

	t.Run("Invalid version produces error", func(t *testing.T) {
		request := &dto.BalanceUpdateRequest{
			QuantityLong:  &decimal.Decimal{},
			QuantityShort: nil,
			Version:       0, // Invalid version
		}

		errors := mapper.ValidateBalanceUpdateRequest(request)
		assert.Greater(t, len(errors), 0)

		hasVersionError := false
		for _, err := range errors {
			if err.Field == "version" {
				hasVersionError = true
				break
			}
		}
		assert.True(t, hasVersionError)
	})

	t.Run("No quantities provided produces error", func(t *testing.T) {
		request := &dto.BalanceUpdateRequest{
			QuantityLong:  nil,
			QuantityShort: nil,
			Version:       1,
		}

		errors := mapper.ValidateBalanceUpdateRequest(request)
		assert.Greater(t, len(errors), 0)

		hasQuantitiesError := false
		for _, err := range errors {
			if err.Field == "quantities" {
				hasQuantitiesError = true
				break
			}
		}
		assert.True(t, hasQuantitiesError)
	})
}
