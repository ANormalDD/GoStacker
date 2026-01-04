package push

import (
	"GoStacker/internal/send/route"
	"GoStacker/pkg/db/redis"
	"GoStacker/pkg/pendingTask"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

var gatewayQueue chan PushMessage

var msg2sender sync.Map // key: msgID, value: senderID

func Dispatch_gateway(msg PushMessage) error {
	// 把任务分割成多个子任务，然后推送到队列，分割方法是每100个用户ID为一组
	msg2sender.Store(msg.ID, msg.SenderID)
	batchSize := 100
	pendingTask.DefaultPendingManager.Init(msg.ID, int32(len(msg.TargetIDs)))
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

// 当所有pending任务完成后调用此函数发送ACK给发送方，ID为msgID，type为ack
func SendACKToSender(msgID int64) {
	ackMsg := ClientMessage{
		ID:   msgID,
		Type: "ack",
	}
	senderID, ok := msg2sender.LoadAndDelete(msgID)
	if !ok {
		zap.L().Warn("sendACKToSender: no senderID found for msgID", zap.Int64("msgID", msgID))
		return
	}
	err := PushSingleViaGateway(senderID.(int64), ackMsg)
	if err != nil {
		zap.L().Error("sendACKToSender: failed to send ACK to sender", zap.Int64("msgID", msgID), zap.Int64("senderID", senderID.(int64)), zap.Error(err))
	} else {
		zap.L().Info("sendACKToSender: ACK sent to sender", zap.Int64("msgID", msgID), zap.Int64("senderID", senderID.(int64)))
	}
}

func PushSingleViaGateway(userID int64, msg ClientMessage) error {
	// Query route from cache/registry
	gatewayID, _, err := route.GetUserGateway(userID)
	if err != nil {
		if err == route.ErrUserOffline {
			zap.L().Debug("User offline, skipping push", zap.Int64("user_id", userID))
			return err
		}
		zap.L().Error("Failed to get gateway for user", zap.Int64("user_id", userID), zap.Error(err))
		return err
	}

	gwMsg := PushMessage{
		ID:        msg.ID,
		Type:      msg.Type,
		RoomID:    msg.RoomID,
		SenderID:  msg.SenderID,
		TargetIDs: []int64{userID},
		Payload:   msg.Payload,
	}

	// Send to gateway via Redis Stream
	if err := sendToGatewayWithRedisStream(gatewayID, gwMsg); err != nil {
		return err
	}
	return nil
}

// sendToGatewayWithRedisStream sends message to gateway via Redis Stream
func sendToGatewayWithRedisStream(gatewayID string, message interface{}) error {
	zap.L().Debug("Sending message to gateway via Redis Stream",
		zap.String("gateway_id", gatewayID),
		zap.Any("message", message))

	data, err := json.Marshal(message)
	if err != nil {
		zap.L().Error("Failed to marshal message for Redis Stream", zap.Error(err))
		return err
	}

	streamName := fmt.Sprintf("%s_stream", gatewayID)
	err = redis.XAddWithRetry(2, streamName, map[string]interface{}{
		"data": data,
	})
	zap.L().Debug("Redis stream enqueue", zap.String("stream", streamName))
	if err != nil {
		zap.L().Error("Failed to add message to Redis Stream", zap.Error(err))
		return err
	}
	return nil
}

func gatewayWorker() {
	for msg := range gatewayQueue {
		// Batch query routes for all target users
		routeMap, err := route.BatchGetUserGateways(msg.TargetIDs)
		if err != nil {
			zap.L().Error("Failed to batch query routes", zap.Error(err))
			// Continue with empty map, will push all to offline
			routeMap = make(map[int64]*route.RouteInfo)
		}

		// Group target ids by gateway id
		groups := make(map[string][]int64)
		for _, uid := range msg.TargetIDs {
			routeInfo, found := routeMap[uid]
			if found {
				groups[routeInfo.GatewayID] = append(groups[routeInfo.GatewayID], uid)
			} else {
				// No gateway found -> push to offline redis queue
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

		// For each gateway, create a PushMessage and send to gateway via Redis Stream
		for gid, uids := range groups {
			gwMsg := PushMessage{
				ID:        msg.ID,
				Type:      msg.Type,
				RoomID:    msg.RoomID,
				SenderID:  msg.SenderID,
				TargetIDs: uids,
				Payload:   msg.Payload,
			}

			// Send to gateway via Redis Stream
			if err := sendToGatewayWithRedisStream(gid, gwMsg); err != nil {
				zap.L().Error("gateway dispatch: send to gateway failed", zap.Error(err), zap.String("gateway", gid))
				// Push to offline queues as fallback
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
				pendingTask.DefaultPendingManager.Delete(msg.ID)
				msg2sender.Delete(msg.ID)
				continue
			}
			pendingTask.DefaultPendingManager.DoneN(msg.ID, int32(len(uids)))
		}
	}
}

// StartGatewayDispatcher 启动 gateway dispatcher 的 worker pool。
// - workers: worker 数量；若传入 0，则调用方应传入 runtime.NumCPU() 或其他值（main 中处理）
// - queueSize: 队列缓冲大小；若为 0 则默认 1024。
func StartGatewayDispatcher(workers int, queueSize int) {
	pendingTask.DefaultPendingManager.SetDoneFunc(SendACKToSender)
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
