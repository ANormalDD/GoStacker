// new way to get task from redis
// use redis streams
package push

import (
	"GoStacker/internal/gateway/push/types"
	Redis "GoStacker/pkg/db/redis"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	streamName    = "push_tasks_stream"
	groupName     = "push_tasks_group"
	consumerGroup = "push_tasks_group"
	pullCancel    context.CancelFunc
	pullCtx       context.Context
	isPulling     = false
	taskChan      = make(chan types.PushMessage, 1000)
	batchSize     = 100
	threshold     = int64(10000)
	lastID        = ">"
)

func dispatchWorker() {
	for {
		select {
		case tasks := <-taskChan:
			Dispatch(tasks)
		case <-pullCtx.Done():
			zap.L().Info("Dispatch worker received shutdown signal")
			return
		}
	}
}

func XStream2PushMessage(xstream []redis.XStream) ([]types.PushMessage, error) {
	var out []types.PushMessage
	for _, xs := range xstream {
		for _, m := range xs.Messages {
			var msgData types.PushMessage
			dataBytes, ok := m.Values["data"].(string)
			if !ok {
				fmt.Println("Error converting message data to string")
				continue
			}
			err := json.Unmarshal([]byte(dataBytes), &msgData)
			if err != nil {
				fmt.Println("Error unmarshaling message data:", err)
				continue
			}
			out = append(out, msgData)
		}
	}
	return out, nil
}

func pullLoop() {
	isPulling = true
	defer func() { isPulling = false }()

	for {
		if atomic.LoadInt64(&totalPending) > threshold {
			return
		}

		resultChan := make(chan pullResult, 1)

		go func() {
			xstreams, err := Redis.XReadGroupBlocking(
				streamName, groupName, consumerGroup,
				int64(batchSize), 0, lastID,
			)
			resultChan <- pullResult{xstreams, err}
		}()

		select {
		case <-pullCtx.Done():
			zap.L().Info("pullLoop canceled")
			return

		case res := <-resultChan:
			if res.err != nil {
				zap.L().Error("XReadGroup error", zap.Error(res.err))
				continue
			}

			tasks, err := XStream2PushMessage(res.xstreams)
			if err != nil {
				zap.L().Error("XStream2PushMessage error", zap.Error(err))
				continue
			}

			for _, task := range tasks {
				taskChan <- task
			}
		}
	}
}

type pullResult struct {
	xstreams []redis.XStream
	err      error
}

// blocking pull task from redis stream
func PullTask() {
	if isPulling {
		return
	}
	taskChanCount := len(taskChan)
	if taskChanCount > 800 {
		zap.L().Debug("taskChan is full, skipping pull", zap.Int("taskChanCount", taskChanCount))
		return
	}
	if atomic.LoadInt64(&totalPending) > threshold {
		zap.L().Debug("totalPending exceed threshold, skipping pull", zap.Int64("totalPending", atomic.LoadInt64(&totalPending)), zap.Int64("threshold", threshold))
		return
	}

	go pullLoop()
}

func startDispatchWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go dispatchWorker()
	}
}

func waitForPullShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("Pull Task recive shutdown signal,delivering cancel to pull task dispatcher")
	pullCancel()

}

func InitStreamAndGroup(StreamName string, GroupName string, ConsumerName string, interval time.Duration, thresholdPending int64) {
	pullCtx, pullCancel = context.WithCancel(context.Background())
	streamName = StreamName
	groupName = GroupName
	consumerGroup = ConsumerName
	threshold = thresholdPending
	//init redis stream and group
	Redis.XGroupCreateMkStreamWithRetry(2, StreamName, GroupName, "0")
	Redis.InitAckCache(StreamName, GroupName, interval)
	go waitForPullShutdown()
	startDispatchWorkers(5)
	go pullLoop()
}
