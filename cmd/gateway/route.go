package main

import (
	"GoStacker/internal/gateway/center"
	"GoStacker/internal/gateway/user/ws"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/monitor"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(logger.GinLogger(), logger.GinRecovery(true))
	// metrics endpoint for Prometheus
	r.GET("/metrics", gin.WrapH(monitor.Handler()))
	User := r.Group("/api", middleware.JWTAuthMiddleware())
	{
		User.GET("/ws", ws.WebSocketHandler)
	}
	Center := r.Group("/center")
	{
		Center.POST("/forward", center.ForwardHandler)

	}
	return r
}
