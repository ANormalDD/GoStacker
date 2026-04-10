package msgflusher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"GoStacker/pkg/config"
	rdb "GoStacker/pkg/db/redis"
	"GoStacker/pkg/registry_client"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type PushMessage struct {
	ID        int64       `json:"id"`
	Type      string      `json:"type"`
	RoomID    int64       `json:"room_id"`
	SenderID  int64       `json:"sender_id"`
	TargetIDs []int64     `json:"target_ids"`
	Payload   interface{} `json:"payload"`
}

type ClientMessage struct {
	ID       int64       `json:"id"`
	Type     string      `json:"type"`
	RoomID   int64       `json:"room_id"`
	SenderID int64       `json:"sender_id"`
	Payload  interface{} `json:"payload"`
}

type Reclaimer struct {
	gatewayClient *registry_client.GatewayClient
	sendClient    *registry_client.SendClient
	httpClient    *http.Client

	consumerName string
	streamSuffix string
	groupSuffix  string
	claimIdle    time.Duration
	interval     time.Duration
	batchSize    int64
	rand         *rand.Rand
}

func NewReclaimer(appCfg *config.AppConfig) (*Reclaimer, error) {
	if appCfg == nil || appCfg.RegistryConfig == nil || appCfg.RegistryConfig.URL == "" {
		return nil, fmt.Errorf("registry url is required for msgflusher")
	}

	interval := 5 * time.Second
	batchSize := int64(100)
	claimIdle := 30 * time.Second
	streamSuffix := "_stream"
	groupSuffix := "_group"

	if appCfg.PendingMsgFlusherConfig != nil {
		if appCfg.PendingMsgFlusherConfig.Interval > 0 {
			interval = time.Duration(appCfg.PendingMsgFlusherConfig.Interval) * time.Second
		}
		if appCfg.PendingMsgFlusherConfig.BatchSize > 0 {
			batchSize = int64(appCfg.PendingMsgFlusherConfig.BatchSize)
		}
		if appCfg.PendingMsgFlusherConfig.ClaimIdleSeconds > 0 {
			claimIdle = time.Duration(appCfg.PendingMsgFlusherConfig.ClaimIdleSeconds) * time.Second
		}
		if appCfg.PendingMsgFlusherConfig.StreamSuffix != "" {
			streamSuffix = appCfg.PendingMsgFlusherConfig.StreamSuffix
		}
		if appCfg.PendingMsgFlusherConfig.GroupSuffix != "" {
			groupSuffix = appCfg.PendingMsgFlusherConfig.GroupSuffix
		}
	}

	consumerName := fmt.Sprintf("msgflusher-%s-%d", appCfg.Name, appCfg.MachineID)
	if appCfg.Name == "" {
		consumerName = fmt.Sprintf("msgflusher-%d", time.Now().Unix())
	}

	return &Reclaimer{
		gatewayClient: registry_client.NewGatewayClient(appCfg.RegistryConfig.URL, consumerName),
		sendClient:    registry_client.NewSendClient(appCfg.RegistryConfig.URL, consumerName),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		consumerName: consumerName,
		streamSuffix: streamSuffix,
		groupSuffix:  groupSuffix,
		claimIdle:    claimIdle,
		interval:     interval,
		batchSize:    batchSize,
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (r *Reclaimer) Run(ctx context.Context) {
	r.runOnce(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("msgflusher reclaimer stopped")
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Reclaimer) runOnce(ctx context.Context) {
	gateways, err := r.gatewayClient.ListGateways()
	if err != nil {
		zap.L().Error("msgflusher failed to list gateways", zap.Error(err))
		return
	}
	if len(gateways) == 0 {
		return
	}

	sendInstances, err := r.sendClient.ListSendInstances()
	if err != nil {
		zap.L().Error("msgflusher failed to list send instances", zap.Error(err))
		return
	}
	if len(sendInstances) == 0 {
		zap.L().Warn("msgflusher found no available send instances")
		return
	}

	for _, gw := range gateways {
		stream := gw.GatewayID + r.streamSuffix
		group := gw.GatewayID + r.groupSuffix
		if err := r.reclaimStream(ctx, stream, group, sendInstances); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			zap.L().Warn("msgflusher reclaim stream failed",
				zap.String("gateway_id", gw.GatewayID),
				zap.String("stream", stream),
				zap.String("group", group),
				zap.Error(err))
		}
	}
}

func (r *Reclaimer) reclaimStream(ctx context.Context, stream string, group string, sendInstances []registry_client.SendInstanceInfo) error {
	startID := "0-0"
	for {
		xmsgs, nextStart, err := rdb.XAutoClaimWithContext(ctx, stream, group, r.consumerName, r.claimIdle, startID, r.batchSize)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return context.Canceled
			}
			if err == goredis.Nil {
				return nil
			}
			return err
		}

		for _, xmsg := range xmsgs {
			msgData, err := xMessageToPushMessage(xmsg)
			if err != nil {
				zap.L().Error("msgflusher decode pending message failed",
					zap.String("stream", stream),
					zap.String("group", group),
					zap.String("stream_msg_id", xmsg.ID),
					zap.Error(err))
				continue
			}

			ok := true
			for _, uid := range msgData.TargetIDs {
				forwardReq := ClientMessage{
					ID:       msgData.ID,
					Type:     msgData.Type,
					RoomID:   msgData.RoomID,
					SenderID: msgData.SenderID,
					Payload:  msgData.Payload,
				}
				if err := r.pushbackToSend(ctx, uid, forwardReq, sendInstances); err != nil {
					ok = false
					zap.L().Error("msgflusher pushback failed",
						zap.String("stream", stream),
						zap.String("group", group),
						zap.String("stream_msg_id", xmsg.ID),
						zap.Int64("target_id", uid),
						zap.Error(err))
					break
				}
			}

			if !ok {
				continue
			}

			if err := rdb.XAckWithRetry(2, stream, group, xmsg.ID); err != nil {
				zap.L().Error("msgflusher ack reclaimed message failed",
					zap.String("stream", stream),
					zap.String("group", group),
					zap.String("stream_msg_id", xmsg.ID),
					zap.Error(err))
				continue
			}
		}

		if len(xmsgs) == 0 || nextStart == "" || nextStart == "0-0" || nextStart == startID {
			return nil
		}
		startID = nextStart
	}
}

func (r *Reclaimer) pushbackToSend(ctx context.Context, targetID int64, forwardReq ClientMessage, sendInstances []registry_client.SendInstanceInfo) error {
	if len(sendInstances) == 0 {
		return fmt.Errorf("no send instances available")
	}

	body := map[string]interface{}{
		"target_id":   targetID,
		"forward_req": forwardReq,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	indices := r.rand.Perm(len(sendInstances))
	var lastErr error
	for _, idx := range indices {
		inst := sendInstances[idx]
		url := fmt.Sprintf("http://%s:%d/internal/pushback", inst.Address, inst.Port)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("send pushback status %d", resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all send instances failed for pushback")
	}
	return lastErr
}

func xMessageToPushMessage(m goredis.XMessage) (PushMessage, error) {
	var msgData PushMessage
	raw, ok := m.Values["data"]
	if !ok {
		return msgData, fmt.Errorf("stream message missing data field")
	}

	var payload []byte
	switch v := raw.(type) {
	case string:
		payload = []byte(v)
	case []byte:
		payload = v
	default:
		return msgData, fmt.Errorf("unsupported data type: %T", raw)
	}

	if err := json.Unmarshal(payload, &msgData); err != nil {
		return msgData, err
	}
	return msgData, nil
}
