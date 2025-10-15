package push

type PushMessage struct {
	ID        int64
	Type      string
	RoomID    int64
	SenderID  int64
	TargetIDs []int64
	Payload   interface{}
}

type ClientMessage struct {
	ID       int64       `json:"id"`
	Type     string      `json:"type"`
	RoomID   int64       `json:"room_id"`
	SenderID int64       `json:"sender_id"`
	Payload  interface{} `json:"payload"`
}
