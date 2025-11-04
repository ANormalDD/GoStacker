package register

import (
	"GoStacker/internal/gateway"
	"strconv"

	"go.uber.org/zap"
)

func RegisterGatewayHandler(msg map[string]interface{}, gatewayID string) {
	zap.L().Debug("register: handler invoked", zap.Any("msg", msg), zap.String("gateway_id", gatewayID))
	gatewayInfo, ok := msg["gateway_info"].(map[string]interface{})
	if !ok {
		zap.L().Error("register: missing or invalid gateway_info field")
		return
	}
	address, ok := gatewayInfo["address"].(string)
	if !ok {
		zap.L().Error("register: missing or invalid address field in gateway_info")
		return
	}
	port, ok := gatewayInfo["port"].(int)
	if !ok {
		zap.L().Error("register: missing or invalid port field in gateway_info")
		return
	}
	address = address + ":" + strconv.Itoa(port)
	load := float32(0)
	if loadVal, ok := gatewayInfo["load"].(float64); ok {
		load = float32(loadVal)
	}
	gateway.DefaultManager.Insert(gatewayID, address, load)
}
