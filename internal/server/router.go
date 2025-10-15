package server

import (
	"GoStacker/internal/chat/group"
	"GoStacker/internal/user"
	"GoStacker/internal/ws"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/response"

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
		response.ReplySuccess(c, "pong")
	})

	g.POST("/register", user.RegisterHandler)
	g.POST("/login", user.LoginHandler)
	auth := g.Group("/api", middleware.JWTAuthMiddleware())
	{
		auth.POST("/chat/group/create", group.CreateRoomHandler)
		auth.POST("/chat/group/add_member", group.AddRoomMemberHandler)
		auth.POST("/chat/group/add_members", group.AddRoomMembersHandler)
		auth.POST("/chat/group/change_nickname", group.ChangeNicknameHandler)
		auth.POST("/chat/group/change_member_role", group.ChangeMemberRoleHandler)
		auth.POST("/chat/group/remove_member", group.RemoveMemberHandler)
		auth.GET("/ws", ws.WebSocketHandler)
	}

	return g
}
