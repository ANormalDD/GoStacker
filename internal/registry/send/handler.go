package send

import (
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RegisterHandler handles send instance registration
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid send register request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := RegisterSendInstance(req)
	if err != nil {
		zap.L().Error("Failed to register send instance", zap.String("instance_id", req.InstanceID), zap.Error(err))
		response.ReplyError500(c, "Failed to register send instance: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Send instance registered successfully")
}

// HeartbeatHandler handles send instance heartbeat
func HeartbeatHandler(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid send heartbeat request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := UpdateSendHeartbeat(req)
	if err != nil {
		if err == ErrSendInstanceNotFound {
			zap.L().Warn("Send instance not found for heartbeat", zap.String("instance_id", req.InstanceID))
			response.ReplyNotFound(c, "Send instance not registered")
			return
		}
		zap.L().Error("Failed to update send heartbeat", zap.String("instance_id", req.InstanceID), zap.Error(err))
		response.ReplyError500(c, "Failed to update heartbeat: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Heartbeat updated successfully")
}

// UnregisterHandler handles send instance unregistration
func UnregisterHandler(c *gin.Context) {
	instanceID := c.Param("instance_id")
	if instanceID == "" {
		response.ReplyBadRequest(c, "instance_id is required")
		return
	}

	err := UnregisterSendInstance(instanceID)
	if err != nil {
		zap.L().Error("Failed to unregister send instance", zap.String("instance_id", instanceID), zap.Error(err))
		response.ReplyError500(c, "Failed to unregister send instance: "+err.Error())
		return
	}

	response.ReplySuccess(c, "Send instance unregistered successfully")
}

// ListInstancesHandler returns all registered send instances
func ListInstancesHandler(c *gin.Context) {
	instances, err := ListAllSendInstances()
	if err != nil {
		zap.L().Error("Failed to list send instances", zap.Error(err))
		response.ReplyError500(c, "Failed to list send instances: "+err.Error())
		return
	}

	response.ReplySuccessWithData(c, "success", gin.H{
		"instances": instances,
		"count":     len(instances),
	})
}

// GetAvailableSendHandler returns an available send instance for client requests
// Uses random selection for simple load balancing
func GetAvailableSendHandler(c *gin.Context) {
	sendInfo, err := GetRandomSendInstance()
	if err != nil {
		if err == ErrNoAvailableSend {
			zap.L().Error("No available send instances")
			response.ReplyError500(c, "No available send instance")
			return
		}
		zap.L().Error("Failed to get available send instance", zap.Error(err))
		response.ReplyError500(c, "Failed to get available send instance: "+err.Error())
		return
	}

	response.ReplySuccessWithData(c, "available", gin.H{
		"instance_id": sendInfo.InstanceID,
		"address":     sendInfo.Address,
		"port":        sendInfo.Port,
		"url":         BuildSendHTTPURL(sendInfo),
	})
}
