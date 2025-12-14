package group

import (
	"GoStacker/pkg/push"
	"errors"
	"time"
)

type RoomInfo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatorID int64     `json:"creator_id"`
	CreatedAt time.Time `json:"created_at"`
}

type MemberInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

const (
	RoleOwner  = 0
	RoleAdmin  = 1
	RoleMember = 2
)

func CreateRoom(name string, isGroup bool, creatorID int64, memberIDs []int64) (int64, error) {
	roomID, err := InsertRoom(name, isGroup, creatorID)
	if err != nil {
		return 0, err
	}
	err = CreateRoomMemberTable(roomID)
	if err != nil {
		return 0, err
	}
	err = InsertRoomMembers(roomID, memberIDs)
	if err != nil {
		return 0, err
	}
	err = InsertRoomMember(roomID, creatorID)
	if err != nil {
		return 0, err
	}

	return roomID, nil
}

func AddRoomMembers(roomID int64, userIDs []int64, requestUserID int64) error {

	isGroup, err := QueryIsGroupRoom(roomID)
	if err != nil {
		return err
	}
	if !isGroup {
		return errors.New("cannot add members to a private chat")
	}

	requestUserRole, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return err
	}
	if requestUserRole != RoleAdmin && requestUserRole != RoleOwner {
		return errors.New("permission denied")
	}
	return InsertRoomMembers(roomID, userIDs)
}

func AddRoomMember(roomID int64, userID int64, requestUserID int64) error {

	isGroup, err := QueryIsGroupRoom(roomID)
	if err != nil {
		return err
	}
	if !isGroup {
		return errors.New("cannot add members to a private chat")
	}

	requestUserRole, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return err
	}
	if requestUserRole != RoleAdmin && requestUserRole != RoleOwner {
		return errors.New("permission denied")
	}
	return InsertRoomMember(roomID, userID)
}

func ChangeMemberNickname(roomID int64, userID int64, nickname string) error {
	return UpdateMemberNickname(roomID, userID, nickname)
}

func ChangeMemberRole(roomID int64, targetUserID int64, newRole int16, requestUserID int64) error {
	requestUserRole, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return err
	}
	if requestUserRole != RoleOwner {
		return errors.New("permission denied")
	}
	return UpdateMemberRole(roomID, targetUserID, newRole)
}

func RemoveRoomMember(roomID int64, targetUserID int64, requestUserID int64) error {
	requestUserRole, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return err
	}
	if requestUserRole != RoleAdmin && requestUserRole != RoleOwner {
		return errors.New("permission denied")
	}
	return DeleteRoomMember(roomID, targetUserID)
}

func MuteMember(roomID int64, targetUserID int64, muteUntil time.Time, requestUserID int64) error {
	isGroupRoom, err := QueryIsGroupRoom(roomID)
	if err != nil {
		return err
	}
	if !isGroupRoom {
		return errors.New("cannot mute members in a private chat")
	}
	requestUserRole, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return err
	}
	targetUserRole, err := QueryMemberRole(roomID, targetUserID)
	if err != nil {
		return err
	}
	if requestUserRole >= targetUserRole {
		return errors.New("permission denied")
	}
	return UpdateMuteUntil(roomID, targetUserID, muteUntil)
}

func GetJoinedChatRooms(userID int64) ([]int64, error) {
	return QueryJoinedRooms(userID)
}

// JoinRoom allows a user to actively join a group room.
func JoinRoom(userID int64, roomID int64) error {
	isGroup, err := QueryIsGroupRoom(roomID)
	if err != nil {
		return err
	}
	if !isGroup {
		return errors.New("cannot join a private chat")
	}
	// if already member, consider idempotent success
	if ok, err := IsRoomMember(roomID, userID); err == nil && ok {
		return nil
	}
	return InsertRoomMember(roomID, userID)
}

// SearchRooms searches public/group rooms by name (fuzzy) with a limit.
func SearchRooms(query string, limit int) ([]RoomInfo, error) {
	return SearchRoomsByName(query, limit)
}

// RequestJoin creates a join request and notifies group owner/admins.
func RequestJoin(userID int64, roomID int64, message string) (int64, error) {
	// check group
	isGroup, err := QueryIsGroupRoom(roomID)
	if err != nil {
		return 0, err
	}
	if !isGroup {
		return 0, errors.New("cannot request to join a private chat")
	}
	// insert request
	reqID, err := InsertJoinRequest(roomID, userID, message)
	if err != nil {
		return 0, err
	}
	// notify creator and admins
	// get room info
	if roomInfo, err := QueryRoomByID(roomID); err == nil && roomInfo != nil {
		_ = push.EnqueueMessage(roomInfo.CreatorID, 100*time.Millisecond, push.ClientMessage{Type: "group_join_request", RoomID: roomID, Payload: map[string]interface{}{"request_id": reqID}})
	}
	// notify admins: iterate members and push to those with admin/owner roles
	memberIDs, err := QueryRoomMemberIDs(roomID)
	if err == nil {
		for _, mid := range memberIDs {
			role, err := QueryMemberRole(roomID, mid)
			if err != nil {
				continue
			}
			if role == RoleOwner || role == RoleAdmin {
				_ = push.EnqueueMessage(mid, 100*time.Millisecond, push.ClientMessage{Type: "group_join_request", RoomID: roomID, Payload: map[string]interface{}{"request_id": reqID}})
			}
		}
	}
	return reqID, nil
}

// List pending requests for a room (must be admin/owner)
func GetPendingJoinRequests(requestUserID int64, roomID int64) ([]JoinRequestRow, error) {
	// permission check
	role, err := QueryMemberRole(roomID, requestUserID)
	if err != nil {
		return nil, err
	}
	if role != RoleOwner && role != RoleAdmin {
		return nil, errors.New("permission denied")
	}
	return QueryPendingJoinRequestsByRoom(roomID)
}

// RespondJoinRequest allows admin/owner to approve or reject a join request.
func RespondJoinRequest(requestUserID int64, requestID int64, approve bool) error {
	// find request
	target, err := QueryJoinRequestByID(requestID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("request not found")
	}
	if target == nil {
		return errors.New("request not found")
	}
	// check permission on target.RoomID
	role, err := QueryMemberRole(target.RoomID, requestUserID)
	if err != nil {
		return err
	}
	if role != RoleOwner && role != RoleAdmin {
		return errors.New("permission denied")
	}
	if approve {
		if err := InsertRoomMember(target.RoomID, target.UserID); err != nil {
			return err
		}
	}
	// delete request
	return DeleteJoinRequestByID(requestID)
}
