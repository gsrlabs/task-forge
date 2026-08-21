// internal/service/errors.go
package service

import "errors"

var (
	// Auth-specific
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUnauthorized       = errors.New("unauthorized")

	// Team-specific
	ErrTeamNotFound          = errors.New("team not found")
	ErrUserNotFound          = errors.New("user not found")
	ErrInsufficientPrivilege = errors.New("insufficient privilege")
	ErrAlreadyTeamMember     = errors.New("user is already a member of this team")
	ErrInvalidRole           = errors.New("invalid role")
	ErrCannotInviteSelf      = errors.New("cannot invite yourself")

	// Task-specific
	ErrInvalidTaskID     = errors.New("invalid task ID")
	ErrInvalidTeamID     = errors.New("invalid team ID")
	ErrInvalidAssigneeID = errors.New("invalid assignee ID")
	ErrInvalidStatus     = errors.New("invalid task status")
	ErrNotTeamMember     = errors.New("user is not a member of this team")
	ErrAssigneeNotMember = errors.New("assignee is not a member of this team")
	ErrTaskAccessDenied  = errors.New("access denied to task")
	ErrNoFieldsToUpdate  = errors.New("no fields to update")
	ErrInvalidPagination = errors.New("invalid pagination parameters")
	ErrInvalidFilter     = errors.New("invalid filter parameter")
)
