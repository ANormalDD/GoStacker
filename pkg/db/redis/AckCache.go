package redis

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var (
	msgCh       chan string
	groupName   string
	streamName  string
	interval    time.Duration
	cacheCancel context.CancelFunc
	cacheCtx    context.Context
)

func cacheFlushWorker() {
	const maxBatchSize = 1024
	batch := make([]string, 0, maxBatchSize)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cacheCtx.Done():
			// flush remaining
			if len(batch) > 0 {
				XAckWithRetry(3, streamName, groupName, batch...)
			}
			zap.L().Info("AckCache flush worker exiting due to context cancellation")
			return
		case id := <-msgCh:
			batch = append(batch, id)
			if len(batch) >= maxBatchSize {
				XAckWithRetry(3, streamName, groupName, batch...)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				XAckWithRetry(3, streamName, groupName, batch...)
				batch = batch[:0]
			}
		}
	}
}

func waitforAckShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("AckCache shutdown signal received, canceling cache context")
	if cacheCancel != nil {
		cacheCancel()
	}
	zap.L().Info("AckCache flush worker has been signaled to exit")
}


func InsertAckCache(messageIDs ...string) {
	for _, id := range messageIDs {
		select {
		case msgCh <- id:
		default:
			zap.L().Warn("AckCache buffer full, dropping message id", zap.String("id", id))
		}
	}
}

func InitAckCache(StreamName string, GroupName string, Interval time.Duration) {
	cacheCtx, cacheCancel = context.WithCancel(context.Background())
	streamName = StreamName
	groupName = GroupName
	interval = Interval
	msgCh = make(chan string, 10000)
	go cacheFlushWorker()
	go waitforAckShutdown()
}
