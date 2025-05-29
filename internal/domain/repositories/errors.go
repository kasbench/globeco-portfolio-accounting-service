package repositories

import (
	"errors"
	"fmt"
)

// Common repository errors
var (
	ErrNotFound            = errors.New("entity not found")
	ErrDuplicateKey        = errors.New("duplicate key violation")
	ErrOptimisticLock      = errors.New("optimistic locking failure")
	ErrInvalidFilter       = errors.New("invalid filter parameters")
	ErrInvalidPagination   = errors.New("invalid pagination parameters")
	ErrConnectionFailed    = errors.New("database connection failed")
	ErrTransactionFailed   = errors.New("database transaction failed")
	ErrConstraintViolation = errors.New("database constraint violation")
)

// RepositoryError wraps errors with additional context
type RepositoryError struct {
	Operation string
	Entity    string
	Cause     error
	Context   map[string]interface{}
}

// Error implements the error interface
func (e *RepositoryError) Error() string {
	if e.Context != nil && len(e.Context) > 0 {
		return fmt.Sprintf("repository error in %s for %s: %v (context: %v)",
			e.Operation, e.Entity, e.Cause, e.Context)
	}
	return fmt.Sprintf("repository error in %s for %s: %v",
		e.Operation, e.Entity, e.Cause)
}

// Unwrap returns the underlying error
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
		Context:   make(map[string]interface{}),
	}
}

// WithContext adds context information to the error
func (e *RepositoryError) WithContext(key string, value interface{}) *RepositoryError {
	e.Context[key] = value
	return e
}

// Repository error constructors for common scenarios

// NewNotFoundError creates a not found error
func NewNotFoundError(entity string, identifier interface{}) *RepositoryError {
	return NewRepositoryError("find", entity, ErrNotFound).
		WithContext("identifier", identifier)
}

// NewDuplicateKeyError creates a duplicate key error
func NewDuplicateKeyError(entity string, field, value interface{}) *RepositoryError {
	return NewRepositoryError("create", entity, ErrDuplicateKey).
		WithContext("field", field).
		WithContext("value", value)
}

// NewOptimisticLockError creates an optimistic locking error
func NewOptimisticLockError(entity string, id, expectedVersion, actualVersion interface{}) *RepositoryError {
	return NewRepositoryError("update", entity, ErrOptimisticLock).
		WithContext("id", id).
		WithContext("expectedVersion", expectedVersion).
		WithContext("actualVersion", actualVersion)
}

// NewConstraintViolationError creates a constraint violation error
func NewConstraintViolationError(entity string, constraint string, cause error) *RepositoryError {
	return NewRepositoryError("persist", entity, ErrConstraintViolation).
		WithContext("constraint", constraint).
		WithContext("details", cause.Error())
}

// NewTransactionError creates a transaction error
func NewTransactionError(operation string, cause error) *RepositoryError {
	return NewRepositoryError(operation, "transaction", ErrTransactionFailed).
		WithContext("details", cause.Error())
}

// NewConnectionError creates a connection error
func NewConnectionError(operation string, cause error) *RepositoryError {
	return NewRepositoryError(operation, "connection", ErrConnectionFailed).
		WithContext("details", cause.Error())
}

// Helper functions to check error types

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicateKeyError checks if the error is a duplicate key error
func IsDuplicateKeyError(err error) bool {
	return errors.Is(err, ErrDuplicateKey)
}

// IsOptimisticLockError checks if the error is an optimistic locking error
func IsOptimisticLockError(err error) bool {
	return errors.Is(err, ErrOptimisticLock)
}

// IsConstraintViolationError checks if the error is a constraint violation error
func IsConstraintViolationError(err error) bool {
	return errors.Is(err, ErrConstraintViolation)
}

// IsTransactionError checks if the error is a transaction error
func IsTransactionError(err error) bool {
	return errors.Is(err, ErrTransactionFailed)
}

// IsConnectionError checks if the error is a connection error
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}
