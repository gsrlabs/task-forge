// internal/repository/teams.go
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

type teamRepository struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewTeamRepository creates an instance of TeamRepository.
func NewTeamRepository(db *pgxpool.Pool, logger zerolog.Logger) TeamRepository {
	return &teamRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a team and adds the creator as an owner to team_members.
// Uses a transaction to ensure atomicity.
func (r *teamRepository) Create(ctx context.Context, userID uuid.UUID, team *domain.Team) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO teams (id, name, created_by)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at
	`

	err = tx.QueryRow(ctx, query, team.ID, team.Name, userID).Scan(
		&team.CreatedAt,
		&team.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert team: %w", err)
	}

	// Add the creator as owner
	memberQuery := `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
	`

	_, err = tx.Exec(ctx, memberQuery, team.ID, userID, domain.TeamRoleOwner)
	if err != nil {
		return fmt.Errorf("insert team member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	r.logger.Debug().
		Str("team_id", team.ID.String()).
		Str("team_name", team.Name).
		Str("owner_id", userID.String()).
		Msg("Team created successfully")

	return nil
}

// FindByID finds a command by ID.
func (r *teamRepository) FindByID(ctx context.Context, teamID uuid.UUID) (*domain.Team, error) {
	query := `
		SELECT id, name, created_by, created_at, updated_at
		FROM teams
		WHERE id = $1
	`

	team := &domain.Team{}
	err := r.db.QueryRow(ctx, query, teamID).Scan(
		&team.ID,
		&team.Name,
		&team.CreatedBy,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("find team by ID: %w", err)
	}

	return team, nil
}

// FindByUserID returns a list of teams where the user is a member.
func (r *teamRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.TeamWithRole, error) {
	query := `
		SELECT 
			t.id,
			t.name,
			t.created_by,
			t.created_at,
			t.updated_at,
			tm.role
		FROM teams t
		INNER JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query teams by user ID: %w", err)
	}
	defer rows.Close()

	var teams []domain.TeamWithRole
	for rows.Next() {
		var team domain.TeamWithRole
		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.CreatedBy,
			&team.CreatedAt,
			&team.UpdatedAt,
			&team.UserRole,
		)
		if err != nil {
			return nil, fmt.Errorf("scan team row: %w", err)
		}
		teams = append(teams, team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return teams, nil
}

// AddMember adds a user to the team with the specified role.
func (r *teamRepository) AddMember(ctx context.Context, member *domain.TeamMember) error {
	query := `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.Exec(ctx, query, member.TeamID, member.UserID, member.Role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return ErrTeamMemberExists
		}
		return fmt.Errorf("add team member: %w", err)
	}

	r.logger.Debug().
		Str("team_id", member.TeamID.String()).
		Str("user_id", member.UserID.String()).
		Str("role", string(member.Role)).
		Msg("Team member added")

	return nil
}

// GetUserRole returns the user’s role in the team.
func (r *teamRepository) GetUserRole(ctx context.Context, teamID, userID uuid.UUID) (domain.TeamRole, error) {
	query := `
		SELECT role
		FROM team_members
		WHERE team_id = $1 AND user_id = $2
	`

	var role domain.TeamRole
	err := r.db.QueryRow(ctx, query, teamID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrTeamMemberNotFound
		}
		return "", fmt.Errorf("get user role: %w", err)
	}

	return role, nil
}

// RemoveMember removes a user from the team.
func (r *teamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	query := `
		DELETE FROM team_members
		WHERE team_id = $1 AND user_id = $2
	`

	result, err := r.db.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrTeamMemberNotFound
	}

	r.logger.Debug().
		Str("team_id", teamID.String()).
		Str("user_id", userID.String()).
		Msg("Team member removed")

	return nil
}

// IsTeamMember checks whether the relationship exists in PostgreSQL.
func (r *teamRepository) IsTeamMember(
	ctx context.Context,
	teamID uuid.UUID,
	userID uuid.UUID,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM team_members
			WHERE team_id = $1
				AND user_id = $2
		)
	`
	
	var exists bool

	if err := r.db.QueryRow(
		ctx,
		query,
		teamID,
		userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf(
			"check team membership: %w",
			err,
		)
	}

	return exists, nil
}