package push

import (
	"fmt"
)

func PushViaWebSocket(userID int64, message ClientMessage) error {
	conn, exists := GetConnection(userID)
	if !exists {
		return fmt.Errorf("no active WebSocket connection for user %d", userID)
	}
	err := conn.WriteJSON(message)
	return err
}
