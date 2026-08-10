// internal/repository/user.go
package repository

import (
	"context"
	"errors"
	"fmt"

	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type userRepository struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewUserRepository creates an instance of UserRepository.
func NewUserRepository(db *pgxpool.Pool, logger zerolog.Logger) UserRepository {
	return &userRepository{
		db:     db,
		logger: logger,
	}
}

// Create saves a new user to the database.
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
	).Scan(
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		// Handling the email uniqueness error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			r.logger.Warn().
				Str("email", user.Email).
				Msg("Attempt to create user with existing email")
			return ErrUserAlreadyExists
		}

		r.logger.Error().
			Err(err).
			Str("email", user.Email).
			Msg("Failed to create user")
		return fmt.Errorf("create user: %w", err)
	}

	r.logger.Debug().
		Str("user_id", user.ID.String()).
		Str("email", user.Email).
		Msg("User created successfully")

	return nil
}

// FindByEmail finds a user by email (for authentication).
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		r.logger.Error().
			Err(err).
			Str("email", email).
			Msg("Failed to find user by email")
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

// FindByID finds a user by ID.
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		r.logger.Error().
			Err(err).
			Str("user_id", id.String()).
			Msg("Failed to find user by ID")
		return nil, fmt.Errorf("find user by ID: %w", err)
	}

	return user, nil
}