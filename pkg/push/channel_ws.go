package push

func PushViaWebSocket(userID int64, message ClientMessage) error {
	conn, exists := GetConnection(userID)
	if !exists {
		return ErrNoConn
	}
	lock, _ := GetConnectionLock(userID)
	lock.Lock()
	defer lock.Unlock()
	err := conn.WriteJSON(message)
	return err
}
