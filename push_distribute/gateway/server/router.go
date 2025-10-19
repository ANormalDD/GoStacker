package server

import (
	"GoStacker/push_distribute/gateway/push"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewRouter builds and returns a gin.Engine with the necessary routes and middleware
func NewRouter() *gin.Engine {
	r := gin.New()
	// custom recovery to handle panics and perform handoff
	r.Use(func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				zap.L().Error("panic recovered in HTTP handler", zap.Any("recover", rec))
				// attempt to flush in-memory queues and unregister users
				push.HandlePanic()
				// return 500 to client
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	})

	r.POST("/forward", func(c *gin.Context) {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(body, &env); err != nil {
			zap.L().Warn("gateway forward invalid envelope", zap.Error(err))
			c.Status(http.StatusBadRequest)
			return
		}
		var uid int64
		if v, ok := env["user_id"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
					uid = parsed
				}
			} else {
				var n int64
				if err := json.Unmarshal(v, &n); err == nil {
					uid = n
				}
			}
		}
		if uid == 0 {
			zap.L().Warn("forward envelope missing user_id")
			c.Status(http.StatusBadRequest)
			return
		}
		msgRaw, ok := env["message"]
		if !ok {
			zap.L().Warn("forward envelope missing message")
			c.Status(http.StatusBadRequest)
			return
		}
		select {
		case push.PushTaskChan <- push.PushTask{UserID: uid, MarshaledMsg: msgRaw}:
			c.Status(http.StatusOK)
			return
		default:
			uidStr := strconv.FormatInt(uid, 10)
			if err := push.RedisRPushWait(uidStr, msgRaw); err != nil {
				zap.L().Error("Failed to RPush wait push message from forward", zap.Int64("userID", uid), zap.Error(err))
				c.Status(http.StatusInternalServerError)
				return
			}
			c.Status(http.StatusOK)
			return
		}
	})

	// WS endpoint will be mounted elsewhere (gin handler from ws package)

	return r
}
