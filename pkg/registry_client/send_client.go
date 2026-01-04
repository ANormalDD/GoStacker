package registry_client

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SendClient provides Send-specific registry operations
type SendClient struct {
	*Client
	instanceID string
}

// NewSendClient creates a new send client
func NewSendClient(registryURL, instanceID string) *SendClient {
	return &SendClient{
		Client:     NewClient(registryURL),
		instanceID: instanceID,
	}
}

// RegisterSendRequest represents send instance registration request
type RegisterSendRequest struct {
	InstanceID string `json:"instance_id"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
}

// HeartbeatSendRequest represents send instance heartbeat request
type HeartbeatSendRequest struct {
	InstanceID string `json:"instance_id"`
}

// Register registers the send instance with the registry service
func (sc *SendClient) Register(address string, port int) error {
	req := RegisterSendRequest{
		InstanceID: sc.instanceID,
		Address:    address,
		Port:       port,
	}

	resp, err := sc.doRequest("POST", "/registry/send/register", req, 2)
	if err != nil {
		zap.L().Error("Failed to register send instance",
			zap.String("instance_id", sc.instanceID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("register failed: %s", resp.Message)
	}

	zap.L().Info("Send instance registered successfully", zap.String("instance_id", sc.instanceID))
	return nil
}

// Heartbeat sends heartbeat to registry service
func (sc *SendClient) Heartbeat() error {
	req := HeartbeatSendRequest{
		InstanceID: sc.instanceID,
	}

	resp, err := sc.doRequest("POST", "/registry/send/heartbeat", req, 2)
	if err != nil {
		zap.L().Error("Failed to send send heartbeat",
			zap.String("instance_id", sc.instanceID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("heartbeat failed: %s", resp.Message)
	}

	zap.L().Debug("Send heartbeat sent", zap.String("instance_id", sc.instanceID))
	return nil
}

// Unregister unregisters the send instance from registry service
func (sc *SendClient) Unregister() error {
	path := fmt.Sprintf("/registry/send/%s", sc.instanceID)
	resp, err := sc.doRequest("DELETE", path, nil, 2)
	if err != nil {
		zap.L().Error("Failed to unregister send instance",
			zap.String("instance_id", sc.instanceID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("unregister failed: %s", resp.Message)
	}

	zap.L().Info("Send instance unregistered successfully", zap.String("instance_id", sc.instanceID))
	return nil
}

// RouteInfo represents user route information
type RouteInfo struct {
	GatewayID string `json:"gateway_id"`
	Address   string `json:"address"`
}

// BatchQueryRoutesRequest represents batch route query request
type BatchQueryRoutesRequest struct {
	UserIDs []int64 `json:"user_ids"`
}

// QueryUserRoutes queries routes for multiple users at once
func (sc *SendClient) QueryUserRoutes(userIDs []int64) (map[int64]*RouteInfo, error) {
	req := BatchQueryRoutesRequest{
		UserIDs: userIDs,
	}

	resp, err := sc.doRequest("POST", "/registry/user/routes/batch", req, 2)
	if err != nil {
		zap.L().Error("Failed to query user routes", zap.Error(err))
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("query user routes failed: %s", resp.Message)
	}

	// Parse data
	result := make(map[int64]*RouteInfo)
	if resp.Data != nil {
		// Convert data to map
		dataBytes, err := json.Marshal(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}
		err = json.Unmarshal(dataBytes, &result)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal route data: %w", err)
		}
	}

	return result, nil
}

// SendInstanceInfo represents send instance information
type SendInstanceInfo struct {
	InstanceID string `json:"instance_id"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
}

// ListSendInstances returns all available send instances
func (sc *SendClient) ListSendInstances() ([]SendInstanceInfo, error) {
	resp, err := sc.doRequest("GET", "/registry/send/instances", nil, 2)
	if err != nil {
		zap.L().Error("Failed to list send instances", zap.Error(err))
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("list send instances failed: %s", resp.Message)
	}

	// Parse data
	var result struct {
		Instances []SendInstanceInfo `json:"instances"`
	}
	if resp.Data != nil {
		dataBytes, err := json.Marshal(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}
		err = json.Unmarshal(dataBytes, &result)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal instances data: %w", err)
		}
	}

	return result.Instances, nil
}

// StartHeartbeat starts a goroutine that sends periodic heartbeats
func (sc *SendClient) StartHeartbeat(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := sc.Heartbeat(); err != nil {
					zap.L().Error("Heartbeat failed", zap.Error(err))
				}
			case <-stopCh:
				zap.L().Info("Heartbeat stopped", zap.String("instance_id", sc.instanceID))
				return
			}
		}
	}()

	return stopCh
}
