package ws

import (
	"GoStacker/pkg/push"
	"net/http"
	"time"

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
	//heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			lock, _ := push.GetConnectionLock(userIDInt64)
			lock.Lock()
			err := conn.WriteMessage(websocket.PingMessage, []byte{})
			lock.Unlock()
			if err != nil {
				zap.L().Error("Failed to send ping", zap.Error(err))
				push.RemoveConnection(userIDInt64)
				return
			}
		}
	}()
	//read loop
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	push.RemoveConnection(userIDInt64)
	zap.L().Info("WebSocket connection closed", zap.Int64("userID", userIDInt64))
}
