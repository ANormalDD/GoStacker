package send

import (
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

type SendMessageRequest struct {
	RoomID  int64       `json:"room_id" binding:"required"`
	Content ChatPayload `json:"content" binding:"required"`
}

type RecallMessageRequest struct {
	MessageID int64 `json:"message_id" binding:"required"`
}

func SendMessageHandler(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	id, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID := id.(int64)
	err := SendMessage(req.RoomID, userID, req.Content)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Message sent successfully")

}
