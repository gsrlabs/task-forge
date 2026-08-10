// internal/domain/team.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// TeamRole the user’s role in the team.
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
)

// Team model.
type Team struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamMember model of a team member.
type TeamMember struct {
	TeamID   uuid.UUID `json:"team_id"`
	UserID   uuid.UUID `json:"user_id"`
	Role     TeamRole  `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// TeamWithRole a command with the role of the current user (for the list of commands).
type TeamWithRole struct {
	Team
	UserRole TeamRole `json:"user_role"`
}