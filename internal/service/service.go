package service

import (
	"context"

	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Services aggregates all the application’s services.
type Services struct {
	Auth  AuthService
	Teams TeamService
}

// AuthService describes the contract for authentication.
type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (uuid.UUID, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
}

// TeamService describes the contract for working with teams.
type TeamService interface {
	// Create creates a new command. The current user becomes the owner.
	Create(ctx context.Context, userID uuid.UUID, req *dto.CreateTeamRequest) (*dto.CreateTeamResponse, error)

	// List returns a list of teams the user is a member of.
	List(ctx context.Context, userID uuid.UUID) ([]dto.TeamListItem, error)

	// Invite invites the user to join the team.
	// Only the owner and admin can invite new participants.
	Invite(ctx context.Context, teamID, inviterID uuid.UUID, req *dto.InviteUserRequest) (*dto.InviteUserResponse, error)
}

// NewServices creates a container with all the services.
func NewServices(repos *repository.Repositories, jwtManager *JWTManager, logger zerolog.Logger) *Services {
	return &Services{
		Auth:  NewAuthService(repos.Users, jwtManager, logger),
		Teams: NewTeamService(repos.Teams, repos.Users, logger),
	}
}
