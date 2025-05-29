package cache

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations
	GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error)
	SetMultiple(ctx context.Context, items map[string]CacheItem) error
	DeleteMultiple(ctx context.Context, keys []string) error

	// Pattern operations
	DeletePattern(ctx context.Context, pattern string) error
	GetKeys(ctx context.Context, pattern string) ([]string, error)

	// TTL operations
	SetTTL(ctx context.Context, key string, ttl time.Duration) error
	GetTTL(ctx context.Context, key string) (time.Duration, error)

	// Cache management
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
	Stats(ctx context.Context) (*CacheStats, error)

	// Health check
	Ping(ctx context.Context) error

	// Connection management
	Close() error
}

// CacheItem represents a cache item with value and TTL
type CacheItem struct {
	Key   string        `json:"key"`
	Value []byte        `json:"value"`
	TTL   time.Duration `json:"ttl"`
}

// CacheStats provides cache statistics
type CacheStats struct {
	HitCount       int64     `json:"hit_count"`
	MissCount      int64     `json:"miss_count"`
	HitRate        float64   `json:"hit_rate"`
	MissRate       float64   `json:"miss_rate"`
	EvictionCount  int64     `json:"eviction_count"`
	Size           int64     `json:"size"`
	MemoryUsage    int64     `json:"memory_usage"`
	MaxMemoryUsage int64     `json:"max_memory_usage"`
	LastAccessTime time.Time `json:"last_access_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

// CacheError represents cache-specific errors
type CacheError struct {
	Operation string
	Key       string
	Cause     error
}

func (e *CacheError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("cache error in %s for key '%s': %v", e.Operation, e.Key, e.Cause)
	}
	return fmt.Sprintf("cache error in %s: %v", e.Operation, e.Cause)
}

func (e *CacheError) Unwrap() error {
	return e.Cause
}

// Common cache errors
var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrKeyExists       = errors.New("key already exists")
	ErrInvalidTTL      = errors.New("invalid TTL")
	ErrCacheNotReady   = errors.New("cache not ready")
	ErrOperationFailed = errors.New("cache operation failed")
)

// NewCacheError creates a new cache error
func NewCacheError(operation, key string, cause error) *CacheError {
	return &CacheError{
		Operation: operation,
		Key:       key,
		Cause:     cause,
	}
}

// IsKeyNotFoundError checks if the error is a key not found error
func IsKeyNotFoundError(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

// IsCacheNotReadyError checks if the error is a cache not ready error
func IsCacheNotReadyError(err error) bool {
	return errors.Is(err, ErrCacheNotReady)
}
