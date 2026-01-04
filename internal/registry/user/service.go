package user

import (
	"GoStacker/internal/registry/gateway"
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	// Redis key prefix for user routes
	userRoutePrefix = "route:user:"
)

var (
	ErrUserRouteNotFound = errors.New("user route not found")
	ErrInvalidRouteData  = errors.New("invalid route data")
)

// RecordUserConnect records user connection to a gateway
func RecordUserConnect(userID int64, gatewayID string) error {
	// Get gateway info to include address
	gwInfo, err := gateway.GetGatewayInfo(gatewayID)
	if err != nil {
		zap.L().Error("Failed to get gateway info for user connect",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", gatewayID),
			zap.Error(err))
		return err
	}

	now := time.Now()
	route := UserRoute{
		UserID:      userID,
		GatewayID:   gatewayID,
		Address:     gateway.GetGatewayAddress(gwInfo),
		ConnectedAt: now,
		Status:      "connected",
	}

	// Serialize to JSON
	data, err := json.Marshal(route)
	if err != nil {
		zap.L().Error("Failed to marshal user route", zap.Error(err))
		return err
	}

	// Get TTL from config
	ttl := 120 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.UserRouteTTL) * time.Second
	}

	// Store in Redis with TTL
	key := userRoutePrefix + strconv.FormatInt(userID, 10)
	err = redis.SetEXWithRetry(2, key, string(data), ttl)
	if err != nil {
		zap.L().Error("Failed to set user route in Redis",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", gatewayID),
			zap.Error(err))
		return err
	}

	zap.L().Info("User route recorded",
		zap.Int64("user_id", userID),
		zap.String("gateway_id", gatewayID),
		zap.String("address", route.Address))

	return nil
}

// RecordUserDisconnect updates user route on disconnection
// Does not delete the route, just sets TTL to allow quick reconnection
func RecordUserDisconnect(userID int64, gatewayID string) error {
	key := userRoutePrefix + strconv.FormatInt(userID, 10)

	// Get existing route
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		zap.L().Warn("User route not found for disconnect",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", gatewayID))
		// Not an error, user might have already expired
		return nil
	}

	var route UserRoute
	err = json.Unmarshal([]byte(data), &route)
	if err != nil {
		zap.L().Error("Failed to unmarshal user route", zap.Error(err))
		return ErrInvalidRouteData
	}

	// Verify gateway ID matches
	if route.GatewayID != gatewayID {
		zap.L().Warn("Gateway ID mismatch on disconnect",
			zap.Int64("user_id", userID),
			zap.String("expected", gatewayID),
			zap.String("actual", route.GatewayID))
		// Don't update if gateway doesn't match
		return nil
	}

	// Update status to disconnected
	route.Status = "disconnected"

	// Serialize back
	newData, err := json.Marshal(route)
	if err != nil {
		zap.L().Error("Failed to marshal user route", zap.Error(err))
		return err
	}

	// Get TTL from config (keep same TTL for quick reconnection)
	ttl := 120 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.UserRouteTTL) * time.Second
	}

	// Update in Redis with TTL
	err = redis.SetEXWithRetry(2, key, string(newData), ttl)
	if err != nil {
		zap.L().Error("Failed to update user route on disconnect",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return err
	}

	zap.L().Info("User route updated on disconnect (retained with TTL)",
		zap.Int64("user_id", userID),
		zap.String("gateway_id", gatewayID),
		zap.Duration("ttl", ttl))

	return nil
}

// GetUserRoute retrieves user route information
func GetUserRoute(userID int64) (*UserRoute, error) {
	key := userRoutePrefix + strconv.FormatInt(userID, 10)
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		return nil, ErrUserRouteNotFound
	}

	var route UserRoute
	err = json.Unmarshal([]byte(data), &route)
	if err != nil {
		zap.L().Error("Failed to unmarshal user route", zap.Error(err))
		return nil, ErrInvalidRouteData
	}

	return &route, nil
}

// BatchGetUserRoutes retrieves multiple user routes at once
func BatchGetUserRoutes(userIDs []int64) (map[int64]*RouteInfo, error) {
	result := make(map[int64]*RouteInfo)

	// Build keys
	keys := make([]string, len(userIDs))
	for i, userID := range userIDs {
		keys[i] = userRoutePrefix + strconv.FormatInt(userID, 10)
	}

	// Batch get from Redis
	values, err := redis.MGetWithRetry(2, keys)
	if err != nil {
		zap.L().Error("Failed to batch get user routes", zap.Error(err))
		return nil, err
	}

	// Parse results
	for i, val := range values {
		if val == "" {
			// User offline or route not found
			continue
		}

		var route UserRoute
		err := json.Unmarshal([]byte(val), &route)
		if err != nil {
			zap.L().Warn("Failed to unmarshal user route in batch",
				zap.Int64("user_id", userIDs[i]),
				zap.Error(err))
			continue
		}

		result[userIDs[i]] = &RouteInfo{
			GatewayID: route.GatewayID,
			Address:   route.Address,
		}
	}

	return result, nil
}

// GetAvailableGatewayForUser returns an available gateway for user connection
// Implements the logic: check existing route first, then select lowest load gateway
func GetAvailableGatewayForUser(userID int64) (*GatewayAvailableResponse, error) {
	// 1. Check if user has existing route (for reconnection optimization)
	existingRoute, err := GetUserRoute(userID)
	if err == nil && existingRoute != nil {
		// Verify gateway is still healthy
		if gateway.IsGatewayHealthy(existingRoute.GatewayID) {
			gwInfo, err := gateway.GetGatewayInfo(existingRoute.GatewayID)
			if err == nil {
				zap.L().Info("User reconnecting to previous gateway",
					zap.Int64("user_id", userID),
					zap.String("gateway_id", existingRoute.GatewayID))
				return &GatewayAvailableResponse{
					GatewayID: gwInfo.GatewayID,
					Address:   gwInfo.Address,
					Port:      gwInfo.Port,
				}, nil
			}
		}
		// Gateway not healthy anymore, proceed to select new one
		zap.L().Info("Previous gateway not available, selecting new one",
			zap.Int64("user_id", userID),
			zap.String("old_gateway_id", existingRoute.GatewayID))
	}

	// 2. Select gateway with lowest load
	gwInfo, err := gateway.GetLowestLoadGateway()
	if err != nil {
		zap.L().Error("Failed to get available gateway for user",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, err
	}

	zap.L().Info("Selected gateway for user",
		zap.Int64("user_id", userID),
		zap.String("gateway_id", gwInfo.GatewayID),
		zap.Float32("load", gwInfo.Load))

	return &GatewayAvailableResponse{
		GatewayID: gwInfo.GatewayID,
		Address:   gwInfo.Address,
		Port:      gwInfo.Port,
	}, nil
}

// DeleteUserRoute explicitly deletes a user route (for cleanup)
func DeleteUserRoute(userID int64) error {
	key := userRoutePrefix + strconv.FormatInt(userID, 10)
	err := redis.DelWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to delete user route", zap.Int64("user_id", userID), zap.Error(err))
		return err
	}
	return nil
}
