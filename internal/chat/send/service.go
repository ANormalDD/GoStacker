package send

import (
	"GoStacker/internal/chat/group"
	"GoStacker/pkg/config"
	"GoStacker/pkg/push"
)

func BroadcastMessage(id int64, roomID int64, senderID int64, content ChatPayload) error {
	members, err := group.QueryRoomMemberIDs(roomID)
	if err != nil {
		return err
	}

	msg := push.PushMessage{
		ID:        id,
		Type:      "chat",
		RoomID:    roomID,
		SenderID:  senderID,
		TargetIDs: members,
		Payload:   content,
	}
	if config.Conf.PushMod == "standalone" {
		return push.Dispatch_StandAlone(msg)
	}
	return push.Dispatch_gateway(msg)
}

func SendMessage(roomID, senderID int64, text ChatPayload) error {
	id, err := InsertMessage(roomID, senderID, text)
	if err != nil {
		return err
	}
	return BroadcastMessage(id, roomID, senderID, text)
}
