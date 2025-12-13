package main

import (
	"GoStacker/internal/meta/chat/group"
	"GoStacker/internal/meta/user"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/middleware"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/response"

	"github.com/gin-gonic/gin"
)

// NewRouter creates a gin engine for the meta service.
// It registers health/metrics, register/login and metadata (group/user) routes.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(logger.GinLogger(), logger.GinRecovery(true))

	// health and metrics
	g.GET("/ping", func(c *gin.Context) { response.ReplySuccess(c, "pong") })
	g.GET("/metrics", gin.WrapH(monitor.Handler()))

	// auth endpoints
	g.POST("/register", user.RegisterHandler)
	g.POST("/login", user.LoginHandler)

	// authenticated metadata routes
	auth := g.Group("/api", middleware.JWTAuthMiddleware())
	{
		auth.POST("/chat/group/create", group.CreateRoomHandler)
		auth.POST("/chat/group/add_member", group.AddRoomMemberHandler)
		auth.POST("/chat/group/add_members", group.AddRoomMembersHandler)
		auth.POST("/chat/group/change_nickname", group.ChangeNicknameHandler)
		auth.POST("/chat/group/change_member_role", group.ChangeMemberRoleHandler)
		auth.POST("/chat/group/remove_member", group.RemoveMemberHandler)
	}

	return g
}
