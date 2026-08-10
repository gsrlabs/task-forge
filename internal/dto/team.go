// internal/dto/team.go
package dto

// TeamListItem is an element of the team list.
type TeamListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// =========================================================================
// Request
// =========================================================================

//CreateTeamRequest request to create a team.
type CreateTeamRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// InviteUserRequest request to invite a user to the team.
type InviteUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=admin member"`
}

// =========================================================================
// Response
// =========================================================================

// CreateTeamResponse the response to creating a team.
type CreateTeamResponse struct {
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}


// InviteUserResponse response to a user invitation.
type InviteUserResponse struct {
	Message string `json:"message"`
	TeamID  string `json:"team_id"`
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
}