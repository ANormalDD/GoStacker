package push

import (
	"fmt"
)

func PushViaWebSocket(userID int64, message ClientMessage) error {
	conn, exists := GetConnection(userID)
	if !exists {
		return fmt.Errorf("no active WebSocket connection for user %d", userID)
	}
	lock, _ := GetConnectionLock(userID)
	lock.Lock()
	defer lock.Unlock()
	err := conn.WriteJSON(message)
	return err
}
