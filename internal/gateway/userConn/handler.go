package userConn

import (
	"sync"

	"go.uber.org/zap"
)

var userID2GatewayMap sync.Map // map[int64]string

func UserConnHandler(msg map[string]interface{}, gatewayID string) {
	zap.L().Debug("userConn: handler invoked", zap.Any("msg", msg), zap.String("gatewayID", gatewayID))
	var userID int64
	if userIDVal, ok := msg["user_id"]; ok {
		switch v := userIDVal.(type) {
		case float64:
			userID = int64(v)
		case int:
			userID = int64(v)
		case int64:
			userID = v
		default:
			zap.L().Error("register: invalid port type", zap.Any("actual_type", v))
			return
		}
	} else {
		zap.L().Error("register: missing port field in data")
		return
	}
	userID2GatewayMap.Store(userID, gatewayID)
}

func UserDisconnHandler(msg map[string]interface{}) {
	zap.L().Debug("userConn: disconn handler invoked", zap.Any("msg", msg))
	var userID int64
	if userIDVal, ok := msg["user_id"]; ok {
		switch v := userIDVal.(type) {
		case float64:
			userID = int64(v)
		case int:
			userID = int64(v)
		case int64:
			userID = v
		default:
			zap.L().Error("register: invalid port type", zap.Any("actual_type", v))
			return
		}
	} else {
		zap.L().Error("register: missing port field in data")
		return
	}
	userID2GatewayMap.Delete(userID)
}

func GetGatewayIDByUserID(userID int64) (string, bool) {
	value, ok := userID2GatewayMap.Load(userID)
	if !ok {
		return "", false
	}
	return value.(string), true
}
