package main

import (
	"GoStacker/internal/send/chat/send"
	gateway_ws "GoStacker/internal/send/gateway/ws"
	"GoStacker/pkg/config"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/response"
	"GoStacker/internal/send/getGatewayAddr"
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
	// authenticated routes
	auth := g.Group("/api", middleware.JWTAuthMiddleware())
	{
		// send route is always registered in send service; handler decides standalone/gateway
		auth.POST("/chat/send_message", send.SendMessageHandler)
		auth.GET("/get_gateway_ws", getGatewayAddr.GetGatewayAddrHandler)

	}
	
	// If this send instance is running in gateway mode, also register gateway ws
	if config.Conf != nil && config.Conf.PushMod == "gateway" {
		gateway := g.Group("/gateway")
		{
			gateway.GET("/ws", gateway_ws.WebSocketHandler)
		}
	}

	return g
}
