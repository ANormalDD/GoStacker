package push

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/monitor"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var dispatcherCtx context.Context
var dispatcherCancel context.CancelFunc
var pushWSMonitor *monitor.Monitor

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
				InsertOfflineQueue(uid, string(marshaledMsg))
				continue
			}
			InsertWaitQueue(uid, string(marshaledMsg))
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
	pushWSMonitor = monitor.NewMonitor("push_ws", 1000, 10000, 60000)
	go waitForShutdown()
	go ListeningWaitQueue()
}
