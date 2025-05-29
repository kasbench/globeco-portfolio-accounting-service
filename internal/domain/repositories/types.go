package repositories

import (
	"errors"
	"fmt"
	"strings"
)

// SortDirection represents the sort direction
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// String returns the string representation of sort direction
func (s SortDirection) String() string {
	return string(s)
}

// IsValid checks if the sort direction is valid
func (s SortDirection) IsValid() bool {
	return s == SortAsc || s == SortDesc
}

// SortField represents a field to sort by
type SortField struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}

// String returns the string representation of sort field
func (s SortField) String() string {
	return fmt.Sprintf("%s %s", s.Field, s.Direction)
}

// Validate validates the sort field
func (s SortField) Validate(allowedFields []string) error {
	if s.Field == "" {
		return errors.New("sort field cannot be empty")
	}

	if !s.Direction.IsValid() {
		return fmt.Errorf("invalid sort direction: %s", s.Direction)
	}

	// Check if field is allowed
	for _, allowed := range allowedFields {
		if s.Field == allowed {
			return nil
		}
	}

	return fmt.Errorf("field '%s' is not allowed for sorting", s.Field)
}

// ParseSortFields parses a comma-separated string of sort fields
// Format: "field1:asc,field2:desc" or "field1,field2:desc"
func ParseSortFields(sortBy string) ([]SortField, error) {
	if sortBy == "" {
		return nil, nil
	}

	fields := strings.Split(sortBy, ",")
	sortFields := make([]SortField, 0, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		parts := strings.Split(field, ":")
		if len(parts) > 2 {
			return nil, fmt.Errorf("invalid sort field format: %s", field)
		}

		sortField := SortField{
			Field:     strings.TrimSpace(parts[0]),
			Direction: SortAsc, // Default direction
		}

		if len(parts) == 2 {
			direction := strings.ToUpper(strings.TrimSpace(parts[1]))
			switch direction {
			case "ASC", "A":
				sortField.Direction = SortAsc
			case "DESC", "D":
				sortField.Direction = SortDesc
			default:
				return nil, fmt.Errorf("invalid sort direction: %s", direction)
			}
		}

		sortFields = append(sortFields, sortField)
	}

	return sortFields, nil
}

// QueryResult represents the result of a query with pagination info
type QueryResult[T any] struct {
	Items      []T   `json:"items"`
	TotalCount int64 `json:"totalCount"`
	Offset     int   `json:"offset"`
	Limit      int   `json:"limit"`
	HasMore    bool  `json:"hasMore"`
}

// NewQueryResult creates a new query result
func NewQueryResult[T any](items []T, totalCount int64, offset, limit int) *QueryResult[T] {
	hasMore := int64(offset+len(items)) < totalCount

	return &QueryResult[T]{
		Items:      items,
		TotalCount: totalCount,
		Offset:     offset,
		Limit:      limit,
		HasMore:    hasMore,
	}
}

// IsEmpty returns true if the result has no items
func (r *QueryResult[T]) IsEmpty() bool {
	return len(r.Items) == 0
}

// GetPageInfo returns pagination information
func (r *QueryResult[T]) GetPageInfo() map[string]interface{} {
	var page int
	if r.Limit > 0 {
		page = (r.Offset / r.Limit) + 1
	} else {
		page = 1
	}

	return map[string]interface{}{
		"offset":     r.Offset,
		"limit":      r.Limit,
		"totalCount": r.TotalCount,
		"hasMore":    r.HasMore,
		"page":       page,
	}
}
