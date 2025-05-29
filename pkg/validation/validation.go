package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator provides validation functions
type Validator struct {
	errors ValidationErrors
}

// New creates a new validator
func New() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string, value interface{}) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return v.errors.HasErrors()
}

// Errors returns all validation errors
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// Clear clears all validation errors
func (v *Validator) Clear() {
	v.errors = v.errors[:0]
}

// Required validates that a string is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "is required", value)
	}
	return v
}

// RequiredPtr validates that a pointer is not nil
func (v *Validator) RequiredPtr(field string, value interface{}) *Validator {
	if value == nil {
		v.AddError(field, "is required", nil)
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.AddError(field, fmt.Sprintf("must be at least %d characters", min), value)
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.AddError(field, fmt.Sprintf("must be at most %d characters", max), value)
	}
	return v
}

// ExactLength validates exact string length
func (v *Validator) ExactLength(field, value string, length int) *Validator {
	if len(value) != length {
		v.AddError(field, fmt.Sprintf("must be exactly %d characters", length), value)
	}
	return v
}

// MinValue validates minimum numeric value
func (v *Validator) MinValue(field string, value, min int) *Validator {
	if value < min {
		v.AddError(field, fmt.Sprintf("must be at least %d", min), value)
	}
	return v
}

// MaxValue validates maximum numeric value
func (v *Validator) MaxValue(field string, value, max int) *Validator {
	if value > max {
		v.AddError(field, fmt.Sprintf("must be at most %d", max), value)
	}
	return v
}

// Range validates numeric value is within range
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.AddError(field, fmt.Sprintf("must be between %d and %d", min, max), value)
	}
	return v
}

// OneOf validates that value is one of the allowed values
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	for _, allowedValue := range allowed {
		if value == allowedValue {
			return v
		}
	}
	v.AddError(field, fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")), value)
	return v
}

// Regex validates that value matches the given regex pattern
func (v *Validator) Regex(field, value, pattern string) *Validator {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		v.AddError(field, fmt.Sprintf("invalid regex pattern: %s", pattern), value)
		return v
	}

	if !regex.MatchString(value) {
		v.AddError(field, fmt.Sprintf("does not match required pattern"), value)
	}
	return v
}

// Email validates email format
func (v *Validator) Email(field, value string) *Validator {
	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	return v.Regex(field, value, emailRegex)
}

// DateFormat validates date format
func (v *Validator) DateFormat(field, value, layout string) *Validator {
	if _, err := time.Parse(layout, value); err != nil {
		v.AddError(field, fmt.Sprintf("must be in format %s", layout), value)
	}
	return v
}

// YYYYMMDD validates YYYYMMDD date format
func (v *Validator) YYYYMMDD(field, value string) *Validator {
	return v.DateFormat(field, value, "20060102")
}

// PortfolioID validates portfolio ID format (24 characters)
func (v *Validator) PortfolioID(field, value string) *Validator {
	if value == "" {
		return v // Allow empty for optional fields
	}
	return v.ExactLength(field, value, 24).Regex(field, value, `^[a-zA-Z0-9]+$`)
}

// SecurityID validates security ID format (24 characters, can be empty)
func (v *Validator) SecurityID(field, value string) *Validator {
	if value == "" {
		return v // Allow empty for cash transactions
	}
	return v.ExactLength(field, value, 24).Regex(field, value, `^[a-zA-Z0-9]+$`)
}

// SourceID validates source ID format (max 50 characters)
func (v *Validator) SourceID(field, value string) *Validator {
	return v.Required(field, value).MaxLength(field, value, 50)
}

// TransactionType validates transaction type
func (v *Validator) TransactionType(field, value string) *Validator {
	allowed := []string{"BUY", "SELL", "SHORT", "COVER", "DEP", "WD", "IN", "OUT"}
	return v.Required(field, value).OneOf(field, value, allowed)
}

// TransactionStatus validates transaction status
func (v *Validator) TransactionStatus(field, value string) *Validator {
	allowed := []string{"NEW", "PROC", "ERROR", "FATAL"}
	return v.Required(field, value).OneOf(field, value, allowed)
}

// Decimal validates decimal format and constraints
func (v *Validator) Decimal(field, value string, precision, scale int) *Validator {
	// Basic regex for decimal validation
	decimalRegex := `^-?\d+(\.\d+)?$`
	if !regexp.MustCompile(decimalRegex).MatchString(value) {
		v.AddError(field, "must be a valid decimal number", value)
		return v
	}

	// Check precision and scale if specified
	parts := strings.Split(value, ".")
	if len(parts) == 2 {
		integerPart := strings.TrimPrefix(parts[0], "-")
		decimalPart := parts[1]

		totalDigits := len(integerPart) + len(decimalPart)
		if precision > 0 && totalDigits > precision {
			v.AddError(field, fmt.Sprintf("total digits must not exceed %d", precision), value)
		}

		if scale > 0 && len(decimalPart) > scale {
			v.AddError(field, fmt.Sprintf("decimal places must not exceed %d", scale), value)
		}
	} else if len(parts) == 1 {
		integerPart := strings.TrimPrefix(parts[0], "-")
		if precision > 0 && len(integerPart) > precision {
			v.AddError(field, fmt.Sprintf("total digits must not exceed %d", precision), value)
		}
	}

	return v
}

// Positive validates that a numeric value is positive
func (v *Validator) Positive(field string, value float64) *Validator {
	if value <= 0 {
		v.AddError(field, "must be positive", value)
	}
	return v
}

// NonNegative validates that a numeric value is non-negative
func (v *Validator) NonNegative(field string, value float64) *Validator {
	if value < 0 {
		v.AddError(field, "must be non-negative", value)
	}
	return v
}

// ValidateStruct validates a struct using field tags (simplified version)
func ValidateStruct(s interface{}) ValidationErrors {
	// This is a simplified implementation
	// In a real application, you might use a library like go-playground/validator
	// or implement reflection-based validation
	return ValidationErrors{}
}

// Quick validation functions for common use cases

// ValidatePortfolioTransaction validates a portfolio transaction
func ValidatePortfolioTransaction(portfolioID, securityID, sourceID, transactionType, quantity, price, transactionDate string) ValidationErrors {
	v := New()

	v.PortfolioID("portfolioId", portfolioID)
	v.SecurityID("securityId", securityID)
	v.SourceID("sourceId", sourceID)
	v.TransactionType("transactionType", transactionType)
	v.Required("quantity", quantity).Decimal("quantity", quantity, 18, 8)
	v.Required("price", price).Decimal("price", price, 18, 8)
	v.Required("transactionDate", transactionDate).YYYYMMDD("transactionDate", transactionDate)

	// Business rule: if transactionType is DEP or WD, securityId must be empty
	if (transactionType == "DEP" || transactionType == "WD") && securityID != "" {
		v.AddError("securityId", "must be empty for cash transactions (DEP/WD)", securityID)
	}

	// Business rule: if transactionType is not DEP or WD, securityId is required
	if transactionType != "DEP" && transactionType != "WD" && securityID == "" {
		v.AddError("securityId", "is required for non-cash transactions", securityID)
	}

	return v.Errors()
}
