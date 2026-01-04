package send

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
	sendInstancePrefix = "registry:send:"
	sendInstanceSetKey = "registry:send:instances"
)

var (
	ErrSendInstanceNotFound = errors.New("send instance not found")
	ErrInvalidSendData      = errors.New("invalid send instance data")
	ErrNoAvailableSend      = errors.New("no available send instance")
)

// RegisterSendInstance registers a new send instance or updates existing one
func RegisterSendInstance(req RegisterRequest) error {
	now := time.Now()
	info := SendInstanceInfo{
		InstanceID:    req.InstanceID,
		Address:       req.Address,
		Port:          req.Port,
		LastHeartbeat: now,
	}

	// Serialize to JSON
	data, err := json.Marshal(info)
	if err != nil {
		zap.L().Error("Failed to marshal send instance info", zap.Error(err))
		return err
	}

	// Get TTL from config
	ttl := 30 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.SendHeartbeatTimeout) * time.Second
	}

	// Store send instance info in Redis with TTL
	key := sendInstancePrefix + req.InstanceID
	err = redis.SetEXWithRetry(2, key, string(data), ttl)
	if err != nil {
		zap.L().Error("Failed to set send instance info in Redis", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return err
	}

	// Add to instance set
	err = redis.SAddWithRetry(2, sendInstanceSetKey, req.InstanceID)
	if err != nil {
		zap.L().Error("Failed to add send instance to set", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return err
	}

	zap.L().Info("Send instance registered successfully",
		zap.String("instance_id", req.InstanceID),
		zap.String("address", req.Address),
		zap.Int("port", req.Port))

	return nil
}

// UpdateSendHeartbeat updates send instance heartbeat
func UpdateSendHeartbeat(req HeartbeatRequest) error {
	key := sendInstancePrefix + req.InstanceID

	// Get existing send instance info
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to get send instance info", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return ErrSendInstanceNotFound
	}

	var info SendInstanceInfo
	err = json.Unmarshal([]byte(data), &info)
	if err != nil {
		zap.L().Error("Failed to unmarshal send instance info", zap.Error(err))
		return ErrInvalidSendData
	}

	// Update heartbeat time
	info.LastHeartbeat = time.Now()

	// Serialize back
	newData, err := json.Marshal(info)
	if err != nil {
		zap.L().Error("Failed to marshal send instance info", zap.Error(err))
		return err
	}

	// Get TTL from config
	ttl := 30 * time.Second
	if config.Conf != nil && config.Conf.RegistryConfig != nil {
		ttl = time.Duration(config.Conf.RegistryConfig.SendHeartbeatTimeout) * time.Second
	}

	// Update in Redis with TTL refresh
	err = redis.SetEXWithRetry(2, key, string(newData), ttl)
	if err != nil {
		zap.L().Error("Failed to update send instance info", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return err
	}

	zap.L().Debug("Send instance heartbeat updated", zap.String("instance_id", req.InstanceID))

	return nil
}

// UnregisterSendInstance removes a send instance from registry
func UnregisterSendInstance(instanceID string) error {
	key := sendInstancePrefix + instanceID

	// Delete send instance info
	err := redis.DelWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to delete send instance info", zap.String("instance_id", instanceID), zap.Error(err))
		return err
	}

	// Remove from instance set
	err = redis.SRemWithRetry(2, sendInstanceSetKey, instanceID)
	if err != nil {
		zap.L().Error("Failed to remove send instance from set", zap.String("instance_id", instanceID), zap.Error(err))
		return err
	}

	zap.L().Info("Send instance unregistered successfully", zap.String("instance_id", instanceID))

	return nil
}

// GetSendInstanceInfo retrieves send instance information
func GetSendInstanceInfo(instanceID string) (*SendInstanceInfo, error) {
	key := sendInstancePrefix + instanceID
	data, err := redis.GetWithRetry(2, key)
	if err != nil {
		return nil, ErrSendInstanceNotFound
	}

	var info SendInstanceInfo
	err = json.Unmarshal([]byte(data), &info)
	if err != nil {
		zap.L().Error("Failed to unmarshal send instance info", zap.Error(err))
		return nil, ErrInvalidSendData
	}

	return &info, nil
}

// ListAllSendInstances returns all registered send instances
func ListAllSendInstances() ([]SendInstanceInfo, error) {
	// Get all instance IDs from set
	members, err := redis.SMembersWithRetry(2, sendInstanceSetKey)
	if err != nil {
		zap.L().Error("Failed to get send instances", zap.Error(err))
		return nil, err
	}

	var instances []SendInstanceInfo
	for _, instanceID := range members {
		info, err := GetSendInstanceInfo(instanceID)
		if err != nil {
			// Skip expired or invalid instances
			zap.L().Warn("Skipping invalid send instance", zap.String("instance_id", instanceID), zap.Error(err))
			// Remove from set if not found
			redis.SRemWithRetry(2, sendInstanceSetKey, instanceID)
			continue
		}
		instances = append(instances, *info)
	}

	return instances, nil
}

// GetRandomSendInstance returns a random available send instance (for load balancing)
func GetRandomSendInstance() (*SendInstanceInfo, error) {
	// Get a random member from the set
	instanceID, err := redis.SRandMemberWithRetry(2, sendInstanceSetKey)
	if err != nil || instanceID == "" {
		zap.L().Warn("No available send instances")
		return nil, ErrNoAvailableSend
	}

	// Get instance info
	info, err := GetSendInstanceInfo(instanceID)
	if err != nil {
		zap.L().Error("Failed to get send instance info",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		// Remove invalid instance from set
		redis.SRemWithRetry(2, sendInstanceSetKey, instanceID)
		return nil, err
	}

	return info, nil
}

// IsSendInstanceHealthy checks if a send instance is still healthy
func IsSendInstanceHealthy(instanceID string) bool {
	key := sendInstancePrefix + instanceID
	exists, err := redis.ExistsWithRetry(2, key)
	if err != nil {
		zap.L().Error("Failed to check send instance existence", zap.String("instance_id", instanceID), zap.Error(err))
		return false
	}
	return exists > 0
}

// GetSendInstanceCount returns the count of registered send instances
func GetSendInstanceCount() (int, error) {
	count, err := redis.SCardWithRetry(2, sendInstanceSetKey)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// BuildSendHTTPURL builds the HTTP URL for a send instance
func BuildSendHTTPURL(info *SendInstanceInfo) string {
	return fmt.Sprintf("http://%s:%d", info.Address, info.Port)
}

// GetSendInstanceAddress returns formatted address string
func GetSendInstanceAddress(info *SendInstanceInfo) string {
	return info.Address + ":" + strconv.Itoa(info.Port)
}
