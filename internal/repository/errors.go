package repository

import "errors"

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrTeamNotFound      = errors.New("team not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrDuplicateEntry    = errors.New("duplicate entry")
)