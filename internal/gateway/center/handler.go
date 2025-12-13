package center

import (
	"GoStacker/internal/gateway/push"
	"GoStacker/internal/gateway/push/types"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

func ForwardHandler(c *gin.Context) {
	var msg types.PushMessage
	if err := c.ShouldBindJSON(&msg); err != nil {
		response.ReplyBadRequest(c, "invalid request payload")
		return
	}
	if err := push.Dispatch(msg); err != nil {
		response.ReplyError500(c, "failed to forward message")
		return
	}
	response.ReplySuccess(c, "message forwarded successfully")
}
