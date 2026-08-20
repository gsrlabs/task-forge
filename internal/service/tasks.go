// internal/service/tasks.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	//"task-forge/internal/cache"
	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// taskService implements TaskService.
type taskService struct {
	taskRepo     repository.TaskRepository
	teamRepo     repository.TeamRepository
	cacheService CacheService
	logger       zerolog.Logger
}

// NewTaskService creates an instance of TaskService.
func NewTaskService(
	taskRepo repository.TaskRepository,
	teamRepo repository.TeamRepository,
	cacheService CacheService,
	logger zerolog.Logger,
) TaskService {
	return &taskService{
		taskRepo:     taskRepo,
		teamRepo:     teamRepo,
		cacheService: cacheService,
		logger:       logger,
	}
}

// Create creates a new task.
func (s *taskService) Create(
	ctx context.Context,
	userID uuid.UUID,
	req *dto.CreateTaskRequest,
) (*dto.TaskResponse, error) {
	if req == nil {
		return nil, errors.New("create task request is nil")
	}

	teamID, err := uuid.Parse(req.TeamID)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTeamID, req.TeamID)
	}

	var assigneeID *uuid.UUID

	if req.AssigneeID != nil && *req.AssigneeID != "" {
		parsedAssigneeID, err := uuid.Parse(*req.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrInvalidAssigneeID,
				*req.AssigneeID,
			)
		}

		assigneeID = &parsedAssigneeID
	}

	// The service owns the membership rule.
	if err := s.ensureTeamMember(ctx, teamID, userID); err != nil {
		return nil, err
	}

	// An assignee must belong to the same team.
	if assigneeID != nil {
		if err := s.ensureTeamMember(ctx, teamID, *assigneeID); err != nil {
			return nil, fmt.Errorf(
				"%w: %w",
				ErrAssigneeNotMember,
				err,
			)
		}
	}

	task := &domain.Task{
		ID:          uuid.New(),
		TeamID:      teamID,
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		Status:      domain.TaskStatusTodo,
		AssigneeID:  assigneeID,
		CreatedBy:   userID,
	}

	auditChanges, err := json.Marshal(map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"status":      task.Status,
		"assignee_id": task.AssigneeID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal task creation audit: %w", err)
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: userID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   auditChanges,
	}

	if err := s.taskRepo.Create(ctx, task, history); err != nil {
		s.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Str("team_id", teamID.String()).
			Str("task_id", task.ID.String()).
			Msg("Failed to create task")

		return nil, fmt.Errorf("create task: %w", err)
	}

	s.logger.Info().
		Str("task_id", task.ID.String()).
		Str("team_id", teamID.String()).
		Str("user_id", userID.String()).
		Msg("Task created successfully")

	if err := s.cacheService.InvalidateTeamTasks(ctx, task.TeamID.String()); err != nil {
		s.logger.Warn().
			Err(err).
			Str("team_id", task.TeamID.String()).
			Msg("Failed to invalidate tasks cache after task creation")
	}

	response := taskToResponse(task)

	return &response, nil
}

// List returns tasks visible to the current user.
func (s *taskService) List(
	ctx context.Context,
	userID uuid.UUID,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
) (*dto.TaskListResponse, error) {
	parsedTeamID, err := uuid.Parse(teamID)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTeamID, teamID)
	}

	if limit <= 0 {
		limit = domain.DefaultTaskPagination().Limit
	}

	if offset < 0 {
		return nil, ErrInvalidPagination
	}

	if limit > 100 {
		limit = 100
	}

	if err := s.ensureTeamMember(ctx, parsedTeamID, userID); err != nil {
		return nil, err
	}

	// Checking it out Redis.
	var cachedResponse dto.TaskListResponse

	err = s.cacheService.GetTeamTasks(
		ctx,
		teamID,
		status,
		assigneeID,
		limit,
		offset,
		&cachedResponse,
	)
	if err == nil {
		s.logger.Debug().
			Str("team_id", teamID).
			Str("user_id", userID.String()).
			Msg("Returning tasks from cache")

		return &cachedResponse, nil
	}

	if !errors.Is(err, redis.Nil) {
		// A Redis error should NOT break the API.
		s.logger.Warn().
			Err(err).
			Str("team_id", teamID).
			Msg("Failed to read tasks from cache")
	}

	filter := domain.TaskFilter{
		TeamID: &parsedTeamID,
	}

	if status != nil && *status != "" {
		parsedStatus := domain.TaskStatus(*status)

		if !parsedStatus.IsValid() {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrInvalidStatus,
				*status,
			)
		}

		filter.Status = &parsedStatus
	}

	if assigneeID != nil && *assigneeID != "" {
		parsedAssigneeID, err := uuid.Parse(*assigneeID)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrInvalidAssigneeID,
				*assigneeID,
			)
		}

		filter.AssigneeID = &parsedAssigneeID
	}

	pagination, err := normalizePagination(limit, offset)
	if err != nil {
		return nil, err
	}

	result, err := s.taskRepo.List(ctx, filter, pagination)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Str("team_id", parsedTeamID.String()).
			Msg("Failed to list tasks")

		return nil, fmt.Errorf("list tasks: %w", err)
	}

	response := &dto.TaskListResponse{
		Tasks:  make([]dto.TaskResponse, 0, len(result.Tasks)),
		Total:  result.Total,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
	}

	for i := range result.Tasks {
		response.Tasks = append(
			response.Tasks,
			taskToResponse(&result.Tasks[i]),
		)
	}

	// A Redis write error should not break a successful PostgreSQL response.
	if err := s.cacheService.SetTeamTasks(
		ctx,
		teamID,
		status,
		assigneeID,
		limit,
		offset,
		response,
	); err != nil {
		s.logger.Warn().
			Err(err).
			Str("team_id", teamID).
			Msg("Failed to cache tasks")
	}

	return response, nil
}

