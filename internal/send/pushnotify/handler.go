package pushnotify

import (
	"GoStacker/pkg/push"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type NotifyRequest struct {
	TargetID int64 `json:"target_id" binding:"required"`
}

// NotifyOnlineHandler handles registry notification that a user came online.
// It triggers PushOfflineMessages asynchronously and returns 202 Accepted immediately.
func NotifyOnlineHandler(c *gin.Context) {
	var req NotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zap.L().Warn("NotifyOnlineHandler: invalid request", zap.Error(err))
		response.ReplyBadRequest(c, "invalid request")
		return
	}

	// trigger asynchronous offline push
	go func(uid int64) {
		zap.L().Info("NotifyOnlineHandler: triggering PushOfflineMessages", zap.Int64("user", uid))
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("PushOfflineMessages panic recovered", zap.Any("panic", r))
			}
		}()
		push.PushOfflineMessages(uid)
	}(req.TargetID)

	// return accepted
	c.JSON(202, response.StandardResponse{Code: 0, Msg: "Accepted"})
}
