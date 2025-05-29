package cache

import (
	"fmt"
	"strings"
	"time"
)

// KeyBuilder provides methods for building cache keys with consistent naming
type KeyBuilder struct {
	prefix string
}

// NewKeyBuilder creates a new key builder with optional prefix
func NewKeyBuilder(prefix string) *KeyBuilder {
	return &KeyBuilder{
		prefix: prefix,
	}
}

// Transaction cache keys
func (kb *KeyBuilder) TransactionByID(id int64) string {
	return kb.buildKey("transaction", "id", fmt.Sprint(id))
}

func (kb *KeyBuilder) TransactionBySourceID(sourceID string) string {
	return kb.buildKey("transaction", "source", sourceID)
}

func (kb *KeyBuilder) TransactionsByPortfolio(portfolioID string, limit, offset int) string {
	return kb.buildKey("transactions", "portfolio", portfolioID, fmt.Sprintf("l%d_o%d", limit, offset))
}

func (kb *KeyBuilder) TransactionsByStatus(status string, limit, offset int) string {
	return kb.buildKey("transactions", "status", status, fmt.Sprintf("l%d_o%d", limit, offset))
}

func (kb *KeyBuilder) TransactionStats() string {
	return kb.buildKey("transaction", "stats")
}

func (kb *KeyBuilder) TransactionCount(filterHash string) string {
	return kb.buildKey("transaction", "count", filterHash)
}

func (kb *KeyBuilder) TransactionList(filterHash string) string {
	return kb.buildKey("transaction", "list", filterHash)
}

// Balance cache keys
func (kb *KeyBuilder) BalanceByID(id int64) string {
	return kb.buildKey("balance", "id", fmt.Sprint(id))
}

func (kb *KeyBuilder) BalanceByPortfolioAndSecurity(portfolioID string, securityID *string) string {
	security := "cash"
	if securityID != nil {
		security = *securityID
	}
	return kb.buildKey("balance", "portfolio", portfolioID, "security", security)
}

func (kb *KeyBuilder) BalancesByPortfolio(portfolioID string) string {
	return kb.buildKey("balances", "portfolio", portfolioID)
}

func (kb *KeyBuilder) CashBalance(portfolioID string) string {
	return kb.buildKey("balance", "cash", portfolioID)
}

func (kb *KeyBuilder) BalanceStats() string {
	return kb.buildKey("balance", "stats")
}

func (kb *KeyBuilder) PortfolioSummary(portfolioID string) string {
	return kb.buildKey("portfolio", "summary", portfolioID)
}

func (kb *KeyBuilder) BalanceCount(filterHash string) string {
	return kb.buildKey("balance", "count", filterHash)
}

func (kb *KeyBuilder) BalanceList(filterHash string) string {
	return kb.buildKey("balance", "list", filterHash)
}

// External service cache keys
func (kb *KeyBuilder) Portfolio(portfolioID string) string {
	return kb.buildKey("external", "portfolio", portfolioID)
}

func (kb *KeyBuilder) Security(securityID string) string {
	return kb.buildKey("external", "security", securityID)
}

// Session and processing cache keys
func (kb *KeyBuilder) ProcessingLock(portfolioID string) string {
	return kb.buildKey("lock", "processing", portfolioID)
}

func (kb *KeyBuilder) UserSession(sessionID string) string {
	return kb.buildKey("session", sessionID)
}

// Pattern keys for bulk operations
func (kb *KeyBuilder) TransactionPattern() string {
	return kb.buildKey("transaction", "*")
}

func (kb *KeyBuilder) BalancePattern() string {
	return kb.buildKey("balance", "*")
}

func (kb *KeyBuilder) PortfolioPattern(portfolioID string) string {
	return kb.buildKey("*", portfolioID, "*")
}

func (kb *KeyBuilder) SecurityPattern(securityID string) string {
	return kb.buildKey("*", "*", securityID, "*")
}

// buildKey constructs a hierarchical cache key
func (kb *KeyBuilder) buildKey(parts ...string) string {
	var keyParts []string

	if kb.prefix != "" {
		keyParts = append(keyParts, kb.prefix)
	}

	// Filter out empty parts
	for _, part := range parts {
		if part != "" {
			keyParts = append(keyParts, part)
		}
	}

	return strings.Join(keyParts, ":")
}

// KeyParser provides methods for parsing cache keys
type KeyParser struct{}

// NewKeyParser creates a new key parser
func NewKeyParser() *KeyParser {
	return &KeyParser{}
}

