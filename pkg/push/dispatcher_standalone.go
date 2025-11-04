package push

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/redis"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var dispatcherCtx context.Context
var dispatcherCancel context.CancelFunc

// no worker pool anymore; dispatcher uses per-connection channels

func Dispatch_StandAlone(msg PushMessage) error {

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
		// try to enqueue directly to the user's send channel; small timeout to avoid blocking
		if err := EnqueueMessage(uid, 100*time.Millisecond, clientMsg); err != nil {
			// fallback: 推送到redis等待队列,并记录
			if err == ErrNoConn {
				zap.L().Info("EnqueueMessage failed due to no connection, pushing to offline queue", zap.Int64("userID", uid))
				redisErr := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(uid, 10), marshaledMsg)
				if redisErr != nil {
					zap.L().Error("EnqueueMessage failed due to no connection and RPush offline message failed. Message missed", zap.Int64("userID", uid), zap.Error(redisErr))
				}
			}
			err2 := redis.SAddWithRetry(2, "wait:push:set", strconv.FormatInt(uid, 10))
			if err2 != nil {
				zap.L().Error("EnqueueMessage failed and SAdd wait push userID failed. Message missed", zap.Int64("userID", uid), zap.Error(err2), zap.Error(err))
				continue
			}
			err2 = redis.RPushWithRetry(2, "wait:push:"+strconv.FormatInt(uid, 10), marshaledMsg)
			if err2 != nil {
				zap.L().Error("EnqueueMessage failed and RPush wait push message failed. Message missed", zap.Int64("userID", uid), zap.Error(err2), zap.Error(err))
			}
		}
	}
	return nil
}

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("Push dispatcher shutdown signal received, canceling dispatcher context")
	if dispatcherCancel != nil {
		dispatcherCancel()
	}
	// ListeningWaitQueue and other goroutines should observe dispatcherCtx.Done() and exit.
	zap.L().Info("Push dispatcher shutdown initiated; background listeners will exit")
}

func InitDispatcher(Conf *config.DispatcherConfig) {
	dispatcherCtx, dispatcherCancel = context.WithCancel(context.Background())
	go waitForShutdown()
	go ListeningWaitQueue()
}
