package chat

import (
	"errors"
	"time"
)

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