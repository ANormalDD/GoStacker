package push

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var PushTaskChan chan PushTask = make(chan PushTask, 1000)

func StartPushDispatcher(Conf *config.DispatcherConfig) {
	PushTaskChan = make(chan PushTask, Conf.TaskQueueSize)
	for i := 0; i < Conf.WorkerCount; i++ {
		go func() {
			for task := range PushTaskChan {
				err := PushViaWSWithRetry(task.UserID, 2, 10*time.Second, task.Msg)
				if err != nil {
					// If WebSocket push fails, fallback to Redis push

					err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(task.UserID, 10), task.MarshaledMsg)
					if err2 != nil {
						zap.L().Error("Failed to RPush offline message. Message missed", zap.Int64("userID", task.UserID), zap.Error(err2))
					}
					if err != ErrNoConn {
						RemoveConnection(task.UserID)
					}
				}
			}
		}()
	}
}

func Dispatch(msg PushMessage) error {

	clientMsg := ClientMessage{
		ID:       msg.ID,
		Type:     msg.Type,
		RoomID:   msg.RoomID,
		SenderID: msg.SenderID,
		Payload:  msg.Payload,
	}

	marshaledMsg, err := json.Marshal(clientMsg)
	if err != nil {
		zap.L().Error("Failed to marshal client message", zap.Error(err), zap.Any("message", clientMsg))
		return err
	}
	for _, uid := range msg.TargetIDs {
		PushTaskChan <- PushTask{
			UserID:       uid,
			Msg:          clientMsg,
			MarshaledMsg: marshaledMsg,
		}
	}
	return nil
}

func InitDispatcher(Conf *config.DispatcherConfig) {
	StartPushDispatcher(Conf)
}
