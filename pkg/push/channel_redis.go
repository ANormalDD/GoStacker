package push

import (
	"GoStacker/pkg/db/redis"
	"fmt"

	Redis "github.com/go-redis/redis"
	"go.uber.org/zap"

	"encoding/json"
	"strconv"
)

// 登录时将存在redis中的离线消息推送给用户，然后删除redis中的离线消息
func PushOfflineMessages(userID int64) {
	for {
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
			redis.Rdb.RPush("offline:push:"+strconv.FormatInt(userID, 10), raw)
			if err == fmt.Errorf("no conn for user %d", userID) {
				break
			}
		}
	}
}
