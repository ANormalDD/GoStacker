package ws

import (
	"GoStacker/pkg/push"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WebSocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Unauthorized"))
		conn.Close()
		return
	}
	userIDInt64 := userID.(int64)
	push.RegisterConnection(userIDInt64, conn)

	push.PushViaWebSocket(userIDInt64, push.ClientMessage{
		ID:       -1,
		Type:     "system",
		RoomID:   -1,
		SenderID: -1,
		Payload:  "Connected to WebSocket server",
	})
	push.PushOfflineMessages(userIDInt64)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				push.RemoveConnection(userIDInt64)
				conn.Close()
				break
			}
		}
	}()
}
