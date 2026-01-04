package route

import (
	"time"

	"github.com/dgraph-io/ristretto"
	"go.uber.org/zap"
)

// RouteCacheEntry represents a cached route entry
type RouteCacheEntry struct {
	GatewayID string
	Address   string
}

// RouteCache manages local route cache with TTL using ristretto
type RouteCache struct {
	cache *ristretto.Cache
}

// NewRouteCache creates a new route cache with ristretto
func NewRouteCache() *RouteCache {
	// Configure ristretto cache
	// NumCounters: 10x of MaxCost for optimal hit ratio
	// MaxCost: maximum number of cached entries
	// BufferItems: number of keys per Get buffer
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 100000, // 10x expected max entries
		MaxCost:     10000,  // max 10k route entries
		BufferItems: 64,     // keys per Get buffer
	})
	if err != nil {
		zap.L().Fatal("Failed to create ristretto cache", zap.Error(err))
	}

	return &RouteCache{
		cache: cache,
	}
}

// Get retrieves a route from cache if not expired
func (rc *RouteCache) Get(userID int64) (*RouteCacheEntry, bool) {
	value, found := rc.cache.Get(userID)
	if !found {
		return nil, false
	}

	entry, ok := value.(*RouteCacheEntry)
	if !ok {
		return nil, false
	}

	return entry, true
}

// Set adds or updates a route in cache
// Cost is set to 1 for each entry (can be adjusted based on actual size)
func (rc *RouteCache) Set(userID int64, gatewayID, address string, ttl time.Duration) {
	entry := &RouteCacheEntry{
		GatewayID: gatewayID,
		Address:   address,
	}

	// SetWithTTL returns false if the entry is rejected due to cost or other reasons
	// We set cost to 1 for each route entry
	rc.cache.SetWithTTL(userID, entry, 1, ttl)
}

// Delete removes a route from cache
func (rc *RouteCache) Delete(userID int64) {
	rc.cache.Del(userID)
}

// Clear removes all entries from cache
func (rc *RouteCache) Clear() {
	rc.cache.Clear()
}

// Size returns approximate number of cached entries
func (rc *RouteCache) Size() int {
	// Ristretto doesn't provide exact size, use metrics instead
	metrics := rc.cache.Metrics
	return int(metrics.KeysAdded() - metrics.KeysEvicted())
}

// GetMetrics returns cache metrics for monitoring
func (rc *RouteCache) GetMetrics() *ristretto.Metrics {
	return rc.cache.Metrics
}

// Close closes the cache and releases resources
func (rc *RouteCache) Close() {
	rc.cache.Close()
}

// StartCleanupRoutine is no longer needed as ristretto handles cleanup automatically
// Kept for API compatibility but does nothing
func (rc *RouteCache) StartCleanupRoutine(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})
	// Ristretto handles TTL and eviction automatically
	// No need for manual cleanup routine
	zap.L().Info("Ristretto cache auto-manages TTL and eviction, cleanup routine not needed")
	return stopCh
}
