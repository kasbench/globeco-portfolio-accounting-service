package cache

import (
	"context"
	"sync"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// MemoryCache implements Cache interface using in-memory storage
type MemoryCache struct {
	data    map[string]*memoryCacheItem
	mu      sync.RWMutex
	options MemoryCacheOptions
	logger  logger.Logger
}

// memoryCacheItem represents a cache item with TTL
type memoryCacheItem struct {
	value     []byte
	expiresAt time.Time
}

// MemoryCacheOptions contains configuration for memory cache
type MemoryCacheOptions struct {
	MaxEntries      int
	CleanupInterval time.Duration
	DefaultTTL      time.Duration
	KeyPrefix       string
	Logger          logger.Logger
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(options MemoryCacheOptions) *MemoryCache {
	if options.Logger == nil {
		options.Logger = logger.NewDevelopment()
	}

	cache := &MemoryCache{
		data:    make(map[string]*memoryCacheItem),
		options: options,
		logger:  options.Logger,
	}

	// Start cleanup goroutine
	if options.CleanupInterval > 0 {
		go cache.cleanup()
	}

	return cache
}

// Get retrieves a value from cache
func (mc *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists {
		return nil, NewCacheError("get", key, ErrKeyNotFound)
	}

	if time.Now().After(item.expiresAt) {
		return nil, NewCacheError("get", key, ErrKeyNotFound)
	}

	return item.value, nil
}

// Set stores a value in cache
func (mc *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = mc.options.DefaultTTL
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if we need to make room
	if len(mc.data) >= mc.options.MaxEntries {
		mc.evictLRU()
	}

	mc.data[key] = &memoryCacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a key from cache
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.data, key)
	return nil
}

// Exists checks if a key exists in cache
func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists {
		return false, nil
	}

	if time.Now().After(item.expiresAt) {
		return false, nil
	}

	return true, nil
}

// GetMultiple retrieves multiple values from cache
func (mc *MemoryCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for _, key := range keys {
		if value, err := mc.Get(ctx, key); err == nil {
			result[key] = value
		}
	}

	return result, nil
}

// SetMultiple stores multiple values in cache
func (mc *MemoryCache) SetMultiple(ctx context.Context, items map[string]CacheItem) error {
	for key, item := range items {
		if err := mc.Set(ctx, key, item.Value, item.TTL); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMultiple removes multiple keys from cache
func (mc *MemoryCache) DeleteMultiple(ctx context.Context, keys []string) error {
	for _, key := range keys {
		mc.Delete(ctx, key)
	}
	return nil
}

// DeletePattern removes all keys matching a pattern
func (mc *MemoryCache) DeletePattern(ctx context.Context, pattern string) error {
	keys, err := mc.GetKeys(ctx, pattern)
	if err != nil {
		return err
	}
	return mc.DeleteMultiple(ctx, keys)
}

// GetKeys retrieves all keys matching a pattern
func (mc *MemoryCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var keys []string
	for key := range mc.data {
		if matchesPattern(key, pattern) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// SetTTL sets TTL for an existing key
func (mc *MemoryCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	item, exists := mc.data[key]
	if !exists {
		return NewCacheError("set_ttl", key, ErrKeyNotFound)
	}

	item.expiresAt = time.Now().Add(ttl)
	return nil
}

// GetTTL gets the remaining TTL for a key
func (mc *MemoryCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists {
		return 0, NewCacheError("get_ttl", key, ErrKeyNotFound)
	}

	remaining := time.Until(item.expiresAt)
	if remaining < 0 {
		return 0, NewCacheError("get_ttl", key, ErrKeyNotFound)
	}

	return remaining, nil
}

// Clear removes all entries from cache
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.data = make(map[string]*memoryCacheItem)
	return nil
}

// Size returns the number of entries in cache
func (mc *MemoryCache) Size(ctx context.Context) (int64, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return int64(len(mc.data)), nil
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats(ctx context.Context) (*CacheStats, error) {
	size, _ := mc.Size(ctx)

	return &CacheStats{
		Size:           size,
		LastAccessTime: time.Now(),
		LastUpdateTime: time.Now(),
	}, nil
}

// Ping checks if cache is available
func (mc *MemoryCache) Ping(ctx context.Context) error {
	return nil // Memory cache is always available
}

// Close closes the cache
func (mc *MemoryCache) Close() error {
	return nil // Nothing to close for memory cache
}

// Helper methods

// cleanup removes expired entries
func (mc *MemoryCache) cleanup() {
	ticker := time.NewTicker(mc.options.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		mc.cleanupExpired()
	}
}

// cleanupExpired removes expired entries
func (mc *MemoryCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, item := range mc.data {
		if now.After(item.expiresAt) {
			delete(mc.data, key)
		}
	}
}

// evictLRU removes the least recently used entry (simplified implementation)
func (mc *MemoryCache) evictLRU() {
	// Simple implementation: remove the first key
	// In a real implementation, you'd track access times
	for key := range mc.data {
		delete(mc.data, key)
		break
	}
}

// matchesPattern checks if a key matches a wildcard pattern
func matchesPattern(key, pattern string) bool {
	// Simple implementation - exact match or wildcard
	if pattern == "*" {
		return true
	}
	return key == pattern
}

// NoopCache implements Cache interface with no-op operations
type NoopCache struct{}

// NewNoopCache creates a new no-op cache
func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

// Get always returns key not found
func (nc *NoopCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, NewCacheError("get", key, ErrKeyNotFound)
}

// Set does nothing
func (nc *NoopCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

// Delete does nothing
func (nc *NoopCache) Delete(ctx context.Context, key string) error {
	return nil
}

// Exists always returns false
func (nc *NoopCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// GetMultiple returns empty map
func (nc *NoopCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	return make(map[string][]byte), nil
}

// SetMultiple does nothing
func (nc *NoopCache) SetMultiple(ctx context.Context, items map[string]CacheItem) error {
	return nil
}

// DeleteMultiple does nothing
func (nc *NoopCache) DeleteMultiple(ctx context.Context, keys []string) error {
	return nil
}

// DeletePattern does nothing
func (nc *NoopCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

// GetKeys returns empty slice
func (nc *NoopCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	return []string{}, nil
}

// SetTTL does nothing
func (nc *NoopCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

// GetTTL returns zero duration
func (nc *NoopCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

// Clear does nothing
func (nc *NoopCache) Clear(ctx context.Context) error {
	return nil
}

// Size returns zero
func (nc *NoopCache) Size(ctx context.Context) (int64, error) {
	return 0, nil
}

// Stats returns empty stats
func (nc *NoopCache) Stats(ctx context.Context) (*CacheStats, error) {
	return &CacheStats{
		Size:           0,
		LastAccessTime: time.Now(),
		LastUpdateTime: time.Now(),
	}, nil
}

// Ping always succeeds
func (nc *NoopCache) Ping(ctx context.Context) error {
	return nil
}

// Close does nothing
func (nc *NoopCache) Close() error {
	return nil
}
