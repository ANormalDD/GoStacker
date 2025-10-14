package push

type PushMessage struct {
	Type     string
	RoomID   int64
	SenderID int64
	TargetID []int64
	Payload  interface{}
}
