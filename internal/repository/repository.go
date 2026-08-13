// internal/repository/repository.go
package repository

import (
	"context"
	"time"

	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Repositories aggregates all the application’s repositories.
type Repositories struct {
	Users UserRepository
	Teams TeamRepository
	Tasks TaskRepository
	Analytics AnalyticsRepository
}

// UserRepository describes a contract for working with users.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// TeamRepository describes the contract for working with teams.
type TeamRepository interface {
	Create(ctx context.Context, userID uuid.UUID, team *domain.Team) error
	FindByID(ctx context.Context, teamID uuid.UUID) (*domain.Team, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.TeamWithRole, error)
	AddMember(ctx context.Context, member *domain.TeamMember) error
	GetUserRole(ctx context.Context, teamID, userID uuid.UUID) (domain.TeamRole, error)
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	IsTeamMember(ctx context.Context, teamID uuid.UUID, userID uuid.UUID,) (bool, error)
}

// TaskRepository describes a contract for working with tasks.
type TaskRepository interface {
	Create(
		ctx context.Context,
		task *domain.Task,
		history domain.TaskHistory,
	) error

	FindByID(
		ctx context.Context,
		taskID uuid.UUID,
	) (*domain.Task, error)

	List(
		ctx context.Context,
		filter domain.TaskFilter,
		pagination domain.TaskPagination,
	) (*domain.TaskListResult, error)

	Update(
		ctx context.Context,
		taskID uuid.UUID,
		changedBy uuid.UUID,
		update domain.TaskUpdate,
		history domain.TaskHistory,
	) (*domain.Task, error)

	GetHistory(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]domain.TaskHistoryWithUser, error)

}

type AnalyticsRepository interface {
	GetTeamStats(ctx context.Context, days int, sinceDate time.Time) ([]domain.TeamStats, error)
	GetTopCreators(ctx context.Context, months, topN int) ([]domain.TopCreator, error)
	FindAssigneeIntegrityViolations(
		ctx context.Context,
		limit int,
	) ([]domain.IntegrityViolation, error)
}

// NewRepositories creates a container with all repositories.
func NewRepositories(db *pgxpool.Pool, logger zerolog.Logger) *Repositories {
	return &Repositories{
		Users: NewUserRepository(db, logger),
		Teams: NewTeamRepository(db, logger),
		Tasks: NewTaskRepository(db, logger),
		Analytics: NewAnalyticsRepository(db, logger),
	}
}