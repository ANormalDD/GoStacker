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
	var tryTimes int64 = 0
	for {
		tryTimes++
		if tryTimes > lenOfList*3 {
			break
		}
		msg, err := redis.Rdb.LPop("offline:push:" + strconv.FormatInt(userID, 10)).Result()
		if err != nil {
			if err == Redis.Nil {
				// No more messages
				break
			}
			// Log the error and break to avoid infinite loop
			zap.L().Error("Failed to LPop offline message", zap.Int64("userID", userID), zap.Error(err))
			break
		}
		var clientMsg ClientMessage
		if err := json.Unmarshal([]byte(msg), &clientMsg); err != nil {
			continue
		}
		err = PushViaWebSocket(userID, clientMsg)
		if err != nil {
			raw, _ := json.Marshal(clientMsg)
			err2 := redis.Rdb.LPush("offline:push:"+strconv.FormatInt(userID, 10), raw).Err()
			if err2 != nil {
				zap.L().Error("Failed to LPush offline message back", zap.Int64("userID", userID), zap.Error(err2))
				//wait before retry
				time.Sleep(100 * time.Millisecond)
				err2 = redis.Rdb.LPush("offline:push:"+strconv.FormatInt(userID, 10), raw).Err()
				if err2 != nil {
					zap.L().Error("Failed to LPush offline message back again, giving up", zap.Int64("userID", userID), zap.Error(err2))
				}
			}
			if err == ErrNoConn {
				break
			}
		}
	}
}
