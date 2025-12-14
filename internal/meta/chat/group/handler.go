package group

import (
	"GoStacker/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CreateRoomRequest struct {
	Name      string  `json:"name" binding:"required,min=1,max=100"`
	IsGroup   bool    `json:"is_group"`
	MemberIDs []int64 `json:"member_ids" binding:"required"`
}

type AddRoomMemberRequest struct {
	RoomID   int64 `json:"room_id" binding:"required"`
	MemberID int64 `json:"member_id" binding:"required"`
}

type AddRoomMembersRequest struct {
	RoomID    int64   `json:"room_id" binding:"required"`
	MemberIDs []int64 `json:"member_ids" binding:"required"`
}

type ChangeNicknameRequest struct {
	RoomID   int64  `json:"room_id" binding:"required"`
	Nickname string `json:"nickname" binding:"required,min=1,max=100"`
}

type ChangeMemberRoleRequest struct {
	RoomID       int64 `json:"room_id" binding:"required"`
	TargetUserID int64 `json:"target_user_id" binding:"required"`
	NewRole      int16 `json:"new_role" binding:"required"`
}

type RemoveMemberRequest struct {
	RoomID       int64 `json:"room_id" binding:"required"`
	TargetUserID int64 `json:"target_user_id" binding:"required"`
}

type JoinRoomRequest struct {
	RoomID int64 `json:"room_id" binding:"required"`
}

type RequestJoinRequest struct {
	RoomID  int64  `json:"room_id" binding:"required"`
	Message string `json:"message"`
}

type RespondJoinReq struct {
	RequestID int64 `json:"request_id" binding:"required"`
	Approve   bool  `json:"approve"`
}

func CreateRoomHandler(c *gin.Context) {
	var req CreateRoomRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	roomID, err := CreateRoom(req.Name, req.IsGroup, userID.(int64), append(req.MemberIDs, userID.(int64)))
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "Chat room created successfully", gin.H{"room_id": roomID})

}

func AddRoomMemberHandler(c *gin.Context) {
	var req AddRoomMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := AddRoomMember(req.RoomID, req.MemberID, userID.(int64))
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Member added successfully")
}

func AddRoomMembersHandler(c *gin.Context) {
	var req AddRoomMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := AddRoomMembers(req.RoomID, req.MemberIDs, userID.(int64))
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Members added successfully")
}

func ChangeNicknameHandler(c *gin.Context) {
	var req ChangeNicknameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := ChangeMemberNickname(req.RoomID, userID.(int64), req.Nickname)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Nickname changed successfully")
}

func ChangeMemberRoleHandler(c *gin.Context) {
	var req ChangeMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := ChangeMemberRole(req.RoomID, req.TargetUserID, req.NewRole, userID.(int64))
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Member role changed successfully")
}
func RemoveMemberHandler(c *gin.Context) {
	var req RemoveMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := DeleteRoomMember(req.RoomID, req.TargetUserID)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Member removed successfully")
}
func JoinRoomHandler(c *gin.Context) {
	var req JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	err := JoinRoom(userID.(int64), req.RoomID)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Joined room successfully")
}

func SearchRoomsHandler(c *gin.Context) {
	q := c.Query("q")
	limitStr := c.Query("limit")
	limit := 20
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	rooms, err := SearchRooms(q, limit)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "ok", gin.H{"rooms": rooms})
}
func RequestJoinHandler(c *gin.Context) {
	var req RequestJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	uid := userID.(int64)
	id, err := RequestJoin(uid, req.RoomID, req.Message)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "request created", gin.H{"request_id": id})
}

func GetPendingJoinRequestsHandler(c *gin.Context) {
	roomIDStr := c.Query("room_id")
	if roomIDStr == "" {
		response.ReplyBadRequest(c, "room_id required")
		return
	}
	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		response.ReplyBadRequest(c, "invalid room_id")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	uid := userID.(int64)
	rows, err := GetPendingJoinRequests(uid, roomID)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "ok", gin.H{"requests": rows})
}

func RespondJoinRequestHandler(c *gin.Context) {
	var req RespondJoinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	uid := userID.(int64)
	if err := RespondJoinRequest(uid, req.RequestID, req.Approve); err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "ok")
}
func GetJoinedRoomsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID = userID.(int64)
	roomIDs, err := GetJoinedChatRooms(userID.(int64))
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccessWithData(c, "ok", gin.H{"room_ids": roomIDs})
}
