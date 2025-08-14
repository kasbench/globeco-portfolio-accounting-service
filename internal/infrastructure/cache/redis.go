package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client     *redis.Client
	logger     logger.Logger
	config     RedisConfig
	keyService *CacheKeyService
}

// RedisConfig holds configuration for Redis client
type RedisConfig struct {
	Address           string        `mapstructure:"address" json:"address"`
	Password          string        `mapstructure:"password" json:"password"`
	Database          int           `mapstructure:"database" json:"database"`
	PoolSize          int           `mapstructure:"pool_size" json:"pool_size"`
	MinIdleConns      int           `mapstructure:"min_idle_conns" json:"min_idle_conns"`
	MaxRetries        int           `mapstructure:"max_retries" json:"max_retries"`
	DialTimeout       time.Duration `mapstructure:"dial_timeout" json:"dial_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout" json:"write_timeout"`
	PoolTimeout       time.Duration `mapstructure:"pool_timeout" json:"pool_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout" json:"idle_timeout"`
	IdleCheckFreq     time.Duration `mapstructure:"idle_check_freq" json:"idle_check_freq"`
	KeyPrefix         string        `mapstructure:"key_prefix" json:"key_prefix"`
	EnableMetrics     bool          `mapstructure:"enable_metrics" json:"enable_metrics"`
	EnableLogging     bool          `mapstructure:"enable_logging" json:"enable_logging"`
	TLSEnabled        bool          `mapstructure:"tls_enabled" json:"tls_enabled"`
	TLSSkipVerify     bool          `mapstructure:"tls_skip_verify" json:"tls_skip_verify"`
}

// NewRedisCache creates a new Redis cache implementation
func NewRedisCache(cfg RedisConfig, lg logger.Logger) (*RedisCache, error) {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Create Redis client options
	opts := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.Database,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create key service
	keyService := NewCacheKeyService(cfg.KeyPrefix)

	cache := &RedisCache{
		client:     client,
		logger:     lg,
		config:     cfg,
		keyService: keyService,
	}

	lg.Info("Redis cache initialized",
		logger.String("address", cfg.Address),
		logger.Int("database", cfg.Database))

	return cache, nil
}

// Get retrieves a value from the cache
func (rc *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := rc.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, NewCacheError("get", key, ErrKeyNotFound)
		}
		return nil, NewCacheError("get", key, err)
	}

	data := []byte(result)

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache hit",
			logger.String("key", key),
			logger.Int("size", len(data)))
	}

	return data, nil
}

// Set stores a value in the cache with TTL
func (rc *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var err error
	if ttl > 0 {
		err = rc.client.Set(ctx, key, value, ttl).Err()
	} else {
		err = rc.client.Set(ctx, key, value, 0).Err()
	}

	if err != nil {
		return NewCacheError("set", key, err)
	}

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache set",
			logger.String("key", key),
			logger.Int("size", len(value)),
			logger.Duration("ttl", ttl))
	}

	return nil
}

// Delete removes a key from the cache
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	err := rc.client.Del(ctx, key).Err()
	if err != nil {
		return NewCacheError("delete", key, err)
	}

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache delete",
			logger.String("key", key))
	}

	return nil
}

// Exists checks if a key exists in the cache
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := rc.client.Exists(ctx, key).Result()
	if err != nil {
		return false, NewCacheError("exists", key, err)
	}

	return result > 0, nil
}

// GetMultiple retrieves multiple values from the cache
func (rc *RedisCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// Convert string keys to interface{}
	keyInterfaces := make([]string, len(keys))
	copy(keyInterfaces, keys)

	values, err := rc.client.MGet(ctx, keyInterfaces...).Result()
	if err != nil {
		return nil, NewCacheError("get_multiple", strings.Join(keys, ","), err)
	}

	result := make(map[string][]byte)
	for i, value := range values {
		if value != nil {
			if strValue, ok := value.(string); ok {
				result[keys[i]] = []byte(strValue)
			}
		}
	}

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache get multiple",
			logger.Int("requested", len(keys)),
			logger.Int("found", len(result)))
	}

	return result, nil
}

// SetMultiple stores multiple values in the cache
func (rc *RedisCache) SetMultiple(ctx context.Context, items map[string]CacheItem) error {
	if len(items) == 0 {
		return nil
	}

	// Use pipeline for better performance
	pipe := rc.client.Pipeline()

	for key, item := range items {
		if item.TTL > 0 {
			pipe.Set(ctx, key, item.Value, item.TTL)
		} else {
			pipe.Set(ctx, key, item.Value, 0)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return NewCacheError("set_multiple", "pipeline", err)
	}

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache set multiple",
			logger.Int("count", len(items)))
	}

	return nil
}

