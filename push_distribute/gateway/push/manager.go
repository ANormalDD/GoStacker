package push

import (
	"errors"
	"sync"
	"time"

	"GoStacker/pkg/db/redis"
	"GoStacker/push_distribute/gateway/centerclient"
	"strconv"

	"github.com/gorilla/websocket"
)

// 单一存储，value: *ConnectionHolder
var connStore sync.Map // key: int64, value: *ConnectionHolder

var ErrNoConn = errors.New("no connection for user")

type ConnectionHolder struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

// WriteJSONSafe 写操作封装在 holder 内，确保在持锁后检查 conn 并设置写超时
func WriteJSONSafe(holder *ConnectionHolder, timeout time.Duration, message interface{}) error {
	holder.Mu.Lock()
	defer holder.Mu.Unlock()
	if holder.Conn == nil {
		return ErrNoConn
	}
	holder.Conn.SetWriteDeadline(time.Now().Add(timeout))
	return holder.Conn.WriteJSON(message)
}

// GetConnectionHolder 返回 holder，用于在调用方持锁/访问 Conn
func GetConnectionHolder(userID int64) (*ConnectionHolder, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*ConnectionHolder), true
}

func RegisterConnection(userID int64, conn *websocket.Conn) {
	// 如果已有旧连接，先关闭并删除
	if val, ok := connStore.Load(userID); ok {
		holder := val.(*ConnectionHolder)
		holder.Mu.Lock()
		if holder.Conn != nil {
			holder.Conn.Close()
		}
		connStore.Delete(userID)
		holder.Conn = nil
		holder.Mu.Unlock()
	}
	holder := &ConnectionHolder{Conn: conn}
	connStore.Store(userID, holder)
	// notify central server about this registration (best-effort)
	go func() {
		_ = centerclient.RegisterUser(userID)
	}()
}

func RemoveConnection(userID int64) error {
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	holder.Mu.Lock()
	if holder.Conn != nil {
		holder.Conn.Close()
	}
	connStore.Delete(userID)
	holder.Conn = nil
	holder.Mu.Unlock()

	// best-effort notify central server
	go func() {
		_ = centerclient.UnregisterUser(userID)
	}()

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

func GetConnectionLock(userID int64) (*sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	holder := val.(*ConnectionHolder)
	return &holder.Mu, true
}

func GetConnectionAndLock(userID int64) (*websocket.Conn, *sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, &holder.Mu, true
}

// HandlePanic drains in-memory push queue to Redis wait/offline queues and unregisters connected users
func HandlePanic() {
	// drain PushTaskChan into a slice for handoff
	var tasks []map[string]interface{}
	for {
		select {
		case task := <-PushTaskChan:
			tasks = append(tasks, map[string]interface{}{"user_id": task.UserID, "message": string(task.MarshaledMsg)})
		default:
			goto afterCollect
		}
	}
afterCollect:
	if len(tasks) > 0 {
		// try sending pages gzipped
		// convert tasks to []map[string]interface{} already
		if err := centerclient.HandoffPaged(tasks, 100); err != nil {
			// fallback: write tasks to local redis wait queues
			for _, t := range tasks {
				uidF := t["user_id"].(float64)
				uid := int64(uidF)
				raw := []byte(t["message"].(string))
				uidStr := strconv.FormatInt(uid, 10)
				_ = redis.SAddWithRetry(2, "wait:push:set", uidStr)
				_ = redis.RPushWithRetry(2, "wait:push:"+uidStr, raw)
			}
		}
	}

	// iterate over current connections, close and unregister
	connStore.Range(func(key, value interface{}) bool {
		uid := key.(int64)
		holder := value.(*ConnectionHolder)
		holder.Mu.Lock()
		if holder.Conn != nil {
			holder.Conn.Close()
		}
		holder.Conn = nil
		holder.Mu.Unlock()
		connStore.Delete(uid)
		// notify central server to remove mapping (best-effort)
		go func(id int64) {
			_ = centerclient.UnregisterUser(id)
		}(uid)
		return true
	})
}
