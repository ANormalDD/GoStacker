package push

import (
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strconv"
)

// 登录时将存在redis中的离线消息推送给用户，然后删除redis中的离线消息
func PushOfflineMessages(userID int64) {
	for {
		msg, err := redis.Rdb.LPop("offline:push:" + strconv.FormatInt(userID, 10)).Result()
		if err != nil {
			//repush to redis

			break
		}
		var clientMsg ClientMessage
		if err := json.Unmarshal([]byte(msg), &clientMsg); err != nil {
			continue
		}
		err = PushViaWebSocket(userID, clientMsg)
		if err != nil {
			// If WebSocket push fails, fallback to Redis push
			raw, _ := json.Marshal(clientMsg)
			redis.Rdb.LPush("offline:push:"+strconv.FormatInt(userID, 10), raw)
		}
	}
}
