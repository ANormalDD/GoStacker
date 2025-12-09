package push

import (
	"GoStacker/pkg/db/redis"

	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	Redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var waitSet sync.Map

// use when redis is down <int,chan<string>>
var waitMsg sync.Map
var offlineMsg sync.Map

func InsertWaitSet(userID int64) {
	_, ok := waitSet.Load(userID)
	if ok {
		return
	}
	waitSet.Store(userID, struct{}{})
}

func RemoveWaitSet(userID int64) {
	_, ok := waitSet.Load(userID)
	if !ok {
		return
	}
	waitSet.Delete(userID)
}
func InsertOfflineMsg(userID int64, msg string) {
	chInterface, ok := offlineMsg.Load(userID)
	if !ok {
		ch := make(chan string, 100)
		offlineMsg.Store(userID, ch)
		chInterface = ch
	}
	ch := chInterface.(chan string)
	ch <- msg

}
func InsertWaitMsg(userID int64, msg string) {
	chInterface, ok := waitMsg.Load(userID)
	if !ok {
		ch := make(chan string, 100)
		waitMsg.Store(userID, ch)
		chInterface = ch
	}
	ch := chInterface.(chan string)
	ch <- msg
}

func InsertOfflineQueue(userID int64, marshaledMsg string) {
	err := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10), marshaledMsg)
	if err != nil {
		zap.L().Error("Failed to RPush offline push message,try store local", zap.Int64("userID", userID), zap.Error(err))
		InsertOfflineMsg(userID, marshaledMsg)
	}
}

func InsertWaitQueue(userID int64, marshaledMsg string) {
	InsertWaitSet(userID)
	err := redis.RPushWithRetry(2, "wait:push:"+strconv.FormatInt(userID, 10), marshaledMsg)
	if err != nil {
		zap.L().Error("Failed to RPush wait push message,try store local", zap.Int64("userID", userID), zap.Error(err))
		InsertWaitMsg(userID, marshaledMsg)
	}
}

// 登录时将存在redis中的离线消息推送给用户，然后删除redis中的离线消息
func PushOfflineMessages(userID int64) {
	lenOfList, err := redis.Rdb.LLen(context.Background(), "offline:push:"+strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		if err != Redis.Nil {
			zap.L().Error("Failed to get length of offline message list", zap.Int64("userID", userID), zap.Error(err))
			//retry once
			time.Sleep(100 * time.Millisecond)
			lenOfList, err = redis.Rdb.LLen(context.Background(), "offline:push:"+strconv.FormatInt(userID, 10)).Result()
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

func ListeningWaitQueue() {
	for {
		// exit promptly if dispatcher is shutting down
		select {
		case <-dispatcherCtx.Done():
			zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
			return
		default:
		}

		// Iterate local waitSet instead of scanning Redis set.
		anyMember := false
		waitSet.Range(func(k, _ interface{}) bool {
			// check shutdown signal inside the inner loop as well
			select {
			case <-dispatcherCtx.Done():
				zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
				return false
			default:
			}

			var uid int64
			switch v := k.(type) {
			case int64:
				uid = v
			case int:
				uid = int64(v)
			case string:
				parsed, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return true
				}
				uid = parsed
			default:
				return true
			}

			anyMember = true
			uidStr := strconv.FormatInt(uid, 10)

			// pop message from wait queue
			msgStr, err := redis.LPopWithRetry(2, "wait:push:"+uidStr)
			if err != nil {
				if err != Redis.Nil {
					zap.L().Error("Failed to LPop wait push message", zap.Int64("userID", uid), zap.Error(err))
				}
				return true
			}
			var clientMsg ClientMessage
			if err := json.Unmarshal([]byte(msgStr), &clientMsg); err != nil {
				// malformed message, skip
				return true
			}
			// try enqueue directly to the user's send channel
			if err := EnqueueMessage(uid, 100*time.Millisecond, clientMsg); err != nil {
				// push back to wait queue
				err2 := redis.RPushWithRetry(2, "wait:push:"+uidStr, msgStr)
				if err2 != nil {
					zap.L().Error("Failed to RPush wait push message back. Message missed", zap.Int64("userID", uid), zap.Error(err2))
				}
				return false // stop iteration when queue is busy
			}
			// if wait queue is empty remove from local waitSet
			lenOfWaitQueue, err := redis.Rdb.LLen(context.Background(), "wait:push:"+uidStr).Result()
			if err != nil {
				if err != Redis.Nil {
					zap.L().Error("Failed to get length of wait push queue", zap.Int64("userID", uid), zap.Error(err))
					return true
				}
				return true
			}
			if lenOfWaitQueue == 0 {
				RemoveWaitSet(uid)
			}
			return true
		})

		if !anyMember {
			time.Sleep(1000 * time.Millisecond)
			continue
		}

	}
}
