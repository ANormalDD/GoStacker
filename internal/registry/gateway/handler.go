package gateway

import (
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RegisterHandler handles gateway registration
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid gateway register request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := RegisterGateway(req)
	if err != nil {
		zap.L().Error("Failed to register gateway", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		response.ReplyError500(c, "Failed to register gateway: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Gateway registered successfully")
}

// HeartbeatHandler handles gateway heartbeat
func HeartbeatHandler(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid gateway heartbeat request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := UpdateHeartbeat(req)
	if err != nil {
		if err == ErrGatewayNotFound {
			zap.L().Warn("Gateway not found for heartbeat", zap.String("gateway_id", req.GatewayID))
			response.ReplyNotFound(c, "Gateway not registered")
			return
		}
		zap.L().Error("Failed to update gateway heartbeat", zap.String("gateway_id", req.GatewayID), zap.Error(err))
		response.ReplyError500(c, "Failed to update heartbeat: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Heartbeat updated successfully")
}

// UnregisterHandler handles gateway unregistration
func UnregisterHandler(c *gin.Context) {
	gatewayID := c.Param("gateway_id")
	if gatewayID == "" {
		response.ReplyBadRequest(c, "gateway_id is required")
		return
	}

	err := UnregisterGateway(gatewayID)
	if err != nil {
		zap.L().Error("Failed to unregister gateway", zap.String("gateway_id", gatewayID), zap.Error(err))
		response.ReplyError500(c, "Failed to unregister gateway: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Gateway unregistered successfully")
}

// ListGatewaysHandler returns all registered gateways
func ListGatewaysHandler(c *gin.Context) {
	gateways, err := ListAllGateways()
	if err != nil {
		zap.L().Error("Failed to list gateways", zap.Error(err))
		response.ReplyError500(c, "Failed to list gateways: "+err.Error())
		return
	}

	response.ReplySuccessWithData(c, "success", gin.H{
		"gateways": gateways,
		"count":    len(gateways),
	})
}

// HealthCheckHandler returns health status of registry service
func HealthCheckHandler(c *gin.Context) {
	resp := HealthCheckResponse{
		Status: "healthy",
		Redis:  "disconnected",
	}

	// Check Redis connection
	_, err := redis.Ping()
	if err == nil {
		resp.Redis = "connected"
	}

	// Get counts
	gwCount, _ := GetGatewayCount()
	resp.GatewayCount = gwCount

	sendCount, err := getSendInstanceCount()
	if err == nil {
		resp.SendCount = sendCount
	}

	userRouteCount, err := getUserRouteCount()
	if err == nil {
		resp.UserRouteCount = userRouteCount
	}

	if resp.Redis == "disconnected" {
		resp.Status = "unhealthy"
		c.JSON(503, resp)
		return
	}

	c.JSON(200, resp)
}

// Helper function to get send instance count (will be implemented in send package)
func getSendInstanceCount() (int, error) {
	// Import will be added when send package is created
	// For now, return placeholder
	count, err := redis.SCardWithRetry(2, "registry:send:instances")
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Helper function to get user route count
func getUserRouteCount() (int, error) {
	// This would require scanning keys or maintaining a counter
	// For now, return 0 as it's expensive to count all user routes
	return 0, nil
}
