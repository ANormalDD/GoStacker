package pushback

import (
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/response"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PushbackRequest represents the HTTP request for pushback
type PushbackRequest struct {
	TargetID   int64       `json:"target_id" binding:"required"`
	ForwardReq interface{} `json:"forward_req" binding:"required"`
}

// PushbackHandler handles HTTP pushback requests from Gateway
// Gateway calls this API to push offline messages to Send service
func PushbackHandler(c *gin.Context) {
	var req PushbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("Invalid pushback request", zap.Error(err))
		response.ReplyBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Marshal the forward request
	marshaledMsg, err := json.Marshal(req.ForwardReq)
	if err != nil {
		zap.L().Error("Failed to marshal forward_req",
			zap.Int64("target_id", req.TargetID),
			zap.Error(err))
		response.ReplyError500(c, "Failed to marshal message")
		return
	}

	// Push to Redis offline queue
	queueKey := "offline:push:" + strconv.FormatInt(req.TargetID, 10)
	err = redis.RPushWithRetry(2, queueKey, marshaledMsg)
	if err != nil {
		zap.L().Error("Failed to push offline message to Redis",
			zap.Int64("target_id", req.TargetID),
			zap.String("queue_key", queueKey),
			zap.Error(err))
		response.ReplyError500(c, "Failed to push offline message")
		return
	}

	zap.L().Debug("Pushback message stored in offline queue",
		zap.Int64("target_id", req.TargetID),
		zap.String("queue_key", queueKey))

	response.ReplySuccess(c, "Pushback success")
}
