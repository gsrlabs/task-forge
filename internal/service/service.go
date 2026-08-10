// internal/service/service.go
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
	Tasks TaskService
}

// AuthService describes the contract for authentication.
type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (uuid.UUID, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
}

// TeamService describes the contract for working with teams.
type TeamService interface {
	Create(ctx context.Context, userID uuid.UUID, req *dto.CreateTeamRequest) (*dto.CreateTeamResponse, error)
	List(ctx context.Context, userID uuid.UUID) ([]dto.TeamListItem, error)
	Invite(ctx context.Context, teamID, inviterID uuid.UUID, req *dto.InviteUserRequest) (*dto.InviteUserResponse, error)
}

// TaskService describes a contract for working with tasks.
type TaskService interface {
	Create(
		ctx context.Context,
		userID uuid.UUID,
		req *dto.CreateTaskRequest,
	) (*dto.TaskResponse, error)

	List(
		ctx context.Context,
		userID uuid.UUID,
		teamID string,
		status *string,
		assigneeID *string,
		limit, offset int,
	) (*dto.TaskListResponse, error)

	Update(
		ctx context.Context,
		userID, taskID uuid.UUID,
		req *dto.UpdateTaskRequest,
	) (*dto.TaskResponse, error)

	GetHistory(
		ctx context.Context,
		userID, taskID uuid.UUID,
	) (*dto.TaskHistoryResponse, error)
}


// NewServices creates a container with all the services.
func NewServices(repos *repository.Repositories, jwtManager *JWTManager, logger zerolog.Logger) *Services {
	return &Services{
		Auth:  NewAuthService(repos.Users, jwtManager, logger),
		Teams: NewTeamService(repos.Teams, repos.Users, logger),
		Tasks: NewTaskService(repos.Tasks, repos.Teams, logger),
	}
}
