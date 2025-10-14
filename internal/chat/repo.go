package chat

import (
	"GoStacker/pkg/db/mysql"
	"fmt"
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
            mute_until DATETIME DEFAULT 0,
            joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    `, tableName)
	_, err := mysql.DB.Exec(query)
	return err
}

func InsertRoomMember(roomID int64, userID int64) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("INSERT INTO %s (user_id) VALUES (?)", tableName)
	_, err := mysql.DB.Exec(query, userID)
	return err
}
func InsertRoomMembers(roomID int64, userIDs []int64) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("INSERT INTO %s (user_id) VALUES (?)", tableName)
	tx, err := mysql.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, userID := range userIDs {
		if _, err := stmt.Exec(userID); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
func DeleteRoomMember(roomID int64, userID int64) error {
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", tableName)
	_, err := mysql.DB.Exec(query, userID)
	return err
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
