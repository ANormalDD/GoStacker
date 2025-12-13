package loadupdate

import (
	"GoStacker/internal/send/gateway"

	"go.uber.org/zap"
)

func LoadUpdateHandler(msg map[string]interface{}, gatewayID string) {
	zap.L().Debug("loadupdate: handler invoked", zap.Any("msg", msg), zap.String("gateway_id", gatewayID))
	loadVal, ok := msg["load"].(float64)
	if !ok {
		zap.L().Error("loadupdate: missing or invalid load field")
		return
	}
	load := float32(loadVal)
	gateway.DefaultManager.UpdateLoadRate(gatewayID, load)
}