// ParseTransactionKey extracts information from a transaction cache key
func (kp *KeyParser) ParseTransactionKey(key string) (*KeyInfo, error) {
	parts := strings.Split(key, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid cache key format: %s", key)
	}

	info := &KeyInfo{
		FullKey: key,
		Type:    parts[0],
		Parts:   parts[1:],
	}

	return info, nil
}

// KeyInfo holds parsed key information
type KeyInfo struct {
	FullKey string   `json:"full_key"`
	Type    string   `json:"type"`
	Parts   []string `json:"parts"`
}

// TTLManager provides TTL configuration for different key types
type TTLManager struct {
	defaultTTL  time.Duration
	keyTTLs     map[string]time.Duration
	patternTTLs map[string]time.Duration
}

// NewTTLManager creates a new TTL manager with default values
func NewTTLManager() *TTLManager {
	return &TTLManager{
		defaultTTL: 15 * time.Minute,
		keyTTLs: map[string]time.Duration{
			"transaction:id":     1 * time.Hour,    // Individual transactions cache longer
			"transaction:source": 1 * time.Hour,    // Source ID lookups cache longer
			"transaction:stats":  5 * time.Minute,  // Stats cache shorter
			"transaction:count":  10 * time.Minute, // Count queries
			"transaction:list":   5 * time.Minute,  // List queries cache shorter
			"balance:id":         30 * time.Minute, // Individual balances
			"balance:portfolio":  15 * time.Minute, // Portfolio balances
			"balance:cash":       30 * time.Minute, // Cash balances
			"balance:stats":      5 * time.Minute,  // Balance stats
			"portfolio:summary":  10 * time.Minute, // Portfolio summaries
			"external:portfolio": 2 * time.Hour,    // External service data cache longer
			"external:security":  2 * time.Hour,    // External service data cache longer
			"lock:processing":    30 * time.Second, // Processing locks are short-lived
			"session":            24 * time.Hour,   // User sessions
		},
		patternTTLs: map[string]time.Duration{
			"transaction:*": 30 * time.Minute,
			"balance:*":     20 * time.Minute,
			"external:*":    2 * time.Hour,
			"lock:*":        1 * time.Minute,
		},
	}
}

// GetTTL returns the TTL for a given cache key
func (tm *TTLManager) GetTTL(key string) time.Duration {
	// Check for exact key match first
	if ttl, exists := tm.keyTTLs[key]; exists {
		return ttl
	}

	// Check for pattern matches
	for pattern, ttl := range tm.patternTTLs {
		if tm.matchesPattern(key, pattern) {
			return ttl
		}
	}

	// Return default TTL
	return tm.defaultTTL
}

// SetTTL sets custom TTL for a specific key pattern
func (tm *TTLManager) SetTTL(keyPattern string, ttl time.Duration) {
	if strings.Contains(keyPattern, "*") {
		tm.patternTTLs[keyPattern] = ttl
	} else {
		tm.keyTTLs[keyPattern] = ttl
	}
}

// matchesPattern checks if a key matches a wildcard pattern
func (tm *TTLManager) matchesPattern(key, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return key == pattern
	}

	keyParts := strings.Split(key, ":")
	patternParts := strings.Split(pattern, ":")

	if len(keyParts) < len(patternParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if patternPart != "*" && (i >= len(keyParts) || keyParts[i] != patternPart) {
			return false
		}
	}

	return true
}

// CacheKeyService provides high-level cache key management
type CacheKeyService struct {
	builder    *KeyBuilder
	parser     *KeyParser
	ttlManager *TTLManager
}

// NewCacheKeyService creates a new cache key service
func NewCacheKeyService(prefix string) *CacheKeyService {
	return &CacheKeyService{
		builder:    NewKeyBuilder(prefix),
		parser:     NewKeyParser(),
		ttlManager: NewTTLManager(),
	}
}

// Builder returns the key builder
func (cks *CacheKeyService) Builder() *KeyBuilder {
	return cks.builder
}

// Parser returns the key parser
func (cks *CacheKeyService) Parser() *KeyParser {
	return cks.parser
}

// TTLManager returns the TTL manager
func (cks *CacheKeyService) TTLManager() *TTLManager {
	return cks.ttlManager
}

// GenerateFilterHash creates a consistent hash for filter objects
func (cks *CacheKeyService) GenerateFilterHash(filter interface{}) string {
	// This is a simplified hash generation
	// In production, you might want to use a more sophisticated approach
	return fmt.Sprintf("%x", filter)
}
