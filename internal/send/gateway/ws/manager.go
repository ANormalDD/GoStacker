package ws

import (
	"errors"
	"sync"
	"time"

	"GoStacker/pkg/config"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 单一存储，key: string(gatewayID), value: *ConnectionHolder
var connStore sync.Map

var ErrNoConn = errors.New("no connection for gateway")

type ConnectionHolder struct {
	Conn    *websocket.Conn
	sendCh  chan interface{}
	closeCh chan struct{}
}

// writerLoop 串行化所有对 websocket 的写入
func writerLoop(ch *ConnectionHolder) {
	for {
		select {
		case msg, ok := <-ch.sendCh:
			if !ok {
				// channel closed -> exit
				return
			}
			// default write deadline; callers may control timeout via SendToGateway
			// If the message is a control ping (int websocket.PingMessage), write control frame
			if mt, ok := msg.(int); ok && mt == websocket.PingMessage {
				ch.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			} else {
				ch.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := ch.Conn.WriteJSON(msg)
				if err != nil {
					zap.L().Error("Failed to write message to gateway", zap.Error(err))
					// on write error, close connection
					RemoveConnection(ch.Conn.RemoteAddr().String())
					return
				}
			}
		case <-ch.closeCh:
			return
		}
	}
}

func GetConnectionHolder(gatewayID string) (*ConnectionHolder, bool) {
	val, ok := connStore.Load(gatewayID)
	if !ok {
		return nil, false
	}
	return val.(*ConnectionHolder), true
}

// RegisterConnection 注册并启动与该连接关联的写入 goroutine
func RegisterConnection(gatewayID string, conn *websocket.Conn) {
	// 如果已有旧连接，先移除
	if val, ok := connStore.Load(gatewayID); ok {
		old := val.(*ConnectionHolder)
		// signal close
		close(old.closeCh)
		// close send channel to stop writerLoop
		close(old.sendCh)
		if old.Conn != nil {
			old.Conn.Close()
		}
		connStore.Delete(gatewayID)
	}

	// determine send channel buffer size from config, fallback to 128
	buf := 128
	if config.Conf != nil && config.Conf.SendDispatcherConfig != nil && config.Conf.SendDispatcherConfig.SendChannelSize > 0 {
		buf = config.Conf.SendDispatcherConfig.SendChannelSize
	}
	holder := &ConnectionHolder{
		Conn:    conn,
		sendCh:  make(chan interface{}, buf),
		closeCh: make(chan struct{}),
	}
	connStore.Store(gatewayID, holder)
	go writerLoop(holder)
}

// SendToGateway 将消息发送到指定 gateway 的发送队列。timeout 为等待发送入队的超时时间
func SendToGateway(gatewayID string, timeout time.Duration, message interface{}) error {
	zap.L().Debug("Sending message to gateway", zap.String("gateway_id", gatewayID), zap.Any("message", message))
	val, ok := connStore.Load(gatewayID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	// 将消息推入发送通道，带超时
	zap.L().Debug("Enqueuing message to gateway send channel", zap.String("gateway_id", gatewayID), zap.Any("message", message))
	select {
	case holder.sendCh <- message:
		return nil
	case <-time.After(timeout):
		return errors.New("send timeout")
	}
}

func RemoveConnection(gatewayID string) error {
	val, ok := connStore.Load(gatewayID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	// signal writer to exit
	select {
	default:
		// try close channels safely
		// close only if not already closed
		// use recover to avoid panic if closed elsewhere
		func() {
			defer func() { _ = recover() }()
			close(holder.closeCh)
			close(holder.sendCh)
		}()
	}
	if holder.Conn != nil {
		holder.Conn.Close()
	}
	connStore.Delete(gatewayID)
	return nil
}

// GetConnection 返回底层连接（不推荐并发写入）
func GetConnection(gatewayID string) (*websocket.Conn, bool) {
	val, ok := connStore.Load(gatewayID)
	if !ok {
		return nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, true
}
