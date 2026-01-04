package registry_client

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// GatewayClient provides Gateway-specific registry operations
type GatewayClient struct {
	*Client
	gatewayID string
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(registryURL, gatewayID string) *GatewayClient {
	return &GatewayClient{
		Client:    NewClient(registryURL),
		gatewayID: gatewayID,
	}
}

// RegisterGatewayRequest represents gateway registration request
type RegisterGatewayRequest struct {
	GatewayID string `json:"gateway_id"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Capacity  int    `json:"capacity"`
}

// HeartbeatGatewayRequest represents gateway heartbeat request
type HeartbeatGatewayRequest struct {
	GatewayID      string  `json:"gateway_id"`
	Load           float32 `json:"load"`
	ConnectedUsers int     `json:"connected_users"`
}

// Register registers the gateway with the registry service
func (gc *GatewayClient) Register(address string, port int, capacity int) error {
	req := map[string]interface{}{
		"gateway_id":  gc.gatewayID,
		"address":     address,
		"port":        port,
		"capacity":        capacity,

	}

	resp, err := gc.doRequest("POST", "/registry/gateway/register", req, 2)
	if err != nil {
		zap.L().Error("Failed to register gateway",
			zap.String("gateway_id", gc.gatewayID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("register failed: %s", resp.Message)
	}

	zap.L().Info("Gateway registered successfully", zap.String("gateway_id", gc.gatewayID))
	return nil
}

// Heartbeat sends heartbeat to registry service
func (gc *GatewayClient) Heartbeat(load float32, connections int, cpu float32, memory int64) error {
	req := map[string]interface{}{
		"gateway_id":  gc.gatewayID,
		"load":        load,
		"connections": connections,
		"cpu":         cpu,
		"memory":      memory,
	}

	resp, err := gc.doRequest("POST", "/registry/gateway/heartbeat", req, 2)
	if err != nil {
		zap.L().Error("Failed to send gateway heartbeat",
			zap.String("gateway_id", gc.gatewayID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("heartbeat failed: %s", resp.Message)
	}

	zap.L().Debug("Gateway heartbeat sent",
		zap.String("gateway_id", gc.gatewayID),
		zap.Float32("load", load),
		zap.Int("connections", connections))
	return nil
}

// Unregister unregisters the gateway from registry service
func (gc *GatewayClient) Unregister() error {
	path := fmt.Sprintf("/registry/gateway/%s", gc.gatewayID)
	resp, err := gc.doRequest("DELETE", path, nil, 2)
	if err != nil {
		zap.L().Error("Failed to unregister gateway",
			zap.String("gateway_id", gc.gatewayID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("unregister failed: %s", resp.Message)
	}

	zap.L().Info("Gateway unregistered successfully", zap.String("gateway_id", gc.gatewayID))
	return nil
}

// NotifyUserConnectRequest represents user connection notification
type NotifyUserConnectRequest struct {
	UserID    int64  `json:"user_id"`
	GatewayID string `json:"gateway_id"`
}

// ReportUserConnect notifies registry that a user has connected
func (gc *GatewayClient) ReportUserConnect(userID int64) error {
	req := NotifyUserConnectRequest{
		UserID:    userID,
		GatewayID: gc.gatewayID,
	}

	resp, err := gc.doRequest("POST", "/registry/user/connect", req, 2)
	if err != nil {
		zap.L().Error("Failed to notify user connect",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", gc.gatewayID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("notify user connect failed: %s", resp.Message)
	}

	zap.L().Debug("User connect notified",
		zap.Int64("user_id", userID),
		zap.String("gateway_id", gc.gatewayID))
	return nil
}

// ReportUserDisconnect notifies registry that a user has disconnected
func (gc *GatewayClient) ReportUserDisconnect(userID int64) error {
	req := NotifyUserDisconnectRequest{
		UserID:    userID,
		GatewayID: gc.gatewayID,
	}

	resp, err := gc.doRequest("POST", "/registry/user/disconnect", req, 2)
	if err != nil {
		zap.L().Error("Failed to notify user disconnect",
			zap.Int64("user_id", userID),
			zap.String("gateway_id", gc.gatewayID),
			zap.Error(err))
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("notify user disconnect failed: %s", resp.Message)
	}

	zap.L().Debug("User disconnect notified",
		zap.Int64("user_id", userID),
		zap.String("gateway_id", gc.gatewayID))
	return nil
}

// Legacy method aliases for backward compatibility
func (gc *GatewayClient) NotifyUserConnect(userID int64) error {
	return gc.ReportUserConnect(userID)
}

func (gc *GatewayClient) NotifyUserDisconnect(userID int64) error {
	return gc.ReportUserDisconnect(userID)
}

// NotifyUserDisconnectRequest represents user disconnection notification
type NotifyUserDisconnectRequest struct {
	UserID    int64  `json:"user_id"`
	GatewayID string `json:"gateway_id"`
}

// StartHeartbeat starts a goroutine that sends periodic heartbeats
func (gc *GatewayClient) StartHeartbeat(interval time.Duration, getLoadFunc func() (float32, int)) chan struct{} {
	stopCh := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				load, connectedUsers := getLoadFunc()
				if err := gc.Heartbeat(load, connectedUsers, 0, 0); err != nil {
					zap.L().Error("Heartbeat failed", zap.Error(err))
				}
			case <-stopCh:
				zap.L().Info("Heartbeat stopped", zap.String("gateway_id", gc.gatewayID))
				return
			}
		}
	}()

	return stopCh
}
