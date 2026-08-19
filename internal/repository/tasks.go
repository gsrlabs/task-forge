// internal/repository/tasks.go
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type taskRepository struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewTaskRepository creates a new task repository.
func NewTaskRepository(
	db *pgxpool.Pool,
	logger zerolog.Logger,
) TaskRepository {
	return &taskRepository{
		db:     db,
		logger: logger,
	}
}

// Create persists a task and its initial history record atomically.
func (r *taskRepository) Create(
	ctx context.Context,
	task *domain.Task,
	history domain.TaskHistory,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin task creation transaction: %w", err)
	}

	defer func() {
    if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
        r.logger.Printf("failed to rollback transaction: %v", err)
    }
	}()

	const insertTaskQuery = `
		INSERT INTO tasks (
			id,
			team_id,
			title,
			description,
			status,
			assignee_id,
			created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	err = tx.QueryRow(
		ctx,
		insertTaskQuery,
		task.ID,
		task.TeamID,
		task.Title,
		task.Description,
		task.Status,
		task.AssigneeID,
		task.CreatedBy,
	).Scan(
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	if err := insertTaskHistory(
		ctx,
		tx,
		history,
	); err != nil {
		return fmt.Errorf("insert task creation history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit task creation transaction: %w", err)
	}

	r.logger.Debug().
		Str("task_id", task.ID.String()).
		Str("team_id", task.TeamID.String()).
		Msg("Task created successfully")

	return nil
}

// FindByID returns a task by ID.
func (r *taskRepository) FindByID(
	ctx context.Context,
	taskID uuid.UUID,
) (*domain.Task, error) {
	const query = `
		SELECT
			id,
			team_id,
			title,
			description,
			status,
			assignee_id,
			created_by,
			created_at,
			updated_at
		FROM tasks
		WHERE id = $1
	`

	var task domain.Task

	err := r.db.QueryRow(ctx, query, taskID).Scan(
		&task.ID,
		&task.TeamID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.AssigneeID,
		&task.CreatedBy,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}

		return nil, fmt.Errorf("find task %s: %w", taskID, err)
	}

	return &task, nil
}

// List returns tasks according to an already validated filter and pagination.
func (r *taskRepository) List(
	ctx context.Context,
	filter domain.TaskFilter,
	pagination domain.TaskPagination,
) (*domain.TaskListResult, error) {
	const countQuery = `
		SELECT COUNT(*)
		FROM tasks
		WHERE team_id = $1
			AND ($2::task_status IS NULL OR status = $2)
			AND ($3::uuid IS NULL OR assignee_id = $3)
	`

	var total int

	err := r.db.QueryRow(
		ctx,
		countQuery,
		filter.TeamID,
		nullableTaskStatus(filter.Status),
		filter.AssigneeID,
	).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count tasks: %w", err)
	}

	const listQuery = `
		SELECT
			id,
			team_id,
			title,
			description,
			status,
			assignee_id,
			created_by,
			created_at,
			updated_at
		FROM tasks
		WHERE team_id = $1
			AND ($2::task_status IS NULL OR status = $2)
			AND ($3::uuid IS NULL OR assignee_id = $3)
		ORDER BY created_at DESC, id DESC
		LIMIT $4
		OFFSET $5
	`

	rows, err := r.db.Query(
		ctx,
		listQuery,
		filter.TeamID,
		nullableTaskStatus(filter.Status),
		filter.AssigneeID,
		pagination.Limit,
		pagination.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0, pagination.Limit)

	for rows.Next() {
		var task domain.Task

		if err := rows.Scan(
			&task.ID,
			&task.TeamID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.AssigneeID,
			&task.CreatedBy,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return &domain.TaskListResult{
		Tasks: tasks,
		Total: total,
	}, nil
}

// Update persists a prepared task update and its audit record atomically.
func (r *taskRepository) Update(
	ctx context.Context,
	taskID uuid.UUID,
	changedBy uuid.UUID,
	update domain.TaskUpdate,
	history domain.TaskHistory,
) (*domain.Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin task update transaction: %w", err)
	}
	defer func() {
    if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
        r.logger.Printf("failed to rollback transaction: %v", err)
    }
	}()

	const query = `
		UPDATE tasks
		SET
			title = CASE
				WHEN $2 THEN $3
				ELSE title
			END,

			description = CASE
				WHEN $4 THEN $5
				ELSE description
			END,

			status = CASE
				WHEN $6 THEN $7
				ELSE status
			END,

			assignee_id = CASE
				WHEN $8 THEN $9
				ELSE assignee_id
			END

		WHERE id = $1

		RETURNING
			id,
			team_id,
			title,
			description,
			status,
			assignee_id,
			created_by,
			created_at,
			updated_at
	`

	var task domain.Task

	err = tx.QueryRow(
		ctx,
		query,
		taskID,

		update.TitleSet,
		update.Title,

		update.DescriptionSet,
		update.Description,

		update.StatusSet,
		update.Status,

		update.AssigneeIDSet,
		update.AssigneeID,
	).Scan(
		&task.ID,
		&task.TeamID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.AssigneeID,
		&task.CreatedBy,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}

		return nil, fmt.Errorf("update task %s: %w", taskID, err)
	}

	if err := insertTaskHistory(
		ctx,
		tx,
		history,
	); err != nil {
		return nil, fmt.Errorf("insert task update history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit task update transaction: %w", err)
	}

	r.logger.Debug().
		Str("task_id", task.ID.String()).
		Str("changed_by", changedBy.String()).
		Msg("Task updated successfully")

	return &task, nil
}

// GetHistory returns task history together with the email of the user
func (r *taskRepository) GetHistory(
	ctx context.Context,
	taskID uuid.UUID,
) ([]domain.TaskHistoryWithUser, error) {
	const query = `
		SELECT
			th.id,
			th.task_id,
			th.changed_by,
			u.email,
			th.action,
			th.changes,
			th.changed_at
		FROM task_history th
		INNER JOIN users u
			ON u.id = th.changed_by
		WHERE th.task_id = $1
		ORDER BY th.changed_at DESC, th.id DESC
	`

	rows, err := r.db.Query(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task history: %w", err)
	}
	defer rows.Close()

	history := make([]domain.TaskHistoryWithUser, 0)

	for rows.Next() {
		var item domain.TaskHistoryWithUser

		if err := rows.Scan(
			&item.ID,
			&item.TaskID,
			&item.ChangedBy,
			&item.ChangedByEmail,
			&item.Action,
			&item.Changes,
			&item.ChangedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task history: %w", err)
		}

		history = append(history, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task history: %w", err)
	}

	return history, nil
}

// IsTeamMember checks whether the relationship exists in PostgreSQL.
func (r *taskRepository) IsTeamMember(
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

// insertTaskHistory persists an already prepared history record.
func insertTaskHistory(
	ctx context.Context,
	tx pgx.Tx,
	history domain.TaskHistory,
) error {
	const query = `
        INSERT INTO task_history (
            id,
            task_id,
            changed_by,
            action,
            changes
        )
        VALUES ($1, $2, $3, $4, $5)
    `

	changes := history.Changes
	if len(changes) == 0 {
		changes = json.RawMessage(`{}`)
	}

	_, err := tx.Exec(
		ctx,
		query,
		history.ID,
		history.TaskID,
		history.ChangedBy,
		history.Action,
		changes,
	)
	if err != nil {
		return fmt.Errorf("insert task history: %w", err)
	}

	return nil
}

// nullableTaskStatus converts a status pointer to a value suitable
func nullableTaskStatus(
	status *domain.TaskStatus,
) any {
	if status == nil {
		return nil
	}

	return *status
}
