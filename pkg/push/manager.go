package push

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

var connMap sync.Map // key: int64, value: *websocket.Conn
// 创建一个lock map 用来控制对connMap中每个websocket连接的并发读写
var connLocks sync.Map // key: int64, value: *sync.Mutex

var ErrNoConn = errors.New("no connection for user")

func RegisterConnection(userID int64, conn *websocket.Conn) {
	//check old connection
	if oldConn, exists := GetConnection(userID); exists {
		lock, _ := GetConnectionLock(userID)
		lock.Lock()
		oldConn.Close()
		lock.Unlock()
		//释放旧连接和锁的资源
		lock = nil
		oldConn = nil
	}
	connMap.Store(userID, conn)
	connLocks.Store(userID, &sync.Mutex{})
}

func RemoveConnection(userID int64) error {
	lock, exists := GetConnectionLock(userID)
	if !exists {
		return ErrNoConn
	}
	lock.Lock()
	defer lock.Unlock()
	conn, _ := GetConnection(userID)
	conn.Close()
	connMap.Delete(userID)
	connLocks.Delete(userID)
	return nil
}

func GetConnection(userID int64) (*websocket.Conn, bool) {
	val, ok := connMap.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*websocket.Conn), true
}

func GetConnectionLock(userID int64) (*sync.Mutex, bool) {
	lock, ok := connLocks.Load(userID)
	if !ok {
		return nil, false
	}
	return lock.(*sync.Mutex), true
}
