// internal/repository/errors.go
package repository

import "errors"

// PostgreSQL error code for a violation of the UNIQUE constraint.
const pgUniqueViolationCode = "23505"

var (
	// User-specific errors
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")

	// Team-specific errors
	ErrTeamNotFound       = errors.New("team not found")
	ErrTeamMemberNotFound = errors.New("team member not found")
	ErrTeamMemberExists   = errors.New("user is already a member of this team")

	// Task-specific errors
	ErrTaskNotFound = errors.New("task not found")
)
