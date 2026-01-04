package main

import (
	"GoStacker/internal/registry/gateway"
	"GoStacker/internal/registry/send"
	"GoStacker/internal/registry/user"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

// NewRouter creates a gin engine for the registry service
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(logger.GinLogger(), logger.GinRecovery(true))

	// Health and metrics
	g.GET("/ping", func(c *gin.Context) {
		response.ReplySuccess(c, "pong")
	})
	g.GET("/health", gateway.HealthCheckHandler)
	g.GET("/metrics", gin.WrapH(monitor.Handler()))

	// Registry API routes
	api := g.Group("/registry")
	{
		// Gateway management
		gwGroup := api.Group("/gateway")
		{
			gwGroup.POST("/register", gateway.RegisterHandler)
			gwGroup.POST("/heartbeat", gateway.HeartbeatHandler)
			gwGroup.DELETE("/:gateway_id", gateway.UnregisterHandler)
			gwGroup.GET("/instances", gateway.ListGatewaysHandler)
		}

		// Send instance management
		sendGroup := api.Group("/send")
		{
			sendGroup.POST("/register", send.RegisterHandler)
			sendGroup.POST("/heartbeat", send.HeartbeatHandler)
			sendGroup.DELETE("/:instance_id", send.UnregisterHandler)
			sendGroup.GET("/instances", send.ListInstancesHandler)
		}

		// User route management
		userGroup := api.Group("/user")
		{
			userGroup.POST("/connect", user.ConnectHandler)
			userGroup.POST("/disconnect", user.DisconnectHandler)
			userGroup.POST("/routes/batch", user.BatchQueryRoutesHandler)
		}

		// Gateway discovery (for clients)
		api.GET("/gateway/available", user.GetAvailableGatewayHandler)
		// Send instance discovery (for clients)
		api.GET("/send/available", send.GetAvailableSendHandler)
	}

	return g
}
