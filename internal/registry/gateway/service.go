package gateway

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	// Redis key prefixes
	gatewayInfoPrefix  = "registry:gateway:"
	gatewayRankingKey  = "registry:gateway:ranking"
	gatewayInstanceKey = "registry:gateway:instances"
)

var (
	ErrGatewayNotFound    = errors.New("gateway not found")
	ErrNoAvailableGateway = errors.New("no available gateway")
	ErrInvalidGatewayData = errors.New("invalid gateway data")
)

// RegisterGateway registers a new gateway or updates existing one
func RegisterGateway(req RegisterRequest) error {
	now := time.Now()
	info := GatewayInfo{
		GatewayID:      req.GatewayID,
		Address:        req.Address,
		Port:           req.Port,
		Capacity:       req.Capacity,
		Load:           0,
		ConnectedUsers: 0,
		LastHeartbeat:  now,
	}

	// Serialize to JSON
	data, err := json.Marshal(info)
	if err != nil {
		zap.L().Error("Failed to marshal gateway info", zap.Error(err))
		return err
	}

	// Get TTL from config
	ttl := 30 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.GatewayHeartbeatTimeout) * time.Second
	}

	// Store gateway info in Redis with TTL
	key := gatewayInfoPrefix + req.GatewayID
	err = redis.SetEXWithRetry(2, key, string(data), ttl)
	if err != nil {
		zap.L().Error("Failed to set gateway info in Redis", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return err
	}

	// Add to ranking sorted set (score = 0 for new gateway)
	err = redis.ZAddWithRetry(2, gatewayRankingKey, 0, req.GatewayID)
	if err != nil {
		zap.L().Error("Failed to add gateway to ranking", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return err
	}

	// Add to instance set
	err = redis.SAddWithRetry(2, gatewayInstanceKey, req.GatewayID)
	if err != nil {
		zap.L().Error("Failed to add gateway to instance set", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return err
	}

	zap.L().Info("Gateway registered successfully",
		zap.String("gateway_id", req.GatewayID),
		zap.String("address", req.Address),
		zap.Int("port", req.Port),
		zap.Int("capacity", req.Capacity))

	return nil
}

// UpdateHeartbeat updates gateway heartbeat and load
func UpdateHeartbeat(req HeartbeatRequest) error {
	key := gatewayInfoPrefix + req.GatewayID

	// Get existing gateway info
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to get gateway info", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return ErrGatewayNotFound
	}

	var info GatewayInfo
	err = json.Unmarshal([]byte(data), &info)
	if err != nil {
		zap.L().Error("Failed to unmarshal gateway info", zap.Error(err))
		return ErrInvalidGatewayData
	}

	// Update fields
	info.Load = req.Load
	info.ConnectedUsers = req.ConnectedUsers
	info.LastHeartbeat = time.Now()

	// Serialize back
	newData, err := json.Marshal(info)
	if err != nil {
		zap.L().Error("Failed to marshal gateway info", zap.Error(err))
		return err
	}

	// Get TTL from config
	ttl := 30 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.GatewayHeartbeatTimeout) * time.Second
	}

	// Update in Redis with TTL refresh
	err = redis.SetEXWithRetry(2, key, string(newData), ttl)
	if err != nil {
		zap.L().Error("Failed to update gateway info", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return err
	}

	// Update load in ranking (score = load * 1000 for precision)
	score := float64(req.Load * 1000)
	err = redis.ZAddWithRetry(2, gatewayRankingKey, score, req.GatewayID)
	if err != nil {
		zap.L().Error("Failed to update gateway ranking", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		return err
	}

	zap.L().Debug("Gateway heartbeat updated",
		zap.String("gateway_id", req.GatewayID),
		zap.Float32("load", req.Load),
		zap.Int("connected_users", req.ConnectedUsers))

	return nil
}

// UnregisterGateway removes a gateway from registry
func UnregisterGateway(gatewayID string) error {
	key := gatewayInfoPrefix + gatewayID

	// Delete gateway info
	err := redis.DelWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to delete gateway info", zap.String("gateway_id", gatewayID), zap.Error(err))
		return err
	}

	// Remove from ranking
	err = redis.ZRemWithRetry(2, gatewayRankingKey, gatewayID)
	if err != nil {
		zap.L().Error("Failed to remove gateway from ranking", zap.String("gateway_id", gatewayID), zap.Error(err))
		return err
	}

	// Remove from instance set
	err = redis.SRemWithRetry(2, gatewayInstanceKey, gatewayID)
	if err != nil {
		zap.L().Error("Failed to remove gateway from instance set", zap.String("gateway_id", gatewayID), zap.Error(err))
		return err
	}

	zap.L().Info("Gateway unregistered successfully", zap.String("gateway_id", gatewayID))

	return nil
}

