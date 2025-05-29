package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-go-client"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// HazelcastCache implements the Cache interface using Hazelcast
type HazelcastCache struct {
	client     *hazelcast.Client
	mapName    string
	logger     logger.Logger
	config     HazelcastConfig
	keyService *CacheKeyService
}

// HazelcastConfig holds configuration for Hazelcast client
type HazelcastConfig struct {
	ClusterName       string        `mapstructure:"cluster_name" json:"cluster_name"`
	ClusterMembers    []string      `mapstructure:"cluster_members" json:"cluster_members"`
	ConnectionRetry   int           `mapstructure:"connection_retry" json:"connection_retry"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout" json:"connection_timeout"`
	MapName           string        `mapstructure:"map_name" json:"map_name"`
	KeyPrefix         string        `mapstructure:"key_prefix" json:"key_prefix"`
	Serialization     string        `mapstructure:"serialization" json:"serialization"` // "json" or "gob"
	EnableMetrics     bool          `mapstructure:"enable_metrics" json:"enable_metrics"`
	EnableLogging     bool          `mapstructure:"enable_logging" json:"enable_logging"`
}

// NewHazelcastCache creates a new Hazelcast cache implementation
func NewHazelcastCache(cfg HazelcastConfig, lg logger.Logger) (*HazelcastCache, error) {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	// Create Hazelcast configuration
	hazelcastConfig := hazelcast.Config{}
	hazelcastConfig.Cluster.Name = cfg.ClusterName

	// Set cluster members
	if len(cfg.ClusterMembers) > 0 {
		hazelcastConfig.Cluster.Network.SetAddresses(cfg.ClusterMembers...)
	} else {
		// Default to localhost
		hazelcastConfig.Cluster.Network.SetAddresses("127.0.0.1:5701")
	}

	// Set connection timeout
	if cfg.ConnectionTimeout > 0 {
		// Note: Hazelcast Go client v1.4.2 may not support all these options
		// This is a simplified configuration
	}

	// Create client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := hazelcast.StartNewClientWithConfig(ctx, hazelcastConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Hazelcast client: %w", err)
	}

	// Set default map name if not provided
	mapName := cfg.MapName
	if mapName == "" {
		mapName = "portfolio-accounting-cache"
	}

	// Create key service
	keyService := NewCacheKeyService(cfg.KeyPrefix)

	cache := &HazelcastCache{
		client:     client,
		mapName:    mapName,
		logger:     lg,
		config:     cfg,
		keyService: keyService,
	}

	lg.Info("Hazelcast cache initialized",
		logger.String("clusterName", cfg.ClusterName),
		logger.String("mapName", mapName))

	return cache, nil
}

// Get retrieves a value from the cache
func (hc *HazelcastCache) Get(ctx context.Context, key string) ([]byte, error) {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return nil, NewCacheError("get_map", key, err)
	}

	value, err := m.Get(ctx, key)
	if err != nil {
		return nil, NewCacheError("get", key, err)
	}

	if value == nil {
		return nil, NewCacheError("get", key, ErrKeyNotFound)
	}

	// Convert value to bytes based on serialization type
	data, err := hc.valueToBytes(value)
	if err != nil {
		return nil, NewCacheError("deserialize", key, err)
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache hit",
			logger.String("key", key),
			logger.Int("size", len(data)))
	}

	return data, nil
}

// Set stores a value in the cache with TTL
func (hc *HazelcastCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("get_map", key, err)
	}

	// Convert bytes to value based on serialization type
	cacheValue, err := hc.bytesToValue(value)
	if err != nil {
		return NewCacheError("serialize", key, err)
	}

	// Set with TTL if specified
	if ttl > 0 {
		err = m.SetWithTTL(ctx, key, cacheValue, ttl)
	} else {
		err = m.Set(ctx, key, cacheValue)
	}

	if err != nil {
		return NewCacheError("set", key, err)
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache set",
			logger.String("key", key),
			logger.Int("size", len(value)),
			logger.Duration("ttl", ttl))
	}

	return nil
}

// Delete removes a key from the cache
func (hc *HazelcastCache) Delete(ctx context.Context, key string) error {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("get_map", key, err)
	}

	_, err = m.Remove(ctx, key)
	if err != nil {
		return NewCacheError("delete", key, err)
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache delete",
			logger.String("key", key))
	}

	return nil
}

// Exists checks if a key exists in the cache
func (hc *HazelcastCache) Exists(ctx context.Context, key string) (bool, error) {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return false, NewCacheError("get_map", key, err)
	}

	exists, err := m.ContainsKey(ctx, key)
	if err != nil {
		return false, NewCacheError("exists", key, err)
	}

	return exists, nil
}

// GetMultiple retrieves multiple values from the cache
func (hc *HazelcastCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return nil, NewCacheError("get_map", strings.Join(keys, ","), err)
	}

	// Convert string keys to interface{}
	keySet := make([]interface{}, len(keys))
	for i, key := range keys {
		keySet[i] = key
	}

	values, err := m.GetAll(ctx, keySet...)
	if err != nil {
		return nil, NewCacheError("get_multiple", strings.Join(keys, ","), err)
	}

	result := make(map[string][]byte)
	for _, entry := range values {
		keyStr, ok := entry.Key.(string)
		if !ok {
			continue
		}

		if entry.Value != nil {
			data, err := hc.valueToBytes(entry.Value)
			if err != nil {
				hc.logger.Warn("Failed to deserialize cache value",
					logger.String("key", keyStr),
					logger.Err(err))
				continue
			}
			result[keyStr] = data
		}
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache get multiple",
			logger.Int("requested", len(keys)),
			logger.Int("found", len(result)))
	}

	return result, nil
}

