package repository

import "errors"

// PostgreSQL error code for a violation of the UNIQUE constraint.
const pgUniqueViolationCode = "23505"

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrTeamNotFound      = errors.New("team not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrDuplicateEntry    = errors.New("duplicate entry")

	// Team-specific errors
	ErrTeamMemberNotFound    = errors.New("team member not found")
	ErrTeamMemberExists      = errors.New("user is already a member of this team")
	ErrInsufficientPrivilege = errors.New("insufficient privilege")
)