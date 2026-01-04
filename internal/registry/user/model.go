package user

import "time"

// UserRoute represents user to gateway mapping
type UserRoute struct {
	UserID      int64     `json:"user_id"`
	GatewayID   string    `json:"gateway_id"`
	Address     string    `json:"address"`
	ConnectedAt time.Time `json:"connected_at"`
	Status      string    `json:"status"` // "connected" or "disconnected"
}

// ConnectRequest represents user connection notification
type ConnectRequest struct {
	UserID    int64  `json:"user_id" binding:"required"`
	GatewayID string `json:"gateway_id" binding:"required"`
}

// DisconnectRequest represents user disconnection notification
type DisconnectRequest struct {
	UserID    int64  `json:"user_id" binding:"required"`
	GatewayID string `json:"gateway_id" binding:"required"`
}

// BatchQueryRoutesRequest represents batch route query request
type BatchQueryRoutesRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

// RouteInfo represents simplified route information in response
type RouteInfo struct {
	GatewayID string `json:"gateway_id"`
	Address   string `json:"address"`
}

// GatewayAvailableResponse represents response for available gateway query
type GatewayAvailableResponse struct {
	GatewayID string `json:"gateway_id"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
}
