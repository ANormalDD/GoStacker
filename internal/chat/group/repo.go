package group

import (
	"GoStacker/pkg/db/mysql"
	rdb "GoStacker/pkg/db/redis"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func InsertRoom(name string, isGroup bool, creatorID int64) (int64, error) {
	query := "INSERT INTO chat_rooms (name, is_group, creator_id, created_at) VALUES (?, ?, ?, ?)"
	result, err := mysql.DB.Exec(query, name, isGroup, creatorID, time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func CreateRoomMemberTable(roomID int64) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            user_id BIGINT NOT NULL,
            nickname VARCHAR(100) DEFAULT NULL,
            role SMALLINT DEFAULT 0,
            mute_until DATETIME DEFAULT '1970-01-01 08:00:00',
            joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    `, tableName)
	_, err := mysql.DB.Exec(query)
	return err
}

func InsertRoomMember(roomID int64, userID int64) error {
	return AddRoomMemberCache(roomID, userID)
}
func InsertRoomMembers(roomID int64, userIDs []int64) error {
	return AddRoomMembersCache(roomID, userIDs)
}
func DeleteRoomMember(roomID int64, userID int64) error {
	return RemoveRoomMemberCache(roomID, userID)
}
func UpdateMemberNickname(roomID int64, userID int64, nickname string) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("UPDATE %s SET nickname = ? WHERE user_id = ?", tableName)
	_, err := mysql.DB.Exec(query, nickname, userID)
	return err
}
func QueryMemberRole(roomID int64, userID int64) (int16, error) {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("SELECT role FROM %s WHERE user_id = ?", tableName)
	var role int16
	err := mysql.DB.QueryRow(query, userID).Scan(&role)
	if err != nil {
		return -1, err
	}
	return role, nil
}

func UpdateMemberRole(roomID int64, userID int64, role int16) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("UPDATE %s SET role = ? WHERE user_id = ?", tableName)
	_, err := mysql.DB.Exec(query, role, userID)
	return err
}

func UpdateMuteUntil(roomID int64, userID int64, muteUntil time.Time) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("UPDATE %s SET mute_until = ? WHERE user_id = ?", tableName)
	_, err := mysql.DB.Exec(query, muteUntil, userID)
	return err
}

func QueryIsGroupRoom(roomID int64) (bool, error) {
	query := "SELECT is_group FROM chat_rooms WHERE id = ?"
	var isGroup bool
	err := mysql.DB.QueryRow(query, roomID).Scan(&isGroup)
	if err != nil {
		return false, err
	}
	return isGroup, nil
}

func IsRoomMember(roomID int64, userID int64) (bool, error) {
	// try cache first
	if ok, err := IsRoomMemberCache(roomID, userID); err == nil {
		return ok, nil
	}
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE user_id = ?", tableName)
	var count int
	err := mysql.DB.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func IsGroupRoom(roomID int64) (bool, error) {
	query := "SELECT is_group FROM chat_rooms WHERE id = ?"
	var isGroup bool
	err := mysql.DB.QueryRow(query, roomID).Scan(&isGroup)
	if err != nil {
		return false, err
	}
	return isGroup, nil
}

func QueryJoinedRooms(userID int64) ([]int64, error) {
	// try cache first
	if rooms, err := GetUserJoinedRoomsCache(userID); err == nil && len(rooms) > 0 {
		return rooms, nil
	}
	// fallback to DB and populate cache
	query := "SELECT joined_chatrooms FROM users WHERE id = ?"
	var joinedChatrooms sql.NullString
	err := mysql.DB.QueryRow(query, userID).Scan(&joinedChatrooms)
	if err != nil {
		return nil, err
	}
	if !joinedChatrooms.Valid || joinedChatrooms.String == "" {
		return []int64{}, nil
	}
	roomIDStrs := strings.Split(joinedChatrooms.String, ",")
	roomIDs := []int64{}
	for _, idStr := range roomIDStrs {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, err
		}
		roomIDs = append(roomIDs, id)
	}
	// populate cache set
	if len(roomIDs) > 0 {
		members := make([]interface{}, 0, len(roomIDs))
		for _, r := range roomIDs {
			members = append(members, strconv.FormatInt(r, 10))
		}
		_ = rdb.Rdb.SAdd(fmt.Sprintf("users:joined:%d", userID), members...)
	}
	return roomIDs, nil
}

func QueryRoomMemberIDs(roomID int64) ([]int64, error) {
	// try cache first
	if members, err := GetRoomMemberIDsCache(roomID); err == nil && len(members) > 0 {
		return members, nil
	}
	// fallback to DB
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("SELECT user_id FROM %s", tableName)
	rows, err := mysql.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	memberIDs := []int64{}
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		memberIDs = append(memberIDs, userID)
	}
	// populate cache
	if len(memberIDs) > 0 {
		members := make([]interface{}, 0, len(memberIDs))
		for _, u := range memberIDs {
			members = append(members, strconv.FormatInt(u, 10))
		}
		_ = rdb.Rdb.SAdd(fmt.Sprintf("groups:members:%d", roomID), members...)
	}
	return memberIDs, nil
}