// DeleteMultiple removes multiple keys from the cache
func (rc *RedisCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	err := rc.client.Del(ctx, keys...).Err()
	if err != nil {
		return NewCacheError("delete_multiple", strings.Join(keys, ","), err)
	}

	if rc.config.EnableLogging {
		rc.logger.Debug("Cache delete multiple",
			logger.Int("count", len(keys)))
	}

	return nil
}

// DeletePattern removes all keys matching a pattern
func (rc *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	keys, err := rc.GetKeys(ctx, pattern)
	if err != nil {
		return NewCacheError("delete_pattern", pattern, err)
	}

	if len(keys) == 0 {
		return nil
	}

	return rc.DeleteMultiple(ctx, keys)
}

// GetKeys retrieves all keys matching a pattern
func (rc *RedisCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	keys, err := rc.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, NewCacheError("get_keys", pattern, err)
	}

	return keys, nil
}

// SetTTL sets TTL for an existing key
func (rc *RedisCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	var err error
	if ttl > 0 {
		err = rc.client.Expire(ctx, key, ttl).Err()
	} else {
		err = rc.client.Persist(ctx, key).Err()
	}

	if err != nil {
		return NewCacheError("set_ttl", key, err)
	}

	return nil
}

// GetTTL gets the remaining TTL for a key
func (rc *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := rc.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, NewCacheError("get_ttl", key, err)
	}

	return ttl, nil
}

// Clear removes all entries from the cache
func (rc *RedisCache) Clear(ctx context.Context) error {
	err := rc.client.FlushDB(ctx).Err()
	if err != nil {
		return NewCacheError("clear", "", err)
	}

	rc.logger.Info("Cache cleared")
	return nil
}

// Size returns the number of entries in the cache
func (rc *RedisCache) Size(ctx context.Context) (int64, error) {
	size, err := rc.client.DBSize(ctx).Result()
	if err != nil {
		return 0, NewCacheError("size", "", err)
	}

	return size, nil
}

// Stats returns cache statistics
func (rc *RedisCache) Stats(ctx context.Context) (*CacheStats, error) {
	size, err := rc.Size(ctx)
	if err != nil {
		return nil, err
	}

	// Get Redis info for more detailed stats
	info, err := rc.client.Info(ctx, "stats").Result()
	if err != nil {
		// If we can't get detailed stats, return basic stats
		return &CacheStats{
			Size:           size,
			LastAccessTime: time.Now(),
			LastUpdateTime: time.Now(),
		}, nil
	}

	stats := &CacheStats{
		Size:           size,
		LastAccessTime: time.Now(),
		LastUpdateTime: time.Now(),
	}

	// Parse Redis info for hit/miss stats
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "keyspace_hits:") {
			if hits, err := parseRedisInfoValue(line); err == nil {
				stats.HitCount = hits
			}
		} else if strings.HasPrefix(line, "keyspace_misses:") {
			if misses, err := parseRedisInfoValue(line); err == nil {
				stats.MissCount = misses
			}
		} else if strings.HasPrefix(line, "evicted_keys:") {
			if evictions, err := parseRedisInfoValue(line); err == nil {
				stats.EvictionCount = evictions
			}
		}
	}

	// Calculate hit/miss rates
	total := stats.HitCount + stats.MissCount
	if total > 0 {
		stats.HitRate = float64(stats.HitCount) / float64(total)
		stats.MissRate = float64(stats.MissCount) / float64(total)
	}

	return stats, nil
}

// Ping checks if the cache is available
func (rc *RedisCache) Ping(ctx context.Context) error {
	err := rc.client.Ping(ctx).Err()
	if err != nil {
		return NewCacheError("ping", "", err)
	}

	return nil
}

// Close closes the cache connection
func (rc *RedisCache) Close() error {
	if rc.client != nil {
		err := rc.client.Close()
		rc.logger.Info("Redis cache connection closed")
		return err
	}
	return nil
}

// Helper functions

// parseRedisInfoValue parses a value from Redis INFO command output
func parseRedisInfoValue(line string) (int64, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid info line format: %s", line)
	}

	var value int64
	if err := json.Unmarshal([]byte(parts[1]), &value); err != nil {
		return 0, fmt.Errorf("failed to parse value: %w", err)
	}

	return value, nil
}