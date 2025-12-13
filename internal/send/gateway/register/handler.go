package register

import (
	"GoStacker/internal/send/gateway"
	"strconv"

	"go.uber.org/zap"
)

func RegisterGatewayHandler(msg map[string]interface{}, gatewayID string) {
	zap.L().Debug("register: handler invoked", zap.Any("msg", msg), zap.String("gateway_id", gatewayID))
	data, ok := msg["data"].(map[string]interface{})
	if !ok {
		zap.L().Error("register: missing or invalid data field")
		return
	}
	address, ok := data["address"].(string)
	if !ok {
		zap.L().Error("register: missing or invalid address field in data")
		return
	}
	var port int
	if portVal, ok := data["port"]; ok {
		switch v := portVal.(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		default:
			zap.L().Error("register: invalid port type", zap.Any("actual_type", v))
			return
		}
	} else {
		zap.L().Error("register: missing port field in data")
		return
	}
	address = address + ":" + strconv.Itoa(port)
	load := float32(0)
	if loadVal, ok := data["load"].(float64); ok {
		load = float32(loadVal)
	}
	gateway.DefaultManager.Insert(gatewayID, address, load)
}