// Update updates a task and records the changes in the audit history.
func (s *taskService) Update(
	ctx context.Context,
	userID, taskID uuid.UUID,
	req *dto.UpdateTaskRequest,
) (*dto.TaskResponse, error) {
	if req == nil {
		return nil, errors.New("update task request is nil")
	}

	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, repository.ErrTaskNotFound
		}

		return nil, fmt.Errorf("find task before update: %w", err)
	}

	// The service decides whether the user belongs to the team.
	role, err := s.teamRepo.GetUserRole(ctx, task.TeamID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamMemberNotFound) {
			return nil, ErrNotTeamMember
		}

		return nil, fmt.Errorf("get task user role: %w", err)
	}

	// Current policy:
	// all team members can modify tasks.
	//
	// If your business rules require only owner/admin,
	// replace this check with the appropriate role policy.
	if !canUpdateTask(role) {
		return nil, ErrTaskAccessDenied
	}

	update := domain.TaskUpdate{}

	changes := make(map[string]any)

	// Title
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)

		if title == "" {
			return nil, errors.New("task title cannot be empty")
		}

		if title != task.Title {
			update.Title = &title
			update.TitleSet = true

			changes["title"] = map[string]any{
				"from": task.Title,
				"to":   title,
			}
		}
	}

	// Description
	if req.Description != nil {
		currentDescription := ""

		if task.Description != nil {
			currentDescription = *task.Description
		}

		newDescription := *req.Description

		if newDescription != currentDescription {
			update.Description = req.Description
			update.DescriptionSet = true

			changes["description"] = map[string]any{
				"from": task.Description,
				"to":   req.Description,
			}
		}
	}

	// Status
	if req.Status != nil {
		newStatus := domain.TaskStatus(*req.Status)

		if !newStatus.IsValid() {
			return nil, fmt.Errorf(
				"%w: %q",
				ErrInvalidStatus,
				*req.Status,
			)
		}

		if newStatus != task.Status {
			update.Status = &newStatus
			update.StatusSet = true

			changes["status"] = map[string]any{
				"from": task.Status,
				"to":   newStatus,
			}
		}
	}

	// Assignee
	if req.AssigneeID != nil {
		var newAssigneeID *uuid.UUID

		if *req.AssigneeID != "" {
			parsedAssigneeID, err := uuid.Parse(*req.AssigneeID)
			if err != nil {
				return nil, fmt.Errorf(
					"%w: %q",
					ErrInvalidAssigneeID,
					*req.AssigneeID,
				)
			}

			newAssigneeID = &parsedAssigneeID

			if err := s.ensureTeamMember(
				ctx,
				task.TeamID,
				parsedAssigneeID,
			); err != nil {
				return nil, fmt.Errorf(
					"%w: %w",
					ErrAssigneeNotMember,
					err,
				)
			}
		}

		if !sameUUIDPointer(task.AssigneeID, newAssigneeID) {
			update.AssigneeID = newAssigneeID
			update.AssigneeIDSet = true

			changes["assignee_id"] = map[string]any{
				"from": task.AssigneeID,
				"to":   newAssigneeID,
			}
		}
	}

	// Nothing actually changed.
	if !update.HasChanges() {
		return nil, ErrNoFieldsToUpdate
	}

	auditChanges, err := json.Marshal(changes)
	if err != nil {
		return nil, fmt.Errorf("marshal task audit: %w", err)
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: userID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   auditChanges,
	}

	updatedTask, err := s.taskRepo.Update(
		ctx,
		taskID,
		userID,
		update,
		history,
	)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, repository.ErrTaskNotFound
		}

		return nil, fmt.Errorf("update task: %w", err)
	}

	s.logger.Info().
		Str("task_id", taskID.String()).
		Str("team_id", task.TeamID.String()).
		Str("user_id", userID.String()).
		Msg("Task updated successfully")

	if err := s.cacheService.InvalidateTeamTasks(
		ctx,
		updatedTask.TeamID.String(),
	); err != nil {
		s.logger.Warn().
			Err(err).
			Str("team_id", updatedTask.TeamID.String()).
			Msg("Failed to invalidate tasks cache after task update")
	}

	response := taskToResponse(updatedTask)

	return &response, nil
}

