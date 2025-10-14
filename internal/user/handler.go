package user

import (
	"GoStacker/pkg/response"
	"GoStacker/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, err.Error())
		return
	}
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		zap.L().Error("failed to hash password", zap.Error(err))
		response.ReplyError500(c, "Failed to hash password")
		return
	}

	// Save the user to the database (omitted for brevity)
	err = CreateUser(req.Username, hashedPassword, req.Nickname)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "User registered successfully")
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, err.Error())
		return
	}
	storedHash, id, err := GetUserByUsername(req.Username)
	if err != nil {
		response.ReplyUnauthorized(c, "Invalid username or password")
		return
	}
	if !utils.CheckPasswordHash(req.Password, storedHash) {
		response.ReplyUnauthorized(c, "Invalid username or password")
		return
	}

	token, err := utils.GenerateToken(req.Username, id)
	if err != nil {
		zap.L().Error("failed to generate token", zap.Error(err))
		response.ReplyError500(c, "Failed to generate token")
		return
	}
	response.ReplySuccessWithData(c, "Login successful", gin.H{"token": token})
}
