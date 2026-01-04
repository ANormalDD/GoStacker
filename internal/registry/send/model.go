package send

import "time"

// SendInstanceInfo represents a send service instance
type SendInstanceInfo struct {
	InstanceID    string    `json:"instance_id"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// RegisterRequest represents send instance registration request
type RegisterRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Address    string `json:"address" binding:"required"`
	Port       int    `json:"port" binding:"required"`
}

// HeartbeatRequest represents send instance heartbeat request
type HeartbeatRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}
