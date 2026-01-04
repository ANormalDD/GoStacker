package main

import (
	"GoStacker/internal/send/chat/send"
	"GoStacker/internal/send/pushback"
	"GoStacker/internal/send/pushnotify"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

// NewRouter creates a gin engine for the send service.
// It registers only health/metrics/auth/login/register and the send API.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(logger.GinLogger(), logger.GinRecovery(true))

	// health and metrics
	g.GET("/ping", func(c *gin.Context) {
		response.ReplySuccess(c, "pong")
	})
	g.GET("/metrics", gin.WrapH(monitor.Handler()))

	// Internal API (no auth required) - for Gateway to call
	internal := g.Group("/internal")
	{
		internal.POST("/pushback", pushback.PushbackHandler)
		internal.POST("/push/notify_online", pushnotify.NotifyOnlineHandler)
	}

	// authenticated routes
	auth := g.Group("/api", middleware.JWTAuthMiddleware())
	{
		// send route is always registered in send service; handler decides standalone/gateway
		auth.POST("/chat/send_message", send.SendMessageHandler)
	}

	return g
}
