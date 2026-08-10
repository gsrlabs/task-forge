package repository

import (
	"context"

	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Repositories aggregates all the application’s repositories.
type Repositories struct {
	Users UserRepository
	Teams TeamRepository
	// Tasks TaskRepository
}

// UserRepository describes a contract for working with users.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// TeamRepository describes the contract for working with teams.
type TeamRepository interface {
	// Create creates a command and adds the creator as the owner.
	Create(ctx context.Context, userID uuid.UUID, team *domain.Team) error
	
	// FindByID finds a command by ID.
	FindByID(ctx context.Context, teamID uuid.UUID) (*domain.Team, error)
	
	// FindByUserID returns a list of teams where the user is a member.
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.TeamWithRole, error)
	
	// AddMember adds a user to the team with the specified role.
	AddMember(ctx context.Context, member *domain.TeamMember) error
	
	// GetUserRole returns the user’s role in the team.
	// Returns ErrTeamMemberNotFound if the user is not part of the team.
	GetUserRole(ctx context.Context, teamID, userID uuid.UUID) (domain.TeamRole, error)
	
	// RemoveMember removes a user from the team.
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
}

// NewRepositories creates a container with all repositories.
func NewRepositories(db *pgxpool.Pool, logger zerolog.Logger) *Repositories {
	return &Repositories{
		Users: NewUserRepository(db, logger),
		Teams: NewTeamRepository(db, logger),
	}
}