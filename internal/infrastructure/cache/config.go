package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// CacheType represents the type of cache implementation
type CacheType string

const (
	// CacheTypeRedis represents Redis cache implementation
	CacheTypeRedis CacheType = "redis"
	// CacheTypeMemory represents in-memory cache implementation
	CacheTypeMemory CacheType = "memory"
	// CacheTypeNoop represents no-op cache implementation for testing
	CacheTypeNoop CacheType = "noop"
)

// Config represents the main cache configuration
type Config struct {
	// Type specifies which cache implementation to use
	Type CacheType `mapstructure:"type" json:"type"`

	// Enabled determines if caching is active
	Enabled bool `mapstructure:"enabled" json:"enabled"`

	// KeyPrefix is prepended to all cache keys
	KeyPrefix string `mapstructure:"key_prefix" json:"key_prefix"`

	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL time.Duration `mapstructure:"default_ttl" json:"default_ttl"`

	// Redis specific configuration
	Redis RedisConfig `mapstructure:"redis" json:"redis"`

	// Memory cache specific configuration
	Memory MemoryCacheConfig `mapstructure:"memory" json:"memory"`

	// Metrics configuration
	EnableMetrics bool `mapstructure:"enable_metrics" json:"enable_metrics"`

	// Logging configuration
	EnableLogging bool `mapstructure:"enable_logging" json:"enable_logging"`
}

// MemoryCacheConfig holds configuration for in-memory cache
type MemoryCacheConfig struct {
	// MaxEntries is the maximum number of entries to store
	MaxEntries int `mapstructure:"max_entries" json:"max_entries"`

	// CleanupInterval is how often to clean expired entries
	CleanupInterval time.Duration `mapstructure:"cleanup_interval" json:"cleanup_interval"`
}

// Validate validates the cache configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if cache is disabled
	}

	if c.Type == "" {
		return fmt.Errorf("cache type must be specified when enabled")
	}

	switch c.Type {
	case CacheTypeRedis:
		if err := c.Redis.Validate(); err != nil {
			return fmt.Errorf("redis config validation failed: %w", err)
		}
	case CacheTypeMemory:
		if err := c.Memory.Validate(); err != nil {
			return fmt.Errorf("memory cache config validation failed: %w", err)
		}
	case CacheTypeNoop:
		// No validation needed for noop cache
	default:
		return fmt.Errorf("unsupported cache type: %s", c.Type)
	}

	if c.DefaultTTL < 0 {
		return fmt.Errorf("default TTL cannot be negative")
	}

	return nil
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	if c.Type == "" {
		c.Type = CacheTypeRedis
	}

	if c.KeyPrefix == "" {
		c.KeyPrefix = "portfolio-accounting"
	}

	if c.DefaultTTL == 0 {
		c.DefaultTTL = 15 * time.Minute
	}

	// Set defaults for Redis
	c.Redis.SetDefaults()

	// Set defaults for Memory cache
	c.Memory.SetDefaults()
}

// Validate validates the Redis configuration
func (rc *RedisConfig) Validate() error {
	if rc.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	if rc.Database < 0 {
		return fmt.Errorf("redis database cannot be negative")
	}

	if rc.PoolSize < 0 {
		return fmt.Errorf("pool size cannot be negative")
	}

	if rc.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	return nil
}

// SetDefaults sets default values for Redis configuration
func (rc *RedisConfig) SetDefaults() {
	if rc.Address == "" {
		rc.Address = "globeco-portfolio-accounting-service-redis:6379"
	}

	if rc.Database == 0 {
		rc.Database = 0
	}

	if rc.PoolSize == 0 {
		rc.PoolSize = 10
	}

	if rc.MinIdleConns == 0 {
		rc.MinIdleConns = 2
	}

	if rc.MaxRetries == 0 {
		rc.MaxRetries = 3
	}

	if rc.DialTimeout == 0 {
		rc.DialTimeout = 5 * time.Second
	}

	if rc.ReadTimeout == 0 {
		rc.ReadTimeout = 5 * time.Second
	}

	if rc.WriteTimeout == 0 {
		rc.WriteTimeout = 5 * time.Second
	}

	if rc.PoolTimeout == 0 {
		rc.PoolTimeout = 5 * time.Second
	}

	if rc.IdleTimeout == 0 {
		rc.IdleTimeout = 5 * time.Minute
	}

	if rc.IdleCheckFreq == 0 {
		rc.IdleCheckFreq = 1 * time.Minute
	}
}

