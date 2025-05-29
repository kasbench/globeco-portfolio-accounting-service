package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// CacheAsideService implements the cache-aside pattern
type CacheAsideService struct {
	cache      Cache
	keyService *CacheKeyService
	logger     logger.Logger
}

// NewCacheAsideService creates a new cache-aside service
func NewCacheAsideService(cache Cache, keyPrefix string, lg logger.Logger) *CacheAsideService {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	return &CacheAsideService{
		cache:      cache,
		keyService: NewCacheKeyService(keyPrefix),
		logger:     lg,
	}
}

// GetOrSet retrieves from cache or sets from provider function
func (cas *CacheAsideService) GetOrSet(ctx context.Context, key string, provider func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get from cache first
	data, err := cas.cache.Get(ctx, key)
	if err == nil {
		// Cache hit - deserialize and return
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			cas.logger.Warn("Failed to deserialize cached data",
				logger.String("key", key),
				logger.Err(err))
			// Continue to fetch from provider
		} else {
			cas.logger.Debug("Cache hit",
				logger.String("key", key))
			return result, nil
		}
	}

	// Cache miss or error - get from provider
	cas.logger.Debug("Cache miss",
		logger.String("key", key),
		logger.Err(err))

	value, err := provider()
	if err != nil {
		return nil, fmt.Errorf("provider failed: %w", err)
	}

	// Store in cache for next time
	if err := cas.Set(ctx, key, value, ttl); err != nil {
		cas.logger.Warn("Failed to cache value",
			logger.String("key", key),
			logger.Err(err))
		// Don't fail the request if caching fails
	}

	return value, nil
}

// Set stores a value in cache
func (cas *CacheAsideService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return cas.cache.Set(ctx, key, data, ttl)
}

// Delete removes a value from cache
func (cas *CacheAsideService) Delete(ctx context.Context, key string) error {
	return cas.cache.Delete(ctx, key)
}

// InvalidatePattern removes all keys matching a pattern
func (cas *CacheAsideService) InvalidatePattern(ctx context.Context, pattern string) error {
	return cas.cache.DeletePattern(ctx, pattern)
}

// TransactionCacheAside provides cache-aside operations for transactions
type TransactionCacheAside struct {
	*CacheAsideService
}

// NewTransactionCacheAside creates a new transaction cache-aside service
func NewTransactionCacheAside(cache Cache, keyPrefix string, lg logger.Logger) *TransactionCacheAside {
	return &TransactionCacheAside{
		CacheAsideService: NewCacheAsideService(cache, keyPrefix, lg),
	}
}

// GetByID retrieves a transaction by ID using cache-aside pattern
func (tca *TransactionCacheAside) GetByID(ctx context.Context, id int64, provider func() (interface{}, error)) (interface{}, error) {
	key := tca.keyService.Builder().TransactionByID(id)
	ttl := tca.keyService.TTLManager().GetTTL(key)
	return tca.GetOrSet(ctx, key, provider, ttl)
}

// GetBySourceID retrieves a transaction by source ID using cache-aside pattern
func (tca *TransactionCacheAside) GetBySourceID(ctx context.Context, sourceID string, provider func() (interface{}, error)) (interface{}, error) {
	key := tca.keyService.Builder().TransactionBySourceID(sourceID)
	ttl := tca.keyService.TTLManager().GetTTL(key)
	return tca.GetOrSet(ctx, key, provider, ttl)
}

// GetStats retrieves transaction stats using cache-aside pattern
func (tca *TransactionCacheAside) GetStats(ctx context.Context, provider func() (interface{}, error)) (interface{}, error) {
	key := tca.keyService.Builder().TransactionStats()
	ttl := tca.keyService.TTLManager().GetTTL(key)
	return tca.GetOrSet(ctx, key, provider, ttl)
}

// InvalidateTransaction removes transaction-related cache entries
func (tca *TransactionCacheAside) InvalidateTransaction(ctx context.Context, id int64, sourceID string) error {
	keys := []string{
		tca.keyService.Builder().TransactionByID(id),
		tca.keyService.Builder().TransactionBySourceID(sourceID),
		tca.keyService.Builder().TransactionStats(),
	}

	return tca.cache.DeleteMultiple(ctx, keys)
}

// InvalidateTransactionsByPortfolio removes portfolio-related transaction cache entries
func (tca *TransactionCacheAside) InvalidateTransactionsByPortfolio(ctx context.Context, portfolioID string) error {
	pattern := tca.keyService.Builder().PortfolioPattern(portfolioID)
	return tca.InvalidatePattern(ctx, pattern)
}

// BalanceCacheAside provides cache-aside operations for balances
type BalanceCacheAside struct {
	*CacheAsideService
}

// NewBalanceCacheAside creates a new balance cache-aside service
func NewBalanceCacheAside(cache Cache, keyPrefix string, lg logger.Logger) *BalanceCacheAside {
	return &BalanceCacheAside{
		CacheAsideService: NewCacheAsideService(cache, keyPrefix, lg),
	}
}

