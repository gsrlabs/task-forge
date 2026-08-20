// internal/handler/tasks.go
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"task-forge/internal/dto"
	"task-forge/internal/repository"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Query parameter defaults
const (
	defaultListLimit  = 20
	maxListLimit     = 100
	defaultListOffset = 0
)

// TasksHandler processes task management requests.
type TasksHandler struct {
	service   service.TaskService
	validator *validator.Validator
	logger    zerolog.Logger
}

// NewTasksHandler creates an instance of TasksHandler.
func NewTasksHandler(
	service service.TaskService,
	validator *validator.Validator,
	logger zerolog.Logger,
) *TasksHandler {
	return &TasksHandler{
		service:   service,
		validator: validator,
		logger:    logger,
	}
}

// Create processes POST /api/v1/tasks — creating a new task.
// The current user must be a member of the team.
func (h *TasksHandler) Create(c *gin.Context) {
	// 1. Extract authenticated user ID from context.
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// 2. Bind and validate the request body.
	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Invalid create task request body")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Create task validation failed")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: err.Error(),
		})
		return
	}

	// 3. Create the task through the service.
	response, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(c, err, "create task",
			"user_id", userID.String(),
			"team_id", req.TeamID,
		)
		return
	}

	h.logger.Info().
		Str("user_id", userID.String()).
		Str("task_id", response.ID).
		Str("team_id", response.TeamID).
		Msg("Task created successfully")

	c.JSON(http.StatusCreated, response)
}

// List processes GET /api/v1/tasks — retrieving a filtered, paginated list.
// Query parameters:
//   - team_id (required) — UUID of the team
//   - status (optional) — one of: todo, in_progress, review, done
//   - assignee_id (optional) — UUID of the assignee
//   - limit (optional, default 20, max 100)
//   - offset (optional, default 0)
func (h *TasksHandler) List(c *gin.Context) {
	// 1. Extract authenticated user ID.
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// 2. Parse and validate query parameters.
	teamID := c.Query("team_id")
	if teamID == "" {
		h.logger.Warn().
			Str("user_id", userID.String()).
			Msg("team_id query parameter is missing")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "team_id query parameter is required",
		})
		return
	}

	// Validate team_id format early.
	if _, err := uuid.Parse(teamID); err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("team_id", teamID).
			Msg("Invalid team_id format in query")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "team_id must be a valid UUID",
		})
		return
	}

	// Optional filters.
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}

	var assigneeID *string
	if a := c.Query("assignee_id"); a != "" {
		// Validate assignee_id format early.
		if _, err := uuid.Parse(a); err != nil {
			h.logger.Warn().
				Err(err).
				Str("user_id", userID.String()).
				Str("assignee_id", a).
				Msg("Invalid assignee_id format in query")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "assignee_id must be a valid UUID",
			})
			return
		}
		assigneeID = &a
	}

	// Pagination: limit.
	limit, err := parseIntQuery(c.Query("limit"), defaultListLimit)
	if err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("limit", c.Query("limit")).
			Msg("Invalid limit query parameter")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "limit must be a non-negative integer",
		})
		return
	}
	// if limit > maxListLimit {
	// 	limit = maxListLimit
	// }
	// if limit < 0 {
	// 	limit = defaultListLimit
	// }

	// Pagination: offset.
	offset, err := parseIntQuery(c.Query("offset"), defaultListOffset)
	if err != nil || offset < 0 {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("offset", c.Query("offset")).
			Msg("Invalid offset query parameter")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "offset must be a non-negative integer",
		})
		return
	}

	// 3. Request data from the service.
	response, err := h.service.List(
		c.Request.Context(),
		userID,
		teamID,
		status,
		assigneeID,
		limit,
		offset,
	)
	if err != nil {
		h.handleServiceError(c, err, "list tasks",
			"user_id", userID.String(),
			"team_id", teamID,
		)
		return
	}

	h.logger.Debug().
		Str("user_id", userID.String()).
		Str("team_id", teamID).
		Int("total", response.Total).
		Int("returned", len(response.Tasks)).
		Msg("Tasks listed successfully")

	c.JSON(http.StatusOK, response)
}

