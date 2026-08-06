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
	// Teams TeamRepository    // Добавим позже
	// Tasks TaskRepository    // Добавим позже
}

// UserRepository describes a contract for working with users.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// NewRepositories creates a container with all repositories.
func NewRepositories(db *pgxpool.Pool, logger zerolog.Logger) *Repositories {
	return &Repositories{
		Users: NewUserRepository(db, logger),
	}
}