package mappers

import (
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
)

// BalanceMapper handles mapping between Balance domain models and DTOs
type BalanceMapper struct{}

// NewBalanceMapper creates a new balance mapper
func NewBalanceMapper() *BalanceMapper {
	return &BalanceMapper{}
}

// ToDTO converts a domain Balance to BalanceDTO
func (m *BalanceMapper) ToDTO(balance *models.Balance) *dto.BalanceDTO {
	if balance == nil {
		return nil
	}

	var securityID *string
	if !balance.SecurityID().IsEmpty() {
		securityIDStr := balance.SecurityID().String()
		securityID = &securityIDStr
	}

	return &dto.BalanceDTO{
		ID:            balance.ID(),
		PortfolioID:   balance.PortfolioID().String(),
		SecurityID:    securityID,
		QuantityLong:  balance.QuantityLong().Value(),
		QuantityShort: balance.QuantityShort().Value(),
		LastUpdated:   balance.LastUpdated().Format(time.RFC3339),
		Version:       balance.Version(),
	}
}

// ToDTOs converts multiple domain Balances to BalanceDTOs
func (m *BalanceMapper) ToDTOs(balances []*models.Balance) []dto.BalanceDTO {
	if balances == nil {
		return nil
	}

	dtos := make([]dto.BalanceDTO, len(balances))
	for i, balance := range balances {
		if balanceDTO := m.ToDTO(balance); balanceDTO != nil {
			dtos[i] = *balanceDTO
		}
	}
	return dtos
}

// ToPortfolioSummaryDTO converts a balance slice to portfolio summary
func (m *BalanceMapper) ToPortfolioSummaryDTO(portfolioID string, balances []*models.Balance) *dto.PortfolioSummaryDTO {
	if balances == nil {
		return nil
	}

	summary := &dto.PortfolioSummaryDTO{
		PortfolioID:   portfolioID,
		CashBalance:   models.ZeroAmount().Value(),
		SecurityCount: 0,
		LastUpdated:   time.Now(),
		Securities:    make([]dto.SecurityPositionDTO, 0),
	}

	for _, balance := range balances {
		if balance.SecurityID().IsCash() {
			// This is the cash balance
			summary.CashBalance = balance.QuantityLong().Value()
		} else {
			// This is a security position
			securityPosition := dto.SecurityPositionDTO{
				SecurityID:    balance.SecurityID().String(),
				QuantityLong:  balance.QuantityLong().Value(),
				QuantityShort: balance.QuantityShort().Value(),
				NetQuantity:   balance.QuantityLong().Value().Sub(balance.QuantityShort().Value()),
				LastUpdated:   balance.LastUpdated(),
			}
			summary.Securities = append(summary.Securities, securityPosition)
			summary.SecurityCount++
		}

		// Update last updated timestamp to the most recent
		if balance.LastUpdated().After(summary.LastUpdated) {
			summary.LastUpdated = balance.LastUpdated()
		}
	}

	return summary
}

// ToBatchUpdateResponse converts balance update results to batch response
func (m *BalanceMapper) ToBatchUpdateResponse(successful []dto.BalanceUpdateResponse, failed []dto.BalanceUpdateError) dto.BulkBalanceUpdateResponse {
	totalRequested := len(successful) + len(failed)
	successfulCount := len(successful)
	failedCount := len(failed)

	var successRate float64
	if totalRequested > 0 {
		successRate = float64(successfulCount) / float64(totalRequested) * 100
	}

	return dto.BulkBalanceUpdateResponse{
		Successful: successful,
		Failed:     failed,
		Summary: dto.BulkUpdateSummaryDTO{
			TotalRequested: totalRequested,
			Successful:     successfulCount,
			Failed:         failedCount,
			SuccessRate:    successRate,
		},
	}
}

// ToBalanceUpdateResponse creates a balance update response
func (m *BalanceMapper) ToBalanceUpdateResponse(updated *models.Balance, previous *models.Balance, wasUpdated bool) dto.BalanceUpdateResponse {
	response := dto.BalanceUpdateResponse{
		Updated: wasUpdated,
	}

	if updated != nil {
		response.Balance = *m.ToDTO(updated)
	}

	if previous != nil {
		response.PreviousValue = *m.ToDTO(previous)
	}

	return response
}

// ToHistoryDTO converts balance change information to history DTO
func (m *BalanceMapper) ToHistoryDTO(balance *models.Balance, changeType string, transactionID *int64) *dto.BalanceHistoryDTO {
	if balance == nil {
		return nil
	}

	var securityID *string
	if !balance.SecurityID().IsEmpty() {
		securityIDStr := balance.SecurityID().String()
		securityID = &securityIDStr
	}

	return &dto.BalanceHistoryDTO{
		BalanceID:     balance.ID(),
		PortfolioID:   balance.PortfolioID().String(),
		SecurityID:    securityID,
		QuantityLong:  balance.QuantityLong().Value(),
		QuantityShort: balance.QuantityShort().Value(),
		Timestamp:     balance.LastUpdated(),
		ChangeType:    changeType,
		TransactionID: transactionID,
	}
}

// ValidateBalanceUpdateRequest validates a balance update request
func (m *BalanceMapper) ValidateBalanceUpdateRequest(request *dto.BalanceUpdateRequest) []dto.ValidationError {
	var errors []dto.ValidationError

	// Validate version
	if request.Version < 1 {
		errors = append(errors, dto.ValidationError{
			Field:   "version",
			Message: "must be positive",
			Value:   string(rune(request.Version)),
		})
	}

	// Validate that at least one quantity is provided
	if request.QuantityLong == nil && request.QuantityShort == nil {
		errors = append(errors, dto.ValidationError{
			Field:   "quantities",
			Message: "at least one quantity (long or short) must be provided",
			Value:   "",
		})
	}

	return errors
}

// ValidateBulkBalanceUpdateRequest validates a bulk balance update request
func (m *BalanceMapper) ValidateBulkBalanceUpdateRequest(request *dto.BulkBalanceUpdateRequest) []dto.ValidationError {
	var errors []dto.ValidationError

	// Validate update count
	if len(request.Updates) == 0 {
		errors = append(errors, dto.ValidationError{
			Field:   "updates",
			Message: "at least one update must be provided",
			Value:   "",
		})
	}

	if len(request.Updates) > 1000 {
		errors = append(errors, dto.ValidationError{
			Field:   "updates",
			Message: "cannot exceed 1000 updates per request",
			Value:   string(rune(len(request.Updates))),
		})
	}

	// Validate each update item
	for i, update := range request.Updates {
		if update.BalanceID <= 0 {
			errors = append(errors, dto.ValidationError{
				Field:   "updates[" + string(rune(i)) + "].balanceId",
				Message: "must be positive",
				Value:   string(rune(int(update.BalanceID))),
			})
		}

		if update.Version < 1 {
			errors = append(errors, dto.ValidationError{
				Field:   "updates[" + string(rune(i)) + "].version",
				Message: "must be positive",
				Value:   string(rune(update.Version)),
			})
		}

		if update.QuantityLong == nil && update.QuantityShort == nil {
			errors = append(errors, dto.ValidationError{
				Field:   "updates[" + string(rune(i)) + "].quantities",
				Message: "at least one quantity (long or short) must be provided",
				Value:   "",
			})
		}
	}

	return errors
}