// Update processes PUT /api/v1/tasks/:id — updating an existing task.
// The current user must be a member of the team the task belongs to.
func (h *TasksHandler) Update(c *gin.Context) {
	// 1. Extract authenticated user ID.
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// 2. Parse task ID from path parameter.
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("task_id", c.Param("id")).
			Msg("Invalid task ID in path")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid task ID",
			Details: "task ID must be a valid UUID",
		})
		return
	}

	// 3. Bind and validate the request body.
	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("task_id", taskID.String()).
			Msg("Invalid update task request body")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("task_id", taskID.String()).
			Msg("Update task validation failed")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: err.Error(),
		})
		return
	}

	// 4. Update the task through the service.
	response, err := h.service.Update(c.Request.Context(), userID, taskID, &req)
	if err != nil {
		h.handleServiceError(c, err, "update task",
			"user_id", userID.String(),
			"task_id", taskID.String(),
		)
		return
	}

	h.logger.Info().
		Str("user_id", userID.String()).
		Str("task_id", taskID.String()).
		Msg("Task updated successfully")

	c.JSON(http.StatusOK, response)
}

// GetHistory processes GET /api/v1/tasks/:id/history — task change history.
// The current user must be a member of the team the task belongs to.
func (h *TasksHandler) GetHistory(c *gin.Context) {
	// 1. Extract authenticated user ID.
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// 2. Parse task ID from path parameter.
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Str("task_id", c.Param("id")).
			Msg("Invalid task ID in path for history")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid task ID",
			Details: "task ID must be a valid UUID",
		})
		return
	}

	// 3. Request history from the service.
	response, err := h.service.GetHistory(c.Request.Context(), userID, taskID)
	if err != nil {
		h.handleServiceError(c, err, "get task history",
			"user_id", userID.String(),
			"task_id", taskID.String(),
		)
		return
	}

	h.logger.Debug().
		Str("user_id", userID.String()).
		Str("task_id", taskID.String()).
		Int("history_count", response.Total).
		Msg("Task history retrieved successfully")

	c.JSON(http.StatusOK, response)
}


// Error mapping

// handleServiceError maps service/repository errors to HTTP responses.
func (h *TasksHandler) handleServiceError(
	c *gin.Context,
	err error,
	operation string,
	logFields ...string,
) {
	// Build a log event with all contextual fields.
	event := h.logger.Warn().Err(err).Str("operation", operation)
	for i := 0; i+1 < len(logFields); i += 2 {
		event = event.Str(logFields[i], logFields[i+1])
	}

	switch {
	// 404 Not Found
	case errors.Is(err, repository.ErrTaskNotFound):
		event.Msg("Task not found")
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "task not found",
		})

	// 403 Forbidden — access control violations
	case errors.Is(err, service.ErrNotTeamMember):
		event.Msg("User is not a member of the team")
		c.JSON(http.StatusForbidden, dto.ErrorResponse{
			Error: "you are not a member of this team",
		})

	case errors.Is(err, service.ErrTaskAccessDenied):
		event.Msg("Access denied to task")
		c.JSON(http.StatusForbidden, dto.ErrorResponse{
			Error: "insufficient privileges to modify this task",
		})

	// 400 Bad Request — validation and business rule violations
	case errors.Is(err, service.ErrInvalidTeamID):
		event.Msg("Invalid team ID")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid team ID",
			Details: err.Error(),
		})

	case errors.Is(err, service.ErrInvalidTaskID):
		event.Msg("Invalid task ID")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid task ID",
			Details: err.Error(),
		})

	case errors.Is(err, service.ErrInvalidAssigneeID):
		event.Msg("Invalid assignee ID")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid assignee ID",
			Details: err.Error(),
		})

	case errors.Is(err, service.ErrInvalidStatus):
		event.Msg("Invalid task status")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid task status",
			Details: "status must be one of: todo, in_progress, review, done",
		})

	case errors.Is(err, service.ErrAssigneeNotMember):
		event.Msg("Assignee is not a team member")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "assignee must be a member of the team",
		})

	case errors.Is(err, service.ErrNoFieldsToUpdate):
		event.Msg("No fields to update")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "no fields to update",
			Details: "at least one field must be provided",
		})

	case errors.Is(err, service.ErrInvalidPagination):
		event.Msg("Invalid pagination parameters")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid pagination",
			Details: err.Error(),
		})

	case errors.Is(err, service.ErrInvalidFilter):
		event.Msg("Invalid filter parameters")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid filter",
			Details: err.Error(),
		})

	// 500 Internal Server Error — unexpected failures
	default:
		// Promote the log level to Error for unexpected failures.
		errEvent := h.logger.Error().Err(err).Str("operation", operation)
		for i := 0; i+1 < len(logFields); i += 2 {
			errEvent = errEvent.Str(logFields[i], logFields[i+1])
		}
		errEvent.Msg("Unexpected error during task operation")

		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
	}
}


// parseIntQuery parses an optional integer query parameter.
// Returns (defaultValue, nil) when the raw value is empty.
// Returns (0, error) when the value is present but not a valid integer.
func parseIntQuery(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}

	return v, nil
}