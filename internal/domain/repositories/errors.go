package repositories

import (
	"errors"
	"fmt"
)

// Standard repository errors
var (
	// ErrNotFound indicates that the requested entity was not found
	ErrNotFound = errors.New("entity not found")

	// ErrDuplicateKey indicates a unique constraint violation
	ErrDuplicateKey = errors.New("duplicate key constraint violation")

	// ErrOptimisticLock indicates an optimistic locking conflict
	ErrOptimisticLock = errors.New("optimistic locking conflict")

	// ErrInvalidFilter indicates invalid filter parameters
	ErrInvalidFilter = errors.New("invalid filter parameters")

	// ErrConnectionFailed indicates database connection failure
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrTransactionFailed indicates transaction failure
	ErrTransactionFailed = errors.New("database transaction failed")

	// ErrConstraintViolation indicates a database constraint violation
	ErrConstraintViolation = errors.New("database constraint violation")
)

// RepositoryError wraps repository errors with additional context
type RepositoryError struct {
	Operation string                 // The operation that failed (e.g., "Create", "Update", "GetByID")
	Entity    string                 // The entity type (e.g., "Transaction", "Balance")
	Cause     error                  // The underlying error
	Details   map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *RepositoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("repository error: %s %s failed: %v", e.Operation, e.Entity, e.Cause)
	}
	return fmt.Sprintf("repository error: %s %s failed", e.Operation, e.Entity)
}

// Unwrap returns the underlying error for error unwrapping
func (e *RepositoryError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error
func (e *RepositoryError) Is(target error) bool {
	return errors.Is(e.Cause, target)
}

// NewRepositoryError creates a new repository error
func NewRepositoryError(operation, entity string, cause error) *RepositoryError {
	return &RepositoryError{
		Operation: operation,
		Entity:    entity,
		Cause:     cause,
		Details:   make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the repository error
func (e *RepositoryError) WithDetail(key string, value interface{}) *RepositoryError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// IsNotFound checks if the error is a "not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicateKey checks if the error is a duplicate key error
func IsDuplicateKey(err error) bool {
	return errors.Is(err, ErrDuplicateKey)
}

// IsOptimisticLock checks if the error is an optimistic locking error
func IsOptimisticLock(err error) bool {
	return errors.Is(err, ErrOptimisticLock)
}

// IsConstraintViolation checks if the error is a constraint violation
func IsConstraintViolation(err error) bool {
	return errors.Is(err, ErrConstraintViolation)
}

// WrapDatabaseError wraps database errors with appropriate repository errors
func WrapDatabaseError(operation, entity string, err error) error {
	if err == nil {
		return nil
	}

	// Map common database errors to repository errors
	errMsg := err.Error()

	// PostgreSQL specific error codes
	switch {
	case contains(errMsg, "no rows in result set"):
		return NewRepositoryError(operation, entity, ErrNotFound)
	case contains(errMsg, "duplicate key value violates unique constraint"):
		return NewRepositoryError(operation, entity, ErrDuplicateKey)
	case contains(errMsg, "check constraint"):
		return NewRepositoryError(operation, entity, ErrConstraintViolation)
	case contains(errMsg, "foreign key constraint"):
		return NewRepositoryError(operation, entity, ErrConstraintViolation)
	case contains(errMsg, "connection refused"):
		return NewRepositoryError(operation, entity, ErrConnectionFailed)
	default:
		return NewRepositoryError(operation, entity, err)
	}
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
