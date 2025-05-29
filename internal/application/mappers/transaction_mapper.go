package mappers

import (
	"fmt"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/application/dto"
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/domain/models"
)

// TransactionMapper handles mapping between Transaction domain models and DTOs
type TransactionMapper struct{}

// NewTransactionMapper creates a new transaction mapper
func NewTransactionMapper() *TransactionMapper {
	return &TransactionMapper{}
}

// ToResponseDTO converts a domain Transaction to TransactionResponseDTO
func (m *TransactionMapper) ToResponseDTO(transaction *models.Transaction) *dto.TransactionResponseDTO {
	if transaction == nil {
		return nil
	}

	var securityID *string
	if !transaction.SecurityID().IsEmpty() {
		securityIDStr := transaction.SecurityID().String()
		securityID = &securityIDStr
	}

	var errorMessage *string
	if transaction.ErrorMessage() != nil {
		errorMessage = transaction.ErrorMessage()
	}

	return &dto.TransactionResponseDTO{
		ID:                   transaction.ID(),
		PortfolioID:          transaction.PortfolioID().String(),
		SecurityID:           securityID,
		SourceID:             transaction.SourceID().String(),
		Status:               string(transaction.Status()),
		TransactionType:      string(transaction.TransactionType()),
		Quantity:             transaction.Quantity().Value(),
		Price:                transaction.Price().Value(),
		TransactionDate:      transaction.TransactionDate().Format("20060102"),
		ReprocessingAttempts: transaction.ReprocessingAttempts(),
		Version:              transaction.Version(),
		ErrorMessage:         errorMessage,
	}
}

// ToResponseDTOs converts multiple domain Transactions to TransactionResponseDTOs
func (m *TransactionMapper) ToResponseDTOs(transactions []*models.Transaction) []dto.TransactionResponseDTO {
	if transactions == nil {
		return nil
	}

	dtos := make([]dto.TransactionResponseDTO, len(transactions))
	for i, transaction := range transactions {
		if responseDTO := m.ToResponseDTO(transaction); responseDTO != nil {
			dtos[i] = *responseDTO
		}
	}
	return dtos
}

// FromPostDTO converts a TransactionPostDTO to domain Transaction
func (m *TransactionMapper) FromPostDTO(postDTO *dto.TransactionPostDTO) (*models.Transaction, error) {
	if postDTO == nil {
		return nil, fmt.Errorf("post DTO cannot be nil")
	}

	// Build transaction using the builder pattern with strings
	builder := models.NewTransactionBuilder().
		WithPortfolioID(postDTO.PortfolioID).
		WithSourceID(postDTO.SourceID).
		WithTransactionType(postDTO.TransactionType).
		WithQuantity(postDTO.Quantity).
		WithPrice(postDTO.Price).
		WithTransactionDateFromString(postDTO.TransactionDate)

	// Handle optional security ID
	if postDTO.SecurityID != nil && *postDTO.SecurityID != "" {
		builder = builder.WithSecurityIDFromString(*postDTO.SecurityID)
	}

	return builder.Build()
}

// FromPostDTOs converts multiple TransactionPostDTOs to domain Transactions
func (m *TransactionMapper) FromPostDTOs(postDTOs []dto.TransactionPostDTO) ([]*models.Transaction, []error) {
	if postDTOs == nil {
		return nil, nil
	}

	transactions := make([]*models.Transaction, 0, len(postDTOs))
	errors := make([]error, 0)

	for i, postDTO := range postDTOs {
		transaction, err := m.FromPostDTO(&postDTO)
		if err != nil {
			errors = append(errors, fmt.Errorf("transaction %d: %w", i, err))
			continue
		}
		transactions = append(transactions, transaction)
	}

	if len(errors) > 0 {
		return transactions, errors
	}

	return transactions, nil
}

// ToBatchResponse converts processing results to batch response
func (m *TransactionMapper) ToBatchResponse(successful []*models.Transaction, failed []dto.TransactionErrorDTO) dto.TransactionBatchResponse {
	successfulDTOs := m.ToResponseDTOs(successful)
	totalRequested := len(successful) + len(failed)
	successfulCount := len(successful)
	failedCount := len(failed)

	var successRate float64
	if totalRequested > 0 {
		successRate = float64(successfulCount) / float64(totalRequested) * 100
	}

	return dto.TransactionBatchResponse{
		Successful: successfulDTOs,
		Failed:     failed,
		Summary: dto.BatchSummaryDTO{
			TotalRequested: totalRequested,
			Successful:     successfulCount,
			Failed:         failedCount,
			SuccessRate:    successRate,
		},
	}
}

// ValidatePostDTO validates a TransactionPostDTO
func (m *TransactionMapper) ValidatePostDTO(postDTO *dto.TransactionPostDTO) []dto.ValidationError {
	var errors []dto.ValidationError

	// Validate portfolio ID length
	if len(postDTO.PortfolioID) != 24 {
		errors = append(errors, dto.ValidationError{
			Field:   "portfolioId",
			Message: "must be exactly 24 characters",
			Value:   postDTO.PortfolioID,
		})
	}

	// Validate security ID if provided
	if postDTO.SecurityID != nil && len(*postDTO.SecurityID) != 24 {
		errors = append(errors, dto.ValidationError{
			Field:   "securityId",
			Message: "must be exactly 24 characters",
			Value:   *postDTO.SecurityID,
		})
	}

	// Validate source ID length
	if len(postDTO.SourceID) > 50 {
		errors = append(errors, dto.ValidationError{
			Field:   "sourceId",
			Message: "must not exceed 50 characters",
			Value:   postDTO.SourceID,
		})
	}

	// Validate transaction type
	validTypes := []string{"BUY", "SELL", "SHORT", "COVER", "DEP", "WD", "IN", "OUT"}
	isValidType := false
	for _, validType := range validTypes {
		if postDTO.TransactionType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		errors = append(errors, dto.ValidationError{
			Field:   "transactionType",
			Message: "must be one of: BUY, SELL, SHORT, COVER, DEP, WD, IN, OUT",
			Value:   postDTO.TransactionType,
		})
	}

	// Validate price is positive
	if postDTO.Price.IsNegative() || postDTO.Price.IsZero() {
		errors = append(errors, dto.ValidationError{
			Field:   "price",
			Message: "must be positive",
			Value:   postDTO.Price.String(),
		})
	}

	// Validate transaction date format
	if _, err := time.Parse("20060102", postDTO.TransactionDate); err != nil {
		errors = append(errors, dto.ValidationError{
			Field:   "transactionDate",
			Message: "must be in YYYYMMDD format",
			Value:   postDTO.TransactionDate,
		})
	}

	// Business rule validation: DEP/WD transactions must not have security ID
	if (postDTO.TransactionType == "DEP" || postDTO.TransactionType == "WD") && postDTO.SecurityID != nil {
		errors = append(errors, dto.ValidationError{
			Field:   "securityId",
			Message: "must be null for DEP/WD transactions",
			Value:   *postDTO.SecurityID,
		})
	}

	// Business rule validation: Non-cash transactions must have security ID
	if (postDTO.TransactionType != "DEP" && postDTO.TransactionType != "WD") && postDTO.SecurityID == nil {
		errors = append(errors, dto.ValidationError{
			Field:   "securityId",
			Message: "is required for non-cash transactions",
			Value:   "",
		})
	}

	return errors
}
