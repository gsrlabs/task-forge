package service

import "errors"

var (
	// Auth errors
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUnauthorized       = errors.New("unauthorized")

	// Team errors
	ErrTeamNotFound          = errors.New("team not found")
	ErrUserNotFound          = errors.New("user not found")
	ErrInsufficientPrivilege = errors.New("insufficient privilege")
	ErrAlreadyTeamMember     = errors.New("user is already a member of this team")
	ErrInvalidRole           = errors.New("invalid role")
	ErrCannotInviteSelf      = errors.New("cannot invite yourself")
)
