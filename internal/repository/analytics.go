package repository

import (
	"context"
	"fmt"
	"time"

	"task-forge/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type analyticsRepository struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewAnalyticsRepository creates an instance of AnalyticsRepository.
func NewAnalyticsRepository(db *pgxpool.Pool, logger zerolog.Logger) AnalyticsRepository {
	return &analyticsRepository{
		db:     db,
		logger: logger,
	}
}

// GetTeamStats returns statistics for all teams with JOIN 3+ tables and aggregation.
//
// SQL query:
// 1. We take all teams from teams
// 2. LEFT JOIN team_members to count participants
// 3. LEFT JOIN tasks to count tasks with the status 'done' for the last N days
// 4. GROUP BY team for aggregation
func (r *analyticsRepository) GetTeamStats(
	ctx context.Context,
	days int,
) ([]domain.TeamStats, error) {

	// Calculating the start date of the period
	sinceDate := time.Now().AddDate(0, 0, -days)

	// Complex SQL query with JOIN 3 tables and aggregation
	query := `
		SELECT 
			t.id AS team_id,
			t.name AS team_name,
			COALESCE(member_counts.count, 0) AS members_count,
			COALESCE(done_tasks.count, 0) AS done_tasks_count,
			$1::timestamptz AS done_tasks_since
		FROM teams t
		
		-- A subquery for counting team members
		LEFT JOIN (
			SELECT 
				team_id,
				COUNT(*) AS count
			FROM team_members
			GROUP BY team_id
		) member_counts ON t.id = member_counts.team_id
		
		-- A subquery for calculating tasks with the 'done' status for the last N days
		LEFT JOIN (
			SELECT 
				team_id,
				COUNT(*) AS count
			FROM tasks
			WHERE status = 'done'::task_status
				AND updated_at >= $1
			GROUP BY team_id
		) done_tasks ON t.id = done_tasks.team_id
		
		ORDER BY t.name ASC
	`

	rows, err := r.db.Query(ctx, query, sinceDate)
	if err != nil {
		r.logger.Error().
			Err(err).
			Int("days", days).
			Time("since_date", sinceDate).
			Msg("Failed to execute team stats query")
		return nil, fmt.Errorf("query team stats: %w", err)
	}
	defer rows.Close()

	var stats []domain.TeamStats
	for rows.Next() {
		var stat domain.TeamStats
		err := rows.Scan(
			&stat.TeamID,
			&stat.TeamName,
			&stat.MembersCount,
			&stat.DoneTasksCount,
			&stat.DoneTasksSince,
		)
		if err != nil {
			r.logger.Error().
				Err(err).
				Msg("Failed to scan team stats row")
			return nil, fmt.Errorf("scan team stats: %w", err)
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error().
			Err(err).
			Msg("Error iterating team stats rows")
		return nil, fmt.Errorf("iterate team stats: %w", err)
	}

	// If there are no commands, we return an empty slice
	if stats == nil {
		stats = []domain.TeamStats{}
	}

	r.logger.Debug().
		Int("teams_count", len(stats)).
		Int("days_period", days).
		Msg("Team stats retrieved successfully")

	return stats, nil
}