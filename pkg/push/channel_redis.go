package push

import (
	"GoStacker/pkg/db/redis"

	"encoding/json"
	"strconv"
	"time"

	Redis "github.com/go-redis/redis"
	"go.uber.org/zap"
)

// 登录时将存在redis中的离线消息推送给用户，然后删除redis中的离线消息
func PushOfflineMessages(userID int64) {
	lenOfList, err := redis.Rdb.LLen("offline:push:" + strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		if err != Redis.Nil {
			zap.L().Error("Failed to get length of offline message list", zap.Int64("userID", userID), zap.Error(err))
			//retry once
			time.Sleep(100 * time.Millisecond)
			lenOfList, err = redis.Rdb.LLen("offline:push:" + strconv.FormatInt(userID, 10)).Result()
			if err != nil {
				zap.L().Error("Failed to get length of offline message list again, giving up", zap.Int64("userID", userID), zap.Error(err))
				return
			}
		} else {
			return
		}
	}
	var messages []ClientMessage
	var msg string
	var batchSize int = 50
	var nowSize int = 0
	for i := int64(0); i < lenOfList; i++ {
		if nowSize >= batchSize {
			// push current batch
			err = PushViaWSWithRetry(userID, 2, 10*time.Second, ClientMessage{
				ID:      -1,
				Type:    "batch",
				Payload: messages,
			})
			if err != nil {
				raw, _ := json.Marshal(messages)
				err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10), raw)
				if err2 != nil {
					zap.L().Error("Failed to RPush offline message back .Message missed", zap.Int64("userID", userID), zap.Error(err2))
				}
				if err != ErrNoConn {
					RemoveConnection(userID)
				}
			}
			// reset batch
			messages = []ClientMessage{}
			nowSize = 0
		}
		msg, err = redis.LPopWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10))
		if err != nil {
			if err == Redis.Nil {
				// No more messages
				break
			}
			// Log the error and break to avoid infinite loop
			zap.L().Error("Failed to LPop offline message", zap.Int64("userID", userID), zap.Error(err))
			//info client that there are offline messages but failed to get them
			err = PushViaWSWithRetry(userID, 2, 10*time.Second, ClientMessage{
				ID:      -1,
				Type:    "info",
				Payload: "You have offline messages but failed to retrieve them.",
			})

			if err != nil && err != ErrNoConn {
				zap.L().Error("Failed to push offline message info", zap.Int64("userID", userID), zap.Error(err))
				RemoveConnection(userID)
			}

			break
		}
		var clientMsg ClientMessage
		if err := json.Unmarshal([]byte(msg), &clientMsg); err != nil {
			continue
		}
		messages = append(messages, clientMsg)
	}
	if len(messages) > 0 {
		err = PushViaWSWithRetry(userID, 2, 10*time.Second, ClientMessage{
			ID:      -1,
			Type:    "batch",
			Payload: messages,
		})
		if err != nil {
			raw, _ := json.Marshal(messages)
			err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10), raw)
			if err2 != nil {
				zap.L().Error("Failed to RPush offline message back .Message missed", zap.Int64("userID", userID), zap.Error(err2))
			}
		}
	}
}