// SetMultiple stores multiple values in the cache
func (hc *HazelcastCache) SetMultiple(ctx context.Context, items map[string]CacheItem) error {
	if len(items) == 0 {
		return nil
	}

	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("get_map", "multiple", err)
	}

	// Set items one by one (Hazelcast Go client doesn't have direct bulk set with TTL)
	for key, item := range items {
		cacheValue, err := hc.bytesToValue(item.Value)
		if err != nil {
			return NewCacheError("serialize", key, err)
		}

		if item.TTL > 0 {
			err = m.SetWithTTL(ctx, key, cacheValue, item.TTL)
		} else {
			err = m.Set(ctx, key, cacheValue)
		}

		if err != nil {
			return NewCacheError("set_multiple", key, err)
		}
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache set multiple",
			logger.Int("count", len(items)))
	}

	return nil
}

// DeleteMultiple removes multiple keys from the cache
func (hc *HazelcastCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("get_map", strings.Join(keys, ","), err)
	}

	// Delete keys one by one
	for _, key := range keys {
		_, err := m.Remove(ctx, key)
		if err != nil {
			return NewCacheError("delete_multiple", key, err)
		}
	}

	if hc.config.EnableLogging {
		hc.logger.Debug("Cache delete multiple",
			logger.Int("count", len(keys)))
	}

	return nil
}

// DeletePattern removes all keys matching a pattern
func (hc *HazelcastCache) DeletePattern(ctx context.Context, pattern string) error {
	// Get all keys first, then filter and delete
	keys, err := hc.GetKeys(ctx, pattern)
	if err != nil {
		return NewCacheError("delete_pattern", pattern, err)
	}

	if len(keys) == 0 {
		return nil
	}

	return hc.DeleteMultiple(ctx, keys)
}

// GetKeys retrieves all keys matching a pattern
func (hc *HazelcastCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return nil, NewCacheError("get_map", pattern, err)
	}

	keySet, err := m.GetKeySet(ctx)
	if err != nil {
		return nil, NewCacheError("get_keys", pattern, err)
	}

	var matchingKeys []string
	for _, key := range keySet {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		if hc.matchesPattern(keyStr, pattern) {
			matchingKeys = append(matchingKeys, keyStr)
		}
	}

	return matchingKeys, nil
}

// SetTTL sets TTL for an existing key
func (hc *HazelcastCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	// Hazelcast doesn't have a direct "set TTL" operation
	// We need to get the value and set it again with TTL
	value, err := hc.Get(ctx, key)
	if err != nil {
		return err
	}

	return hc.Set(ctx, key, value, ttl)
}

// GetTTL gets the remaining TTL for a key
func (hc *HazelcastCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	// Hazelcast Go client doesn't expose TTL information directly
	// This is a limitation of the current client
	return 0, NewCacheError("get_ttl", key, fmt.Errorf("TTL retrieval not supported by Hazelcast Go client"))
}

// Clear removes all entries from the cache
func (hc *HazelcastCache) Clear(ctx context.Context) error {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("get_map", "clear", err)
	}

	err = m.Clear(ctx)
	if err != nil {
		return NewCacheError("clear", "", err)
	}

	hc.logger.Info("Cache cleared")
	return nil
}

// Size returns the number of entries in the cache
func (hc *HazelcastCache) Size(ctx context.Context) (int64, error) {
	m, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return 0, NewCacheError("get_map", "size", err)
	}

	size, err := m.Size(ctx)
	if err != nil {
		return 0, NewCacheError("size", "", err)
	}

	return int64(size), nil
}

// Stats returns cache statistics
func (hc *HazelcastCache) Stats(ctx context.Context) (*CacheStats, error) {
	size, err := hc.Size(ctx)
	if err != nil {
		return nil, err
	}

	// Hazelcast Go client has limited stats support
	// This is a basic implementation
	stats := &CacheStats{
		Size:           size,
		LastAccessTime: time.Now(),
		LastUpdateTime: time.Now(),
	}

	return stats, nil
}

// Ping checks if the cache is available
func (hc *HazelcastCache) Ping(ctx context.Context) error {
	if hc.client == nil {
		return NewCacheError("ping", "", ErrCacheNotReady)
	}

	// Try to get the map as a health check
	_, err := hc.client.GetMap(ctx, hc.mapName)
	if err != nil {
		return NewCacheError("ping", "", err)
	}

	return nil
}

// Close closes the cache connection
func (hc *HazelcastCache) Close() error {
	if hc.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := hc.client.Shutdown(ctx)
		hc.logger.Info("Hazelcast cache connection closed")
		return err
	}
	return nil
}

// Helper methods

// valueToBytes converts cache value to bytes based on serialization type
func (hc *HazelcastCache) valueToBytes(value interface{}) ([]byte, error) {
	switch hc.config.Serialization {
	case "json":
		return json.Marshal(value)
	default:
		// Default to assuming it's already bytes or string
		switch v := value.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		default:
			return json.Marshal(v)
		}
	}
}

// bytesToValue converts bytes to cache value based on serialization type
func (hc *HazelcastCache) bytesToValue(data []byte) (interface{}, error) {
	switch hc.config.Serialization {
	case "json":
		var value interface{}
		err := json.Unmarshal(data, &value)
		return value, err
	default:
		// Default to string
		return string(data), nil
	}
}

// matchesPattern checks if a key matches a wildcard pattern
func (hc *HazelcastCache) matchesPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return key == pattern
	}

	// Simple wildcard matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	// Check if key starts with first part (if not empty)
	if parts[0] != "" && !strings.HasPrefix(key, parts[0]) {
		return false
	}

	// Check if key ends with last part (if not empty)
	if parts[len(parts)-1] != "" && !strings.HasSuffix(key, parts[len(parts)-1]) {
		return false
	}

	// For more complex patterns, we'd need more sophisticated matching
	return true
}
