package user

import (
	"GoStacker/internal/registry/gateway"
	sendreg "GoStacker/internal/registry/send"
	"GoStacker/pkg/response"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ConnectHandler handles user connection notification
func ConnectHandler(c *gin.Context) {
	var req ConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid user connect request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := RecordUserConnect(req.UserID, req.GatewayID)
	if err != nil {
		zap.L().Error("Failed to record user connection",
			zap.Int64("user_id", req.UserID),
			zap.String("gateway_id", req.GatewayID),
			zap.Error(err))
		response.ReplyError500(c, "Failed to record user connection: "+err.Error())
		return
	}

	// Trigger send service to push offline messages for this user asynchronously.
	go func(uid int64) {
		// choose a random send instance
		inst, err := sendreg.GetRandomSendInstance()
		if err != nil {
			zap.L().Warn("No available send instance to notify online", zap.Int64("user", uid), zap.Error(err))
			return
		}

		url := sendreg.BuildSendHTTPURL(inst) + "/internal/push/notify_online"
		bodyMap := map[string]interface{}{"target_id": uid}
		data, _ := json.Marshal(bodyMap)
		client := &http.Client{Timeout: 3 * time.Second}

		// try a couple of times
		for i := 0; i < 2; i++ {
			req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				zap.L().Warn("Failed to notify send instance", zap.String("url", url), zap.Int("attempt", i), zap.Error(err))
				time.Sleep(100 * time.Millisecond)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				zap.L().Info("Notified send instance to push offline messages", zap.String("url", url), zap.Int64("user", uid))
				return
			}
			zap.L().Warn("Send notify returned non-2xx", zap.Int("status", resp.StatusCode), zap.String("url", url))
			time.Sleep(100 * time.Millisecond)
		}
	}(req.UserID)

	response.ReplySuccess(c, "User connection recorded successfully")
}

// DisconnectHandler handles user disconnection notification
func DisconnectHandler(c *gin.Context) {
	var req DisconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid user disconnect request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := RecordUserDisconnect(req.UserID, req.GatewayID)
	if err != nil {
		zap.L().Error("Failed to record user disconnection",
			zap.Int64("user_id", req.UserID),
			zap.String("gateway_id", req.GatewayID),
			zap.Error(err))
		response.ReplyError500(c, "Failed to record user disconnection: "+err.Error())
		return
	}

	response.ReplySuccess(c, "User disconnection recorded successfully")
}

// BatchQueryRoutesHandler handles batch user route queries
func BatchQueryRoutesHandler(c *gin.Context) {
	var req BatchQueryRoutesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid batch query routes request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.UserIDs) == 0 {
		response.ReplyBadRequest(c, "user_ids cannot be empty")
		return
	}

	routes, err := BatchGetUserRoutes(req.UserIDs)
	if err != nil {
		zap.L().Error("Failed to batch query user routes", zap.Error(err))
		response.ReplyError500(c, "Failed to query user routes: "+err.Error())
		return
	}

	response.ReplySuccessWithData(c, "success", routes)
}

// GetAvailableGatewayHandler returns an available gateway for client connection
// This is called by clients to get a gateway address before connecting
func GetAvailableGatewayHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		// If no user_id provided, just return lowest load gateway
		gwInfo, err := gateway.GetLowestLoadGateway()
		if err != nil {
			if err == gateway.ErrNoAvailableGateway {
				zap.L().Error("No available gateway")
				response.ReplyError500(c, "No available gateway")
				return
			}
			zap.L().Error("Failed to get available gateway", zap.Error(err))
			response.ReplyError500(c, "Failed to get available gateway: "+err.Error())
			return
		}

		response.ReplySuccessWithData(c, "available", gin.H{
			"gateway_id": gwInfo.GatewayID,
			"address":    gwInfo.Address,
			"port":       gwInfo.Port,
		})
		return
	}

	// Parse user_id
	var userID int64
	if err := json.Unmarshal([]byte(userIDStr), &userID); err != nil {
		// Try parsing as string integer
		if parsed, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = parsed
		} else {
			response.ReplyBadRequest(c, "Invalid user_id format")
			return
		}
	}

	// Get available gateway for specific user (with reconnection optimization)
	gwResp, err := GetAvailableGatewayForUser(userID)
	if err != nil {
		if err == gateway.ErrNoAvailableGateway {
			zap.L().Error("No available gateway for user", zap.Int64("user_id", userID))
			response.ReplyError500(c, "No available gateway")
			return
		}
		zap.L().Error("Failed to get available gateway for user",
			zap.Int64("user_id", userID),
			zap.Error(err))
		response.ReplyError500(c, "Failed to get available gateway: "+err.Error())
		return
	}

	response.ReplySuccessWithData(c, "available", gin.H{
		"gateway_id": gwResp.GatewayID,
		"address":    gwResp.Address,
		"port":       gwResp.Port,
	})
}
