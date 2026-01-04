package server

import (
	"GoStacker/internal/meta/chat/group"
	"GoStacker/internal/meta/user"
	"GoStacker/internal/send/chat/send"
	user_ws "GoStacker/internal/send/ws"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

// NewRouter creates and returns a gin.Engine with middleware and routes registered.
// Put route registration here so `cmd/server/main.go` stays concise.
func NewRouter(PushMod string) *gin.Engine {
	// Use gin in release mode in production
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	// register logger and recovery from pkg/logger
	g.Use(logger.GinLogger(), logger.GinRecovery(true))

	// health check
	g.GET("/ping", func(c *gin.Context) {
		response.ReplySuccess(c, "pong")
	})

	g.GET("/metrics", gin.WrapH(monitor.Handler()))
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
		if PushMod == "standalone" {
			auth.GET("/ws", user_ws.WebSocketHandler)
		}
		auth.POST("/chat/send_message", send.SendMessageHandler)
	}
	// Gateway mode no longer needs WebSocket endpoint at server level
	// Gateway instances register directly with Registry service
	return g
}