// GetHistory returns task audit history.
// The user must be a member of the team to which the task belongs.
func (s *taskService) GetHistory(
	ctx context.Context,
	userID, taskID uuid.UUID,
) (*dto.TaskHistoryResponse, error) {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, repository.ErrTaskNotFound
		}

		return nil, fmt.Errorf("find task for history: %w", err)
	}

	if err := s.ensureTeamMember(ctx, task.TeamID, userID); err != nil {
		return nil, err
	}

	history, err := s.taskRepo.GetHistory(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task history: %w", err)
	}

	response := &dto.TaskHistoryResponse{
		TaskID:  taskID.String(),
		History: make([]dto.TaskHistoryItem, 0, len(history)),
		Total:   len(history),
	}

	for _, item := range history {
		var changes map[string]any

		if len(item.Changes) > 0 {
			if err := json.Unmarshal(item.Changes, &changes); err != nil {
				return nil, fmt.Errorf(
					"unmarshal task history changes: %w",
					err,
				)
			}
		}

		response.History = append(
			response.History,
			dto.TaskHistoryItem{
				ID:             item.ID.String(),
				TaskID:         item.TaskID.String(),
				ChangedBy:      item.ChangedBy.String(),
				ChangedByEmail: item.ChangedByEmail,
				Action:         string(item.Action),
				Changes:        changes,
				ChangedAt:      item.ChangedAt.Format(timeFormat),
			},
		)
	}

	return response, nil
}

// ensureTeamMember verifies that user belongs to the team.
func (s *taskService) ensureTeamMember(
	ctx context.Context,
	teamID, userID uuid.UUID,
) error {
	isMember, err := s.teamRepo.IsTeamMember(ctx, teamID, userID)
	if err != nil {
		return fmt.Errorf("check team membership: %w", err)
	}

	if !isMember {
		return ErrNotTeamMember
	}

	return nil
}

// canUpdateTask determines whether a team member can update tasks.
//
// Current application policy allows all team members to update tasks.
// Keep this rule in the service layer so it can easily be changed later.
func canUpdateTask(role domain.TeamRole) bool {
	switch role {
	case domain.TeamRoleOwner,
		domain.TeamRoleAdmin,
		domain.TeamRoleMember:
		return true

	default:
		return false
	}
}

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

const (
	defaultTaskLimit = 20
	maxTaskLimit     = 100
)

// normalizePagination validates and normalizes pagination.
// The HTTP handler may pass zero values when query parameters are omitted.
func normalizePagination(limit, offset int) (domain.TaskPagination, error) {
	if limit == 0 {
		limit = defaultTaskLimit
	}

	if limit < 1 || limit > maxTaskLimit {
		return domain.TaskPagination{}, fmt.Errorf(
			"%w: limit must be between 1 and %d",
			ErrInvalidPagination,
			maxTaskLimit,
		)
	}

	if offset < 0 {
		return domain.TaskPagination{}, fmt.Errorf(
			"%w: offset cannot be negative",
			ErrInvalidPagination,
		)
	}

	return domain.TaskPagination{
		Limit:  limit,
		Offset: offset,
	}, nil
}

// taskToResponse converts domain Task to DTO.
func taskToResponse(task *domain.Task) dto.TaskResponse {
	var assigneeID *string

	if task.AssigneeID != nil {
		value := task.AssigneeID.String()
		assigneeID = &value
	}

	return dto.TaskResponse{
		ID:          task.ID.String(),
		TeamID:      task.TeamID.String(),
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
		AssigneeID:  assigneeID,
		CreatedBy:   task.CreatedBy.String(),
		CreatedAt:   task.CreatedAt.Format(timeFormat),
		UpdatedAt:   task.UpdatedAt.Format(timeFormat),
	}
}

// sameUUIDPointer compares nullable UUID values.
func sameUUIDPointer(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return *a == *b
}
