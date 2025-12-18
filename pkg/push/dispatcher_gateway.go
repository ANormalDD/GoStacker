package push

import (
	"GoStacker/internal/send/gateway/userConn"
	gw "GoStacker/internal/send/gateway/ws"
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var gatewayQueue chan PushMessage

// StartGatewayDispatcher 启动 gateway dispatcher 的 worker pool。
// - workers: worker 数量；若传入 0，则调用方应传入 runtime.NumCPU() 或其他值（main 中处理）
// - queueSize: 队列缓冲大小；若为 0 则默认 1024。
func StartGatewayDispatcher(workers int, queueSize int) {
	if queueSize <= 0 {
		queueSize = 1024
	}
	gatewayQueue = make(chan PushMessage, queueSize)
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go gatewayWorker()
	}
	zap.L().Info("gateway dispatcher started", zap.Int("workers", workers), zap.Int("queueSize", queueSize))
}

// Dispatch_gateway 将 PushMessage 推入 gateway dispatcher 队列；调用方应先在 main 中通过 StartGatewayDispatcher 启动。
func Dispatch_gateway_former(msg PushMessage) error {
	zap.L().Debug("Dispatching message to gateway", zap.Any("msg", msg))
	if gatewayQueue == nil {
		zap.L().Error("Dispatch_gateway: gateway dispatcher not started")
		return errors.New("gateway dispatcher not started")
	}

	// try enqueue, with short timeout to avoid blocking caller
	select {
	case gatewayQueue <- msg:
		return nil
	case <-time.After(200 * time.Millisecond):
		zap.L().Warn("gateway dispatch queue full, dropping message", zap.Any("msg", msg))
		return nil
	}
}

func Dispatch_gateway(msg PushMessage) error {
	// 把任务分割成多个子任务，然后推送到队列，分割方法是每100个用户ID为一组
	batchSize := 100
	for i := 0; i < len(msg.TargetIDs); i += batchSize {
		end := i + batchSize
		if end > len(msg.TargetIDs) {
			end = len(msg.TargetIDs)
		}
		subMsg := PushMessage{
			ID:        msg.ID,
			Type:      msg.Type,
			RoomID:    msg.RoomID,
			SenderID:  msg.SenderID,
			TargetIDs: msg.TargetIDs[i:end],
			Payload:   msg.Payload,
		}
		// try enqueue, with short timeout to avoid blocking caller
		select {
		case gatewayQueue <- subMsg:
			// enqueued
		case <-time.After(200 * time.Millisecond):
			zap.L().Warn("gateway dispatch queue full, dropping sub-message", zap.Any("msg", subMsg))
			// continue to next batch
		}
	}
	return nil
}

func PushSingleViaGateway(userID int64, msg ClientMessage) error {
	gid, ok := userConn.GetGatewayIDByUserID(userID)
	if !ok {
		return errors.New("no gateway found for user")
	}
	gwMsg := PushMessage{
		ID:        msg.ID,
		Type:      msg.Type,
		RoomID:    msg.RoomID,
		SenderID:  msg.SenderID,
		TargetIDs: []int64{userID},
		Payload:   msg.Payload,
	}
	// send to gateway via internal ws manager; use 10s write timeout
	if err := gw.SendToGatewayWithRedisStream(gid, gwMsg); err != nil {
		return err
	}
	return nil
}

func gatewayWorker() {
	for msg := range gatewayQueue {
		// group target ids by gateway id
		groups := make(map[string][]int64)
		for _, uid := range msg.TargetIDs {
			if gid, ok := userConn.GetGatewayIDByUserID(uid); ok {
				groups[gid] = append(groups[gid], uid)
			} else {
				// no gateway found -> push to offline redis queue
				clientMsg := ClientMessage{
					ID:       msg.ID,
					Type:     msg.Type,
					RoomID:   msg.RoomID,
					SenderID: msg.SenderID,
					Payload:  msg.Payload,
				}
				marshaledMsg, err := json.Marshal(clientMsg)
				if err != nil {
					zap.L().Error("gateway dispatch: marshal offline client msg failed", zap.Error(err), zap.Int64("user", uid))
					continue
				}
				InsertOfflineQueue(uid, string(marshaledMsg))
			}
		}

		// for each gateway, create a PushMessage and send to gateway
		for gid, uids := range groups {
			gwMsg := PushMessage{
				ID:        msg.ID,
				Type:      msg.Type,
				RoomID:    msg.RoomID,
				SenderID:  msg.SenderID,
				TargetIDs: uids,
				Payload:   msg.Payload,
			}

			// send to gateway via internal ws manager; use 10s write timeout
			if err := gw.SendToGatewayWithRedisStream(gid, gwMsg); err != nil {
				if err == gw.ErrNoConn {
					zap.L().Info("redis down, push to offline queues", zap.String("gateway", gid))
					// push all messages for these user ids to offline redis
					for _, uid := range uids {
						clientMsg := ClientMessage{
							ID:       msg.ID,
							Type:     msg.Type,
							RoomID:   msg.RoomID,
							SenderID: msg.SenderID,
							Payload:  msg.Payload,
						}
						marshaledMsg, err := json.Marshal(clientMsg)
						if err != nil {
							zap.L().Error("gateway dispatch: marshal offline client msg failed", zap.Error(err), zap.Int64("user", uid))
							continue
						}
						if err := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(uid, 10), marshaledMsg); err != nil {
							zap.L().Error("gateway dispatch: rpush offline failed", zap.Error(err), zap.Int64("user", uid))
						}
					}
				} else {
					zap.L().Error("gateway dispatch: send to gateway failed", zap.Error(err), zap.String("gateway", gid))
				}
			}
		}
	}
}
