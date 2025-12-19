package push

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"GoStacker/pkg/pendingTask"
	"GoStacker/internal/gateway/centerclient"
	"GoStacker/internal/gateway/push/types"
	"GoStacker/pkg/config"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var connStore sync.Map // key: int64, value: *ConnectionHolder

var ErrNoConn = errors.New("no connection for user")

var connCount int = 0
var connCountLock sync.Mutex

var totalPending int64

type sendRequest struct {
	msg  interface{}
	resp chan error
}

type ConnectionHolder struct {
	Conn    *websocket.Conn
	sendCh  chan sendRequest
	closeCh chan struct{}
}

func incPending(count int64) {
	atomic.AddInt64(&totalPending, count)
}

func decPending(count int64) {
	atomic.AddInt64(&totalPending, -count)
	PullTask()
}

// writerLoop 串行化对 websocket 的写入并在完成后将结果返回给请求方
func writerLoop(ch *ConnectionHolder) {
	for {
		select {
		case req, ok := <-ch.sendCh:
			if !ok {
				return
			}
			// one task consumed from channel
			decPending(1)
			var err error
			// If caller passed a websocket control message type (e.g. websocket.PingMessage)
			// write it as a control frame instead of JSON.
			if mt, ok := req.msg.(int); ok && mt == websocket.PingMessage {
				err = ch.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			} else {
				msgraw, ok := req.msg.(types.ClientMessage)
				if ok {
					pendingTask.DefaultPendingManager.Done(msgraw.ID)
				}
				zap.L().Debug("writerLoop sending message", zap.Any("message", req.msg))
				ch.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = ch.Conn.WriteJSON(req.msg)
			}
			// send result back if caller expects it
			if req.resp != nil {
				select {
				case req.resp <- err:
				default:
				}
			}
			if err != nil {
				// on write error, close connection and exit writer
				ch.Conn.Close()
				return
			}
		case <-ch.closeCh:
			return
		}
	}
}

// WriteJSONSafe 将消息封装成请求并发送到连接的 writer goroutine，等待写入完成或超时
func WriteJSONSafe(holder *ConnectionHolder, timeout time.Duration, message interface{}) error {
	if holder == nil {
		return ErrNoConn
	}
	// prepare request
	req := sendRequest{msg: message, resp: make(chan error, 1)}
	// enqueue with timeout
	select {
	case holder.sendCh <- req:
		// increment global task count for this enqueue
		incPending(1)
		// wait for write result or timeout
		select {
		case err := <-req.resp:
			if err != nil {
				return err
			}
			return nil
		case <-time.After(timeout):
			return errors.New("write result timeout")
		}
	case <-time.After(timeout):
		return errors.New("enqueue timeout")
	}
}

// EnqueueMessage 将消息尽力推入连接发送队列，不等待写入执行，仅等待入队成功或超时。
func EnqueueMessage(userID int64, timeout time.Duration, message interface{}) error {
	zap.L().Debug("EnqueueMessage", zap.Int64("userID", userID), zap.Any("message", message))
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	req := sendRequest{msg: message, resp: nil}
	zap.L().Debug("EnqueueMessage prepared request", zap.Int64("userID", userID), zap.Any("request", req))
	select {
	case holder.sendCh <- req:
		incPending(1)
		return nil
	case <-time.After(timeout):
		return errors.New("enqueue timeout")
	}
}

// GetConnectionHolder 返回 holder，用于在调用方检查/访问 Conn
func GetConnectionHolder(userID int64) (*ConnectionHolder, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*ConnectionHolder), true
}

func RegisterConnection(userID int64, conn *websocket.Conn) {
	// determine send channel buffer size from config, fallback to 128
	buf := 128
	if config.Conf != nil && config.Conf.GatewayDispatcherConfig != nil && config.Conf.GatewayDispatcherConfig.SendChannelSize > 0 {
		buf = config.Conf.GatewayDispatcherConfig.SendChannelSize
	}

	// If there's an existing connection, stop its writer, drain buffered messages, and migrate them to the new sendCh.
	var drained []sendRequest
	if val, ok := connStore.Load(userID); ok {
		old := val.(*ConnectionHolder)
		// signal old writer to stop
		func() {
			defer func() { _ = recover() }()
			close(old.closeCh)
		}()
		// drain any buffered items (non-blocking) into slice
		for {
			select {
			case req, ok := <-old.sendCh:
				if !ok {
					goto drainedDone
				}
				drained = append(drained, req)
			default:
				goto drainedDone
			}
		}
	drainedDone:
		// close old sendCh safely
		func() {
			defer func() { _ = recover() }()
			close(old.sendCh)
		}()
		if old.Conn != nil {
			old.Conn.Close()
		}
		connCountLock.Lock()
		connCount--
		connCountLock.Unlock()
		connStore.Delete(userID)
	}

	// create new holder with capacity to hold drained items plus configured buffer
	newBuf := buf + len(drained)
	holder := &ConnectionHolder{
		Conn:    conn,
		sendCh:  make(chan sendRequest, newBuf),
		closeCh: make(chan struct{}),
	}
	// migrate drained items into new sendCh without changing TotalTaskCount (they were already counted)
	for _, req := range drained {
		holder.sendCh <- req
	}

	connStore.Store(userID, holder)
	connCountLock.Lock()
	connCount++
	connCountLock.Unlock()
	go writerLoop(holder)
}

func RemoveConnection(userID int64) error {
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	// drain pending items and send them back to center
	var drained []sendRequest
	func() {
		defer func() { _ = recover() }()
		// signal writer to stop
		close(holder.closeCh)
		// non-blocking drain
		for {
			select {
			case req, ok := <-holder.sendCh:
				if !ok {
					goto drainedDone
				}
				drained = append(drained, req)
			default:
				goto drainedDone
			}
		}
	drainedDone:
		// close the sendCh
		func() {
			defer func() { _ = recover() }()
			close(holder.sendCh)
		}()
	}()

	// decrement global task count by number drained
	if len(drained) > 0 {
		decPending(int64(len(drained)))
	}

	// send drained tasks back to center server
	for _, req := range drained {
		// attempt to convert message to types.ClientMessage
		if msg, ok := req.msg.(types.ClientMessage); ok {
			if config.Conf != nil {
				pendingTask.DefaultPendingManager.Done(msg.ID)
				if err := centerclient.SendPushBackRequest(config.Conf.CenterConfig, msg, userID); err != nil {
					zap.L().Error("SendPushBackRequest failed during RemoveConnection", zap.Int64("userID", userID), zap.Error(err))
				}
			}
		} else {
			zap.L().Warn("skipping pushback for non-client message", zap.Any("msg", req.msg))
		}
	}

	if holder.Conn != nil {
		holder.Conn.Close()
	}
	connStore.Delete(userID)
	connCountLock.Lock()
	connCount--
	connCountLock.Unlock()
	return nil
}

func GetConnection(userID int64) (*websocket.Conn, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, true
}

// compatibility: keep these helpers but note they are no-ops for lock-based access
func GetConnectionLock(userID int64) (*sync.Mutex, bool) {
	return nil, false
}

func GetConnectionAndLock(userID int64) (*websocket.Conn, *sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, nil, true
}

func GetConnectionCount() int {
	connCountLock.Lock()
	defer connCountLock.Unlock()
	return connCount
}
