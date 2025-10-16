package push

import (
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strconv"
)

func Dispatch(msg PushMessage) error {
	for _, uid := range msg.TargetIDs {
		clientMsg := ClientMessage{
			ID:       msg.ID,
			Type:     msg.Type,
			RoomID:   msg.RoomID,
			SenderID: msg.SenderID,
			Payload:  msg.Payload,
		}
		//创建一个线程调用push
		go func(uid int64, clientMsg ClientMessage) {
			err := PushViaWebSocket(uid, clientMsg)
			if err != nil {
				// If WebSocket push fails, fallback to Redis push
				raw, _ := json.Marshal(clientMsg)
				redis.Rdb.RPush("offline:push:"+strconv.FormatInt(uid, 10), raw)
			}
		}(uid, clientMsg)
		continue
	}
	return nil
}
