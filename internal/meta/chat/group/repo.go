package group

import (
	"GoStacker/pkg/db/mysql"
	rdb "GoStacker/pkg/db/redis"
	"context"
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
		_ = rdb.Rdb.SAdd(context.Background(), fmt.Sprintf("users:joined:%d", userID), members...)
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
		_ = rdb.Rdb.SAdd(context.Background(), fmt.Sprintf("groups:members:%d", roomID), members...)
	}
	return memberIDs, nil
}

// SearchRoomsByName returns group rooms matching the fuzzy name query.
func SearchRoomsByName(q string, limit int) ([]RoomInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	like := "%" + q + "%"
	query := "SELECT id, name, creator_id, created_at FROM chat_rooms WHERE is_group = 1 AND name LIKE ? LIMIT ?"
	rows, err := mysql.DB.Query(query, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]RoomInfo, 0)
	for rows.Next() {
		var r RoomInfo
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatorID, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

func QueryRoomByID(roomID int64) (*RoomInfo, error) {
	query := "SELECT id, name, creator_id, created_at FROM chat_rooms WHERE id = ? LIMIT 1"
	var r RoomInfo
	err := mysql.DB.QueryRow(query, roomID).Scan(&r.ID, &r.Name, &r.CreatorID, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Join request storage
func ensureJoinRequestsTable() error {
	query := `CREATE TABLE IF NOT EXISTS join_requests (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		room_id BIGINT NOT NULL,
		user_id BIGINT NOT NULL,
		message TEXT DEFAULT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	_, err := mysql.DB.Exec(query)
	return err
}

func InsertJoinRequest(roomID int64, userID int64, message string) (int64, error) {
	if err := ensureJoinRequestsTable(); err != nil {
		return 0, err
	}
	q := "INSERT INTO join_requests (room_id, user_id, message, created_at) VALUES (?, ?, ?, ?)"
	res, err := mysql.DB.Exec(q, roomID, userID, message, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type JoinRequestRow struct {
	ID        int64     `json:"id"`
	RoomID    int64     `json:"room_id"`
	UserID    int64     `json:"user_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func QueryPendingJoinRequestsByRoom(roomID int64) ([]JoinRequestRow, error) {
	if err := ensureJoinRequestsTable(); err != nil {
		return nil, err
	}
	q := "SELECT id, room_id, user_id, message, created_at FROM join_requests WHERE room_id = ? ORDER BY created_at ASC"
	rows, err := mysql.DB.Query(q, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]JoinRequestRow, 0)
	for rows.Next() {
		var r JoinRequestRow
		if err := rows.Scan(&r.ID, &r.RoomID, &r.UserID, &r.Message, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

func QueryJoinRequestByID(id int64) (*JoinRequestRow, error) {
	if err := ensureJoinRequestsTable(); err != nil {
		return nil, err
	}
	q := "SELECT id, room_id, user_id, message, created_at FROM join_requests WHERE id = ? LIMIT 1"
	var r JoinRequestRow
	err := mysql.DB.QueryRow(q, id).Scan(&r.ID, &r.RoomID, &r.UserID, &r.Message, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func DeleteJoinRequestByID(id int64) error {
	if err := ensureJoinRequestsTable(); err != nil {
		return err
	}
	q := "DELETE FROM join_requests WHERE id = ?"
	_, err := mysql.DB.Exec(q, id)
	return err
}