// GetByID retrieves a balance by ID using cache-aside pattern
func (bca *BalanceCacheAside) GetByID(ctx context.Context, id int64, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().BalanceByID(id)
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// GetByPortfolioAndSecurity retrieves a balance by portfolio and security using cache-aside pattern
func (bca *BalanceCacheAside) GetByPortfolioAndSecurity(ctx context.Context, portfolioID string, securityID *string, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().BalanceByPortfolioAndSecurity(portfolioID, securityID)
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// GetCashBalance retrieves cash balance using cache-aside pattern
func (bca *BalanceCacheAside) GetCashBalance(ctx context.Context, portfolioID string, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().CashBalance(portfolioID)
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// GetBalancesByPortfolio retrieves portfolio balances using cache-aside pattern
func (bca *BalanceCacheAside) GetBalancesByPortfolio(ctx context.Context, portfolioID string, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().BalancesByPortfolio(portfolioID)
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// GetStats retrieves balance stats using cache-aside pattern
func (bca *BalanceCacheAside) GetStats(ctx context.Context, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().BalanceStats()
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// GetPortfolioSummary retrieves portfolio summary using cache-aside pattern
func (bca *BalanceCacheAside) GetPortfolioSummary(ctx context.Context, portfolioID string, provider func() (interface{}, error)) (interface{}, error) {
	key := bca.keyService.Builder().PortfolioSummary(portfolioID)
	ttl := bca.keyService.TTLManager().GetTTL(key)
	return bca.GetOrSet(ctx, key, provider, ttl)
}

// InvalidateBalance removes balance-related cache entries
func (bca *BalanceCacheAside) InvalidateBalance(ctx context.Context, id int64, portfolioID string, securityID *string) error {
	keys := []string{
		bca.keyService.Builder().BalanceByID(id),
		bca.keyService.Builder().BalanceByPortfolioAndSecurity(portfolioID, securityID),
		bca.keyService.Builder().BalancesByPortfolio(portfolioID),
		bca.keyService.Builder().BalanceStats(),
		bca.keyService.Builder().PortfolioSummary(portfolioID),
	}

	if securityID == nil {
		// Also invalidate cash balance
		keys = append(keys, bca.keyService.Builder().CashBalance(portfolioID))
	}

	return bca.cache.DeleteMultiple(ctx, keys)
}

// InvalidatePortfolioBalances removes all balance cache entries for a portfolio
func (bca *BalanceCacheAside) InvalidatePortfolioBalances(ctx context.Context, portfolioID string) error {
	pattern := bca.keyService.Builder().PortfolioPattern(portfolioID)
	return bca.InvalidatePattern(ctx, pattern)
}

// ExternalServiceCacheAside provides cache-aside operations for external service data
type ExternalServiceCacheAside struct {
	*CacheAsideService
}

// NewExternalServiceCacheAside creates a new external service cache-aside service
func NewExternalServiceCacheAside(cache Cache, keyPrefix string, lg logger.Logger) *ExternalServiceCacheAside {
	return &ExternalServiceCacheAside{
		CacheAsideService: NewCacheAsideService(cache, keyPrefix, lg),
	}
}

// GetPortfolio retrieves portfolio data using cache-aside pattern
func (esca *ExternalServiceCacheAside) GetPortfolio(ctx context.Context, portfolioID string, provider func() (interface{}, error)) (interface{}, error) {
	key := esca.keyService.Builder().Portfolio(portfolioID)
	ttl := esca.keyService.TTLManager().GetTTL(key)
	return esca.GetOrSet(ctx, key, provider, ttl)
}

// GetSecurity retrieves security data using cache-aside pattern
func (esca *ExternalServiceCacheAside) GetSecurity(ctx context.Context, securityID string, provider func() (interface{}, error)) (interface{}, error) {
	key := esca.keyService.Builder().Security(securityID)
	ttl := esca.keyService.TTLManager().GetTTL(key)
	return esca.GetOrSet(ctx, key, provider, ttl)
}

// InvalidatePortfolio removes portfolio cache entry
func (esca *ExternalServiceCacheAside) InvalidatePortfolio(ctx context.Context, portfolioID string) error {
	key := esca.keyService.Builder().Portfolio(portfolioID)
	return esca.Delete(ctx, key)
}

// InvalidateSecurity removes security cache entry
func (esca *ExternalServiceCacheAside) InvalidateSecurity(ctx context.Context, securityID string) error {
	key := esca.keyService.Builder().Security(securityID)
	return esca.Delete(ctx, key)
}

// CacheAsideManager manages all cache-aside services
type CacheAsideManager struct {
	Transaction     *TransactionCacheAside
	Balance         *BalanceCacheAside
	ExternalService *ExternalServiceCacheAside
	cache           Cache
	logger          logger.Logger
}

// NewCacheAsideManager creates a new cache-aside manager
func NewCacheAsideManager(cache Cache, keyPrefix string, lg logger.Logger) *CacheAsideManager {
	if lg == nil {
		lg = logger.NewDevelopment()
	}

	return &CacheAsideManager{
		Transaction:     NewTransactionCacheAside(cache, keyPrefix, lg),
		Balance:         NewBalanceCacheAside(cache, keyPrefix, lg),
		ExternalService: NewExternalServiceCacheAside(cache, keyPrefix, lg),
		cache:           cache,
		logger:          lg,
	}
}

// ClearAll clears all cached data
func (cam *CacheAsideManager) ClearAll(ctx context.Context) error {
	if err := cam.cache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	cam.logger.Info("All cache data cleared")
	return nil
}

// Stats returns cache statistics
func (cam *CacheAsideManager) Stats(ctx context.Context) (*CacheStats, error) {
	return cam.cache.Stats(ctx)
}

// Health checks cache health
func (cam *CacheAsideManager) Health(ctx context.Context) error {
	return cam.cache.Ping(ctx)
}

// Close closes all cache connections
func (cam *CacheAsideManager) Close() error {
	return cam.cache.Close()
}
