package push

import (
	"sync"

	"github.com/gorilla/websocket"
)

var connMap sync.Map // key: int64, value: *websocket.Conn

func RegisterConnection(userID int64, conn *websocket.Conn) {
	connMap.Store(userID, conn)
}

func RemoveConnection(userID int64) {
	connMap.Delete(userID)
}

func GetConnection(userID int64) (*websocket.Conn, bool) {
	val, ok := connMap.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*websocket.Conn), true
}
