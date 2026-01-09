package send

import (
	"GoStacker/pkg/response"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// 使用 RawMessage 接收 content，后续按 type 字段反序列化为具体 ChatPayload
type SendMessageRequest struct {
	RoomID  int64           `json:"room_id" binding:"required"`
	Content json.RawMessage `json:"content" binding:"required"`
}

type ResendMessageRequest struct {
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

	// 将 raw content 解析为具体的 ChatPayload
	payload, err := UnmarshalChatPayload(req.Content)
	if err != nil {
		response.ReplyBadRequest(c, "Invalid content: "+err.Error())
		return
	}

	id, err = SendMessage(req.RoomID, userID, payload)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "success", gin.H{"msgID": id})
}

func ResendHandler(c *gin.Context) {
	var req ResendMessageRequest
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
	cm, err := getMsgInfoByID(req.MessageID)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	if cm.SenderID != userID {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	payload, err := UnmarshalChatPayload(cm.Content)
	if err != nil {
		response.ReplyBadRequest(c, "Invalid content: "+err.Error())
		return
	}
	//to do,check msg
	err = BroadcastMessage(req.MessageID, cm.RoomID, userID, payload)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "success")
}
