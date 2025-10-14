package chat

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type CreateRoomRequest struct {
	Name      string  `json:"name" binding:"required,min=1,max=100"`
	IsGroup   bool    `json:"is_group"`
	MemberIDs []int64 `json:"member_ids" binding:"required"`
}

type AddRoomMemberRequest struct {
	RoomID        int64 `json:"room_id" binding:"required"`
	MemberID      int64 `json:"member_id" binding:"required"`
}

type AddRoomMembersRequest struct {
	RoomID        int64   `json:"room_id" binding:"required"`
	MemberIDs     []int64 `json:"member_ids" binding:"required"`
}

type ChangeNicknameRequest struct {
	RoomID   int64  `json:"room_id" binding:"required"`
	Nickname string `json:"nickname" binding:"required,min=1,max=100"`
}

type ChangeMemberRoleRequest struct {
	RoomID        int64 `json:"room_id" binding:"required"`
	TargetUserID  int64 `json:"target_user_id" binding:"required"`
	NewRole       int16 `json:"new_role" binding:"required"`
}

type RemoveMemberRequest struct {
	RoomID        int64 `json:"room_id" binding:"required"`
	TargetUserID  int64 `json:"target_user_id" binding:"required"`
}

func CreateRoomHandler(c *gin.Context) {
	var req CreateRoomRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
}

func AddRoomMemberHandler(c *gin.Context) {
	var req AddRoomMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
	err := AddRoomMember(req.RoomID, req.MemberID, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}

func AddRoomMembersHandler(c *gin.Context) {
	var req AddRoomMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
	err := AddRoomMembers(req.RoomID, req.MemberIDs, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Members added successfully"})
}

func ChangeNicknameHandler(c *gin.Context) {
	var req ChangeNicknameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
	err := ChangeMemberNickname(req.RoomID, userID.(int64), req.Nickname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Nickname changed successfully"})
}

func ChangeMemberRoleHandler(c *gin.Context) {
	var req ChangeMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
	err := ChangeMemberRole(req.RoomID, req.TargetUserID, req.NewRole, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Member role changed successfully"})
}
func RemoveMemberHandler(c *gin.Context) {
	var req RemoveMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID = userID.(int64)
	err := DeleteRoomMember(req.RoomID, req.TargetUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}