// Validate validates the memory cache configuration
func (mc *MemoryCacheConfig) Validate() error {
	if mc.MaxEntries < 0 {
		return fmt.Errorf("max entries cannot be negative")
	}

	if mc.CleanupInterval < 0 {
		return fmt.Errorf("cleanup interval cannot be negative")
	}

	return nil
}

// SetDefaults sets default values for memory cache configuration
func (mc *MemoryCacheConfig) SetDefaults() {
	if mc.MaxEntries == 0 {
		mc.MaxEntries = 10000
	}

	if mc.CleanupInterval == 0 {
		mc.CleanupInterval = 10 * time.Minute
	}
}

// CacheFactory creates cache instances based on configuration
type CacheFactory struct {
	logger logger.Logger
}

// NewCacheFactory creates a new cache factory
func NewCacheFactory(lg logger.Logger) *CacheFactory {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	return &CacheFactory{
		logger: lg,
	}
}

// CreateCache creates a cache instance based on the configuration
func (cf *CacheFactory) CreateCache(config Config) (Cache, error) {
	if !config.Enabled {
		cf.logger.Info("Cache is disabled, using noop cache")
		return NewNoopCache(), nil
	}

	// Set defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("cache config validation failed: %w", err)
	}

	switch config.Type {
	case CacheTypeRedis:
		return cf.createRedisCache(config)
	case CacheTypeMemory:
		return cf.createMemoryCache(config)
	case CacheTypeNoop:
		return NewNoopCache(), nil
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", config.Type)
	}
}

// createRedisCache creates a Redis cache instance
func (cf *CacheFactory) createRedisCache(config Config) (Cache, error) {
	redisConfig := config.Redis
	redisConfig.KeyPrefix = config.KeyPrefix
	redisConfig.EnableMetrics = config.EnableMetrics
	redisConfig.EnableLogging = config.EnableLogging

	cache, err := NewRedisCache(redisConfig, cf.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis cache: %w", err)
	}

	cf.logger.Info("Redis cache created successfully",
		logger.String("address", redisConfig.Address),
		logger.Int("database", redisConfig.Database))

	return cache, nil
}

// createMemoryCache creates an in-memory cache instance
func (cf *CacheFactory) createMemoryCache(config Config) (Cache, error) {
	memoryConfig := config.Memory

	cache := NewMemoryCache(MemoryCacheOptions{
		MaxEntries:      memoryConfig.MaxEntries,
		CleanupInterval: memoryConfig.CleanupInterval,
		DefaultTTL:      config.DefaultTTL,
		KeyPrefix:       config.KeyPrefix,
		Logger:          cf.logger,
	})

	cf.logger.Info("Memory cache created successfully",
		logger.Int("maxEntries", memoryConfig.MaxEntries),
		logger.Duration("cleanupInterval", memoryConfig.CleanupInterval))

	return cache, nil
}

// CacheManager manages cache instances and provides unified access
type CacheManager struct {
	cache      Cache
	cacheAside *CacheAsideManager
	config     Config
	logger     logger.Logger
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config Config, lg logger.Logger) (*CacheManager, error) {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	factory := NewCacheFactory(lg)
	cache, err := factory.CreateCache(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	cacheAside := NewCacheAsideManager(cache, config.KeyPrefix, lg)

	return &CacheManager{
		cache:      cache,
		cacheAside: cacheAside,
		config:     config,
		logger:     lg,
	}, nil
}

// Cache returns the underlying cache instance
func (cm *CacheManager) Cache() Cache {
	return cm.cache
}

// CacheAside returns the cache-aside manager
func (cm *CacheManager) CacheAside() *CacheAsideManager {
	return cm.cacheAside
}

// IsEnabled returns whether caching is enabled
func (cm *CacheManager) IsEnabled() bool {
	return cm.config.Enabled
}

// GetConfig returns the cache configuration
func (cm *CacheManager) GetConfig() Config {
	return cm.config
}

// Health checks cache health
func (cm *CacheManager) Health() error {
	return cm.cache.Ping(context.Background())
}

// Close closes the cache manager and all associated resources
func (cm *CacheManager) Close() error {
	cm.logger.Info("Closing cache manager")

	if err := cm.cacheAside.Close(); err != nil {
		cm.logger.Error("Failed to close cache-aside manager", logger.Err(err))
		return err
	}

	return nil
}
