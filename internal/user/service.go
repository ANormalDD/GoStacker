package user

import (
	"errors"
)

func CreateUser(username, hashedPassword, nickname string) error {
	// Simulate a user creation process
	if username == "" || hashedPassword == "" {
		return errors.New("username and password cannot be empty")
	}
	if len(username) < 3 || len(username) > 50 {
		return errors.New("username must be between 3 and 50 characters")
	}
	if len(nickname) > 50 {
		return errors.New("nickname cannot exceed 50 characters")
	}
	// Here you would typically interact with your database to save the user
	err := InsertUser(username, hashedPassword, nickname)
	if err != nil {
		return err
	}
	return nil
}
