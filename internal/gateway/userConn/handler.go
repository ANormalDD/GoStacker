package userConn

import (
	"sync"

	"go.uber.org/zap"
)

var userID2GatewayMap sync.Map // map[int64]string

func UserConnHandler(msg map[string]interface{}, gatewayID string) {
	zap.L().Debug("userConn: handler invoked", zap.Any("msg", msg), zap.String("gatewayID", gatewayID))
	userID := msg["user_id"].(int64)
	userID2GatewayMap.Store(userID, gatewayID)
}

func UserDisconnHandler(msg map[string]interface{}) {
	zap.L().Debug("userConn: disconn handler invoked", zap.Any("msg", msg))
	userID := msg["user_id"].(int64)
	userID2GatewayMap.Delete(userID)
}

func GetGatewayIDByUserID(userID int64) (string, bool) {
	value, ok := userID2GatewayMap.Load(userID)
	if !ok {
		return "", false
	}
	return value.(string), true
}
