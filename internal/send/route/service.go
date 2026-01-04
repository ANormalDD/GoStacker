package route

import (
	"GoStacker/pkg/registry_client"
	"errors"
	"time"

	"go.uber.org/zap"
)

var (
	ErrUserOffline      = errors.New("user is offline")
	ErrRegistryNotReady = errors.New("registry client not initialized")
)

// LocalRouteCacheTTL is the TTL for local route cache (shorter than Registry's TTL)
const LocalRouteCacheTTL = 60 * time.Second

// Global route cache instance
var localCache *RouteCache

// Global registry client (will be set during initialization)
var registryClient *registry_client.SendClient

// InitRouteService initializes the route service with registry client
func InitRouteService(client *registry_client.SendClient) {
	localCache = NewRouteCache()
	registryClient = client

	// Start cleanup routine (every 30 seconds)
	localCache.StartCleanupRoutine(30 * time.Second)

	zap.L().Info("Route service initialized with local cache")
}

// GetUserGateway retrieves the gateway information for a user
// It first checks local cache, then queries Registry if cache miss
func GetUserGateway(userID int64) (gatewayID, address string, err error) {
	// 1. Check local cache first
	if localCache != nil {
		if entry, ok := localCache.Get(userID); ok {
			zap.L().Debug("Route cache hit",
				zap.Int64("user_id", userID),
				zap.String("gateway_id", entry.GatewayID))
			return entry.GatewayID, entry.Address, nil
		}
	}

	// 2. Cache miss, query from Registry
	if registryClient == nil {
		zap.L().Error("Registry client not initialized")
		return "", "", ErrRegistryNotReady
	}

	routes, err := registryClient.QueryUserRoutes([]int64{userID})
	if err != nil {
		zap.L().Error("Failed to query user route from registry",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return "", "", err
	}

	routeInfo, exists := routes[userID]
	if !exists {
		zap.L().Debug("User route not found in registry (offline)",
			zap.Int64("user_id", userID))
		return "", "", ErrUserOffline
	}

	// 3. Update local cache
	if localCache != nil {
		localCache.Set(userID, routeInfo.GatewayID, routeInfo.Address, LocalRouteCacheTTL)
		zap.L().Debug("Route cached locally",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", routeInfo.GatewayID),
			zap.Duration("ttl", LocalRouteCacheTTL))
	}

	return routeInfo.GatewayID, routeInfo.Address, nil
}

// BatchGetUserGateways retrieves gateway information for multiple users
// Uses batch query to Registry for better performance
func BatchGetUserGateways(userIDs []int64) (map[int64]*RouteInfo, error) {
	if registryClient == nil {
		return nil, ErrRegistryNotReady
	}

	result := make(map[int64]*RouteInfo)
	missingUserIDs := []int64{}

	// 1. Check local cache for all users
	if localCache != nil {
		for _, userID := range userIDs {
			if entry, ok := localCache.Get(userID); ok {
				result[userID] = &RouteInfo{
					GatewayID: entry.GatewayID,
					Address:   entry.Address,
				}
			} else {
				missingUserIDs = append(missingUserIDs, userID)
			}
		}
	} else {
		missingUserIDs = userIDs
	}

	// 2. Query missing routes from Registry
	if len(missingUserIDs) > 0 {
		routes, err := registryClient.QueryUserRoutes(missingUserIDs)
		if err != nil {
			zap.L().Error("Failed to batch query user routes from registry",
				zap.Int("missing_count", len(missingUserIDs)),
				zap.Error(err))
			return result, err
		}

		// 3. Merge results and update cache
		for userID, routeInfo := range routes {
			result[userID] = &RouteInfo{
				GatewayID: routeInfo.GatewayID,
				Address:   routeInfo.Address,
			}

			// Update local cache
			if localCache != nil {
				localCache.Set(userID, routeInfo.GatewayID, routeInfo.Address, LocalRouteCacheTTL)
			}
		}

		zap.L().Debug("Batch route query completed",
			zap.Int("requested", len(userIDs)),
			zap.Int("cache_hits", len(userIDs)-len(missingUserIDs)),
			zap.Int("registry_queries", len(missingUserIDs)),
			zap.Int("found", len(result)))
	}

	return result, nil
}

// RouteInfo represents route information
type RouteInfo struct {
	GatewayID string
	Address   string
}

// InvalidateCache removes a user route from local cache
func InvalidateCache(userID int64) {
	if localCache != nil {
		localCache.Delete(userID)
		zap.L().Debug("Route cache invalidated", zap.Int64("user_id", userID))
	}
}

// ClearCache clears all cached routes
func ClearCache() {
	if localCache != nil {
		localCache.Clear()
		zap.L().Info("Route cache cleared")
	}
}

// GetCacheStats returns cache statistics
func GetCacheStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if localCache != nil {
		metrics := localCache.GetMetrics()
		stats["size"] = localCache.Size()
		stats["ttl_seconds"] = int(LocalRouteCacheTTL.Seconds())
		stats["hits"] = metrics.Hits()
		stats["misses"] = metrics.Misses()
		stats["hit_ratio"] = metrics.Ratio()
		stats["keys_added"] = metrics.KeysAdded()
		stats["keys_updated"] = metrics.KeysUpdated()
		stats["keys_evicted"] = metrics.KeysEvicted()
		stats["cost_added"] = metrics.CostAdded()
		stats["cost_evicted"] = metrics.CostEvicted()
	} else {
		stats["size"] = 0
		stats["initialized"] = false
	}

	return stats
}
