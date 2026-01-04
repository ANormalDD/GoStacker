package gateway

import "time"

// GatewayInfo represents a gateway instance
type GatewayInfo struct {
	GatewayID      string    `json:"gateway_id"`
	Address        string    `json:"address"`
	Port           int       `json:"port"`
	Load           float32   `json:"load"`
	Capacity       int       `json:"capacity"`
	ConnectedUsers int       `json:"connected_users"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
}

// RegisterRequest represents gateway registration request
type RegisterRequest struct {
	GatewayID string `json:"gateway_id" binding:"required"`
	Address   string `json:"address" binding:"required"`
	Port      int    `json:"port" binding:"required"`
	Capacity  int    `json:"capacity" binding:"required"` // Maximum connections
}

// HeartbeatRequest represents gateway heartbeat request
type HeartbeatRequest struct {
	GatewayID      string  `json:"gateway_id" binding:"required"`
	Load           float32 `json:"load"`
	ConnectedUsers int     `json:"connected_users"`
}

// HealthCheckResponse represents health check response
type HealthCheckResponse struct {
	Status         string `json:"status"`
	Redis          string `json:"redis"`
	GatewayCount   int    `json:"gateway_count"`
	SendCount      int    `json:"send_count"`
	UserRouteCount int    `json:"user_route_count"`
}