// GetGatewayInfo retrieves gateway information
func GetGatewayInfo(gatewayID string) (*GatewayInfo, error) {
	key := gatewayInfoPrefix + gatewayID
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		return nil, ErrGatewayNotFound
	}

	var info GatewayInfo
	err = json.Unmarshal([]byte(data), &info)
	if err != nil {
		zap.L().Error("Failed to unmarshal gateway info", zap.Error(err))
		return nil, ErrInvalidGatewayData
	}

	return &info, nil
}

// GetLowestLoadGateway returns the gateway with the lowest load
func GetLowestLoadGateway() (*GatewayInfo, error) {
	// Get gateway with lowest score from ranking
	members, err := redis.ZRangeWithScoresWithRetry(2, gatewayRankingKey, 0, 0)
	if err != nil || len(members) == 0 {
		zap.L().Warn("No available gateways in ranking")
		return nil, ErrNoAvailableGateway
	}

	gatewayID := members[0]

	// Get gateway info
	info, err := GetGatewayInfo(gatewayID)
	if err != nil {
		zap.L().Error("Failed to get gateway info for lowest load gateway",
			zap.String("gateway_id", gatewayID),
			zap.Error(err))
		return nil, err
	}

	// Check capacity
	if info.ConnectedUsers >= info.Capacity {
		zap.L().Warn("Lowest load gateway is at capacity",
			zap.String("gateway_id", gatewayID),
			zap.Int("connected", info.ConnectedUsers),
			zap.Int("capacity", info.Capacity))

		// Try next gateway
		if len(members) > 1 {
			return getNextAvailableGateway(1)
		}
		return nil, ErrNoAvailableGateway
	}

	return info, nil
}

// getNextAvailableGateway tries to get next available gateway from ranking
func getNextAvailableGateway(startIndex int) (*GatewayInfo, error) {
	members, err := redis.ZRangeWithScoresWithRetry(2, gatewayRankingKey, int64(startIndex), int64(startIndex+5))
	if err != nil || len(members) == 0 {
		return nil, ErrNoAvailableGateway
	}

	for _, gatewayID := range members {
		info, err := GetGatewayInfo(gatewayID)
		if err != nil {
			continue
		}
		if info.ConnectedUsers < info.Capacity {
			return info, nil
		}
	}

	return nil, ErrNoAvailableGateway
}

// ListAllGateways returns all registered gateways
func ListAllGateways() ([]GatewayInfo, error) {
	// Get all gateway IDs from instance set
	members, err := redis.SMembersWithRetry(2, gatewayInstanceKey)
	if err != nil {
		zap.L().Error("Failed to get gateway instances", zap.Error(err))
		return nil, err
	}

	var gateways []GatewayInfo
	for _, gatewayID := range members {
		info, err := GetGatewayInfo(gatewayID)
		if err != nil {
			// Skip expired or invalid gateways
			zap.L().Warn("Skipping invalid gateway", zap.String("gateway_id", gatewayID), zap.Error(err))
			continue
		}
		gateways = append(gateways, *info)
	}

	return gateways, nil
}

// IsGatewayHealthy checks if a gateway is still healthy (exists in Redis)
func IsGatewayHealthy(gatewayID string) bool {
	key := gatewayInfoPrefix + gatewayID
	exists, err := redis.ExistsWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to check gateway existence", zap.String("gateway_id", gatewayID), zap.Error(err))
		return false
	}
	return exists > 0
}

// GetGatewayCount returns the count of registered gateways
func GetGatewayCount() (int, error) {
	count, err := redis.SCardWithRetry(2, gatewayInstanceKey)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// BuildGatewayWebSocketURL builds the WebSocket URL for a gateway
func BuildGatewayWebSocketURL(info *GatewayInfo) string {
	return fmt.Sprintf("ws://%s:%d", info.Address, info.Port)
}

// BuildGatewayHTTPURL builds the HTTP URL for a gateway
func BuildGatewayHTTPURL(info *GatewayInfo) string {
	return fmt.Sprintf("http://%s:%d", info.Address, info.Port)
}

// GetGatewayAddress returns formatted address string
func GetGatewayAddress(info *GatewayInfo) string {
	return info.Address + ":" + strconv.Itoa(info.Port)
}
