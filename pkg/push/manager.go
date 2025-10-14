package push

import (
	"github.com/gorilla/websocket"
	"sync"
)

var connMap = make(map[int64]*websocket.Conn)
var connLock sync.RWMutex

func RegisterConnection(userID int64, conn *websocket.Conn) {
	connLock.Lock()
	defer connLock.Unlock()
	connMap[userID] = conn
}

func RemoveConnection(userID int64) {
	connLock.Lock()
	defer connLock.Unlock()
	delete(connMap, userID)
}

func GetConnection(userID int64) (*websocket.Conn, bool) {
	connLock.RLock()
	defer connLock.RUnlock()
	conn, exists := connMap[userID]
	return conn, exists
}