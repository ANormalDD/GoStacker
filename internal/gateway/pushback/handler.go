package pushback

import (
	"GoStacker/pkg/db/redis"
	"encoding/json"
	"strconv"
	"go.uber.org/zap"
)

/*
msg:
	{
		"type":    "pushback",
		"message":
		{
			"target_id": int64,
			"forward_req": ClientMessage,
		},
		"ts":      time.Now().Unix(),
	}
*/
func PushBackHandler(msg map[string]interface{}) error {
	zap.L().Debug("pushback: handler invoked", zap.Any("msg", msg))
	// push msg to offline redis queue
	messageField, ok := msg["message"]
	if !ok {
		zap.L().Error("pushback: missing message field")
		return nil
	}
	messageMap, ok := messageField.(map[string]interface{})
	if !ok {
		zap.L().Error("pushback: message field is not a map")
		return nil
	}
	targetID, ok := messageMap["target_id"].(int64)
	if !ok {
		zap.L().Error("pushback: target_id field is missing or not a number")
		return nil
	}
	forwardReq, ok := messageMap["forward_req"]
	if !ok {
		zap.L().Error("pushback: forward_req field is missing")
		return nil
	}
	marshaledMsg, err := json.Marshal(forwardReq)
	if err != nil {
		zap.L().Error("pushback: failed to marshal forward_req", zap.Error(err), zap.Any("forward_req", forwardReq))
		return nil
	}
	err = redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(targetID, 10), marshaledMsg)
	if err != nil {
		zap.L().Error("pushback: failed to RPush offline message", zap.Int64("target_id", targetID), zap.Error(err))
	}
	return nil
}
