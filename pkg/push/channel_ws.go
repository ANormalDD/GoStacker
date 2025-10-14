package push

import (
	"fmt"
)

func pushViaWebSocket(userID int64, message PushMessage) error {
	conn, exists := GetConnection(userID)
	if !exists {
		return fmt.Errorf("no active WebSocket connection for user %d", userID)
	}
	err := conn.WriteJSON(message)
	return err
}
