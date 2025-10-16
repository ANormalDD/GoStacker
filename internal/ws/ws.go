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

	push.PushViaWS(userIDInt64, 10*time.Second, push.ClientMessage{
		ID:       -1,
		Type:     "system",
		RoomID:   -1,
		SenderID: -1,
		Payload:  "Connected to WebSocket server",
	})
	push.PushOfflineMessages(userIDInt64)
	// heartbeat: use pong handler to extend read deadline and periodic pings
	const (
		pongWait   = 60 * time.Second
		pingPeriod = (pongWait * 9) / 10 // send pings slightly before pong timeout
		writeWait  = 10 * time.Second
	)

	// set initial read deadline and pong handler
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(appData string) error {
		// extend read deadline on pong
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			holder, ok := push.GetConnectionHolder(userIDInt64)
			if !ok {
				zap.L().Warn("Connection holder not found during heartbeat", zap.Int64("userID", userIDInt64))
				continue
			}

			err := push.WriteJSONSafe(holder, writeWait, websocket.PingMessage)

			if err != nil {
				if err == push.ErrNoConn {
					zap.L().Info("Connection closed, stopping heartbeat", zap.Int64("userID", userIDInt64))
					return
				}
				zap.L().Error("Failed to send ping", zap.Int64("userID", userIDInt64), zap.Error(err))
				// one quick retry
				time.Sleep(100 * time.Millisecond)
				err = push.WriteJSONSafe(holder, writeWait, websocket.PingMessage)
				if err != nil {
					zap.L().Error("Ping retry failed, removing connection", zap.Int64("userID", userIDInt64), zap.Error(err))
					push.RemoveConnection(userIDInt64)
					return
				}
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
