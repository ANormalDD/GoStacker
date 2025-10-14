package server

import (
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/internal/ws"
	"GoStacker/internal/chat"
	"GoStacker/internal/user"

	"github.com/gin-gonic/gin"
)

// NewRouter creates and returns a gin.Engine with middleware and routes registered.
// Put route registration here so `cmd/server/main.go` stays concise.
func NewRouter() *gin.Engine {
	// Use gin in release mode in production
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	// register logger and recovery from pkg/logger
	g.Use(logger.GinLogger(), logger.GinRecovery(true))

	// health check
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	g.POST("/register", user.RegisterHandler)
	g.POST("/login", user.LoginHandler)
	auth := g.Group("/api", middleware.JWTAuthMiddleware())
	{
		auth.POST("/chat/create", chat.CreateRoomHandler)
		auth.GET("/ws", ws.WebSocketHandler)
	}

	return g
}
