package push

import (
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// TODO1: 优化点，创建一个goroutine池，限制最大并发数，防止过多的goroutine导致系统资源耗尽
// TODO2: 优化点，增加重试机制，防止偶发的网络问题导致消息发送失败
// TODO3: 优化点，增加消息发送的超时机制，防止某些用户长时间不响应导致阻塞
func Dispatch(msg PushMessage) error {

	clientMsg := ClientMessage{
		ID:       -1,
		Type:     msg.Type,
		RoomID:   msg.RoomID,
		SenderID: msg.SenderID,
		Payload:  msg.Payload,
	}

	_, err := json.Marshal(clientMsg)

	if err != nil {
		zap.L().Error("Failed to marshal client message", zap.Error(err), zap.Any("message", clientMsg))
		return err
	}

	for _, uid := range msg.TargetIDs {
		clientMsg.ID = msg.ID
		go func(uid int64, clientMsg ClientMessage) {
			err := PushViaWSWithRetry(uid, 2, 10*time.Second, clientMsg)
			if err != nil {
				// If WebSocket push fails, fallback to Redis push
				raw, _ := json.Marshal(clientMsg)
				err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(uid, 10), raw)
				if err2 != nil {
					zap.L().Error("Failed to RPush offline message. Message missed", zap.Int64("userID", uid), zap.Error(err2))
				}
				if err != ErrNoConn {
					RemoveConnection(uid)
				}
			}
		}(uid, clientMsg)
		continue
	}

	return nil
}
