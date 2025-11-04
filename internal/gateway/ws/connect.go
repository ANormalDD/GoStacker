package ws

import (
	"GoStacker/internal/gateway/pushback"
	"GoStacker/internal/gateway/register"
	"GoStacker/internal/gateway/userConn"
	"GoStacker/internal/gateway/loadupdate"
	"encoding/json"
	"net"
	"net/http"
	"strings"
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

// gateway会通过ws与中心服务器建立连接，并接收推送的消息
func WebSocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("Failed to upgrade to WebSocket (gateway)", zap.Error(err))
		return
	}

	// gateway 标识优先从 query 获取，然后从 Header 获取
	gatewayID := c.Query("gateway_id")
	if gatewayID == "" {
		gatewayID = c.GetHeader("Gateway-ID")
	}
	if gatewayID == "" {
		// 无 id，直接关闭
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "missing gateway_id"))
		conn.Close()
		return
	}

	// 注册到 manager
	RegisterConnection(gatewayID, conn)
	zap.L().Info("Gateway connected", zap.String("gateway_id", gatewayID))

	// heartbeat constants
	const (
		pongWait   = 60 * time.Second
		pingPeriod = (pongWait * 9) / 10
		writeWait  = 10 * time.Second
	)

	// set initial read deadline and pong handler
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// periodic pings (send via per-connection writer goroutine)
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			err := SendToGateway(gatewayID, writeWait, websocket.PingMessage)
			if err != nil {
				if err == ErrNoConn {
					zap.L().Info("Gateway connection missing, stop heartbeat", zap.String("gateway_id", gatewayID))
					return
				}
				zap.L().Error("Failed to send ping to gateway", zap.String("gateway_id", gatewayID), zap.Error(err))
				// quick retry
				time.Sleep(100 * time.Millisecond)
				err = SendToGateway(gatewayID, writeWait, websocket.PingMessage)
				if err != nil {
					zap.L().Error("Ping retry failed, removing gateway connection", zap.String("gateway_id", gatewayID), zap.Error(err))
					RemoveConnection(gatewayID)
					return
				}
			}
		}
	}()

	// read loop
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zap.L().Info("Gateway WebSocket closed by peer", zap.String("gateway_id", gatewayID), zap.Error(err))
				break
			}
			if ne, ok := err.(net.Error); ok {
				if ne.Timeout() {
					zap.L().Info("Gateway read timeout (deadline exceeded)", zap.String("gateway_id", gatewayID), zap.Error(err))
					break
				}
				if ne.Temporary() {
					zap.L().Warn("Temporary network error reading gateway message", zap.String("gateway_id", gatewayID), zap.Error(err))
					continue
				}
			}
			if strings.Contains(err.Error(), "use of closed network connection") || strings.Contains(err.Error(), "EOF") {
				zap.L().Info("Gateway WebSocket connection closed (EOF)", zap.String("gateway_id", gatewayID), zap.Error(err))
				break
			}
			zap.L().Error("Failed to read gateway WebSocket message", zap.String("gateway_id", gatewayID), zap.Error(err))
			break
		}

		zap.L().Debug("Received message from gateway", zap.String("gateway_id", gatewayID), zap.ByteString("msg", message))

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			zap.L().Error("Failed to unmarshal gateway message", zap.Error(err))
			continue
		}

		t, _ := msg["type"].(string)
		switch t {
		case "user_connect":
			go userConn.UserConnHandler(msg, gatewayID)
		case "register_gateway":
			go register.RegisterGatewayHandler(msg, gatewayID)
		case "pushback":
			// client 将推送消息回传给中心，委托给 pushback 包处理
			go pushback.PushBackHandler(msg)
		case "user_disconnect":
			go userConn.UserDisconnHandler(msg)
		case "load_update":
			go loadupdate.LoadUpdateHandler(msg, gatewayID)
		default:
			zap.L().Warn("Unknown gateway message type, ignoring", zap.String("type", t))
		}
	}
	RemoveConnection(gatewayID)
	zap.L().Info("Gateway WebSocket connection closed and removed", zap.String("gateway_id", gatewayID))
}
