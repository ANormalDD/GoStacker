package user

import (
	"GoStacker/pkg/db/mysql"
	"fmt"

	"go.uber.org/zap"
)

func InsertUser(username, hashedPassword, nickname string) error {
	query := "INSERT INTO users (username, password_hash, nickname) VALUES (?, ?, ?)"
	_, err := mysql.DB.Exec(query, username, hashedPassword, nickname)
	if err != nil {
		zap.L().Error("failed to insert user", zap.Error(err))
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

func GetUserByUsername(username string) (string, int64, error) {
	var passwordHash string
	var id int64
	query := "SELECT id, password_hash FROM users WHERE username = ?"
	err := mysql.DB.QueryRow(query, username).Scan(&id, &passwordHash)
	if err != nil {
		zap.L().Error("failed to get user by username", zap.Error(err))
		return "", 0, fmt.Errorf("failed to get user by username: %w", err)
	}
	return passwordHash, id, nil
}
