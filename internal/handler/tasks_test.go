package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-forge/internal/dto"
	"task-forge/internal/middleware"
	"task-forge/internal/repository"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Mock TaskService
// ============================================================================

type mockTaskService struct {
	mock.Mock
}

func (m *mockTaskService) Create(
	ctx context.Context,
	userID uuid.UUID,
	req *dto.CreateTaskRequest,
) (*dto.TaskResponse, error) {
	args := m.Called(ctx, userID, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TaskResponse), args.Error(1)
}

func (m *mockTaskService) List(
	ctx context.Context,
	userID uuid.UUID,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
) (*dto.TaskListResponse, error) {
	args := m.Called(ctx, userID, teamID, status, assigneeID, limit, offset)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TaskListResponse), args.Error(1)
}

func (m *mockTaskService) Update(
	ctx context.Context,
	userID, taskID uuid.UUID,
	req *dto.UpdateTaskRequest,
) (*dto.TaskResponse, error) {
	args := m.Called(ctx, userID, taskID, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TaskResponse), args.Error(1)
}

func (m *mockTaskService) GetHistory(
	ctx context.Context,
	userID, taskID uuid.UUID,
) (*dto.TaskHistoryResponse, error) {
	args := m.Called(ctx, userID, taskID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TaskHistoryResponse), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

func newTestTasksHandler(taskSvc *mockTaskService) *TasksHandler {
	return &TasksHandler{
		service:   taskSvc,
		validator: validator.NewValidator(),
		logger:    zerolog.Nop(),
	}
}

func performTasksRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body any,
	userID uuid.UUID,
	pathParams map[string]string,
	queryParams map[string]string,
) *httptest.ResponseRecorder {
	var requestBody bytes.Buffer

	if body != nil {
		err := json.NewEncoder(&requestBody).Encode(body)
		if err != nil {
			panic(err)
		}
	}

	req := httptest.NewRequest(method, path, &requestBody)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = req

	if userID != uuid.Nil {
		middleware.SetUserIdentity(c, userID, "user@example.com")
	}

	if len(pathParams) > 0 {
		for key, value := range pathParams {
			c.Params = append(c.Params, gin.Param{Key: key, Value: value})
		}
	}

	if queryParams != nil {
		q := req.URL.Query()
		for key, value := range queryParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	handler(c)

	return w
}

func performRawTasksRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body string,
	userID uuid.UUID,
	pathParams map[string]string,
	queryParams map[string]string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		path,
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = req

	if userID != uuid.Nil {
		middleware.SetUserIdentity(c, userID, "user@example.com")
	}

	if len(pathParams) > 0 {
		for key, value := range pathParams {
			c.Params = append(c.Params, gin.Param{Key: key, Value: value})
		}
	}

	if queryParams != nil {
		q := req.URL.Query()
		for key, value := range queryParams {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	handler(c)

	return w
}

// ============================================================================
// TestTasksHandler_Create
// ============================================================================

func TestTasksHandler_Create_Success(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()
	taskID := uuid.New()
	assigneeID := uuid.New()

	req := dto.CreateTaskRequest{
		TeamID:      teamID.String(),
		Title:       "Implement authentication",
		Description: new("Add JWT support"),
		AssigneeID:  new(assigneeID.String()),
	}

	expectedResp := &dto.TaskResponse{
		ID:          taskID.String(),
		TeamID:      teamID.String(),
		Title:       "Implement authentication",
		Description: new("Add JWT support"),
		Status:      "todo",
		AssigneeID:  new(assigneeID.String()),
		CreatedBy:   userID.String(),
		CreatedAt:   "2026-08-13T10:30:00Z",
		UpdatedAt:   "2026-08-13T10:30:00Z",
	}

	taskSvc.
		On("Create", mock.Anything, userID, mock.MatchedBy(func(r *dto.CreateTaskRequest) bool {
			return r.TeamID == req.TeamID &&
				r.Title == req.Title &&
				r.Description != nil && *r.Description == "Add JWT support"
		})).
		Return(expectedResp, nil).
		Once()

	w := performTasksRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/tasks",
		req,
		userID,
		nil,
		nil,
	)

	require.Equal(t, http.StatusCreated, w.Code)

	var response dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, taskID.String(), response.ID)
	assert.Equal(t, teamID.String(), response.TeamID)
	assert.Equal(t, "Implement authentication", response.Title)
	assert.Equal(t, "todo", response.Status)

	require.True(t, taskSvc.AssertExpectations(t))
}

func TestTasksHandler_Create_Unauthorized(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	teamID := uuid.New()
	req := dto.CreateTaskRequest{
		TeamID: teamID.String(),
		Title:  "Test Task",
	}

	w := performTasksRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/tasks",
		req,
		uuid.Nil, // Not authenticated
		nil,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	taskSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_Create_InvalidJSON(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	w := performRawTasksRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/tasks",
		"{invalid json",
		userID,
		nil,
		nil,
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)

	taskSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_Create_ValidationError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	tests := []struct {
		name string
		req  dto.CreateTaskRequest
	}{
		{
			name: "missing team_id",
			req: dto.CreateTaskRequest{
				TeamID: "",
				Title:  "Test Task",
			},
		},
		{
			name: "invalid team_id UUID",
			req: dto.CreateTaskRequest{
				TeamID: "not-a-uuid",
				Title:  "Test Task",
			},
		},
		{
			name: "missing title",
			req: dto.CreateTaskRequest{
				TeamID: uuid.New().String(),
				Title:  "",
			},
		},
		{
			name: "invalid assignee_id UUID",
			req: dto.CreateTaskRequest{
				TeamID:     uuid.New().String(),
				Title:      "Test Task",
				AssigneeID: new("not-a-uuid"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performTasksRequest(
				handler.Create,
				http.MethodPost,
				"/api/v1/tasks",
				tt.req,
				userID,
				nil,
				nil,
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, "validation failed", response.Error)
			assert.NotEmpty(t, response.Details)

			taskSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestTasksHandler_Create_ServiceError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	req := dto.CreateTaskRequest{
		TeamID: teamID.String(),
		Title:  "Test Task",
	}

	dbErr := errors.New("database error")

	taskSvc.
		On("Create", mock.Anything, userID, mock.Anything).
		Return(nil, dbErr).
		Once()

	w := performTasksRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/tasks",
		req,
		userID,
		nil,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "internal server error", response.Error)

	require.True(t, taskSvc.AssertExpectations(t))
}

// ============================================================================
// TestTasksHandler_List
// ============================================================================

func TestTasksHandler_List_Success_WithFilters(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()
	taskID := uuid.New()

	status := "todo"

	expectedResp := &dto.TaskListResponse{
		Tasks: []dto.TaskResponse{
			{
				ID:     taskID.String(),
				TeamID: teamID.String(),
				Title:  "Test Task",
				Status: "todo",
			},
		},
		Total:  1,
		Limit:  20,
		Offset: 0,
	}

	taskSvc.
		On("List", mock.Anything, userID, teamID.String(), &status, new(assigneeID.String()), 20, 0).
		Return(expectedResp, nil).
		Once()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id":     teamID.String(),
			"status":      "todo",
			"assignee_id": assigneeID.String(),
			"limit":       "20",
			"offset":      "0",
		},
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TaskListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Len(t, response.Tasks, 1)
	assert.Equal(t, 1, response.Total)
	assert.Equal(t, 20, response.Limit)
	assert.Equal(t, 0, response.Offset)

	require.True(t, taskSvc.AssertExpectations(t))
}

func TestTasksHandler_List_Success_DefaultPagination(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	expectedResp := &dto.TaskListResponse{
		Tasks:  []dto.TaskResponse{},
		Total:  0,
		Limit:  20,
		Offset: 0,
	}

	taskSvc.
		On("List", mock.Anything, userID, teamID.String(), (*string)(nil), (*string)(nil), 20, 0).
		Return(expectedResp, nil).
		Once()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": teamID.String(),
		},
	)

	require.Equal(t, http.StatusOK, w.Code)

	require.True(t, taskSvc.AssertExpectations(t))
}

func TestTasksHandler_List_MissingTeamID(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		nil, // No query params
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "team_id query parameter is required")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_InvalidTeamID(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": "not-a-uuid",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "team_id must be a valid UUID")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_InvalidAssigneeID(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id":     teamID.String(),
			"assignee_id": "not-a-uuid",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "assignee_id must be a valid UUID")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_InvalidLimit(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": teamID.String(),
			"limit":   "not-a-number",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "limit must be a non-negative integer")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_InvalidOffset(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": teamID.String(),
			"offset":  "not-a-number",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "offset must be a non-negative integer")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_NegativeOffset(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": teamID.String(),
			"offset":  "-5",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "offset must be a non-negative integer")

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_Unauthorized(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	teamID := uuid.New()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		uuid.Nil,
		nil,
		map[string]string{
			"team_id": teamID.String(),
		},
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	taskSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_List_ServiceError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	teamID := uuid.New()

	dbErr := errors.New("query timeout")

	taskSvc.
		On("List", mock.Anything, userID, teamID.String(), mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, dbErr).
		Once()

	w := performTasksRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/tasks",
		nil,
		userID,
		nil,
		map[string]string{
			"team_id": teamID.String(),
		},
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "internal server error", response.Error)

	require.True(t, taskSvc.AssertExpectations(t))
}

// ============================================================================
// TestTasksHandler_Update
// ============================================================================

func TestTasksHandler_Update_Success(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	newStatus := "in_progress"
	req := dto.UpdateTaskRequest{
		Title:  new("Updated Title"),
		Status: &newStatus,
	}

	expectedResp := &dto.TaskResponse{
		ID:     taskID.String(),
		TeamID: teamID.String(),
		Title:  "Updated Title",
		Status: "in_progress",
	}

	taskSvc.
		On("Update", mock.Anything, userID, taskID, mock.MatchedBy(func(r *dto.UpdateTaskRequest) bool {
			return r.Title != nil && *r.Title == "Updated Title" &&
				r.Status != nil && *r.Status == "in_progress"
		})).
		Return(expectedResp, nil).
		Once()

	w := performTasksRequest(
		handler.Update,
		http.MethodPut,
		"/api/v1/tasks/"+taskID.String(),
		req,
		userID,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "Updated Title", response.Title)
	assert.Equal(t, "in_progress", response.Status)

	require.True(t, taskSvc.AssertExpectations(t))
}

func TestTasksHandler_Update_InvalidTaskID(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	req := dto.UpdateTaskRequest{
		Title: new("Updated Title"),
	}

	w := performTasksRequest(
		handler.Update,
		http.MethodPut,
		"/api/v1/tasks/not-a-uuid",
		req,
		userID,
		map[string]string{"id": "not-a-uuid"},
		nil,
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid task ID", response.Error)
	assert.Contains(t, response.Details, "task ID must be a valid UUID")

	taskSvc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_Update_InvalidJSON(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()

	w := performRawTasksRequest(
		handler.Update,
		http.MethodPut,
		"/api/v1/tasks/"+taskID.String(),
		"{invalid json",
		userID,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)

	taskSvc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_Update_ValidationError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()

	tests := []struct {
		name string
		req  dto.UpdateTaskRequest
	}{
		{
			name: "invalid status",
			req: dto.UpdateTaskRequest{
				Status: new("invalid_status"),
			},
		},
		{
			name: "invalid assignee_id UUID",
			req: dto.UpdateTaskRequest{
				AssigneeID: new("not-a-uuid"),
			},
		},
		{
			name: "title too long",
			req: dto.UpdateTaskRequest{
				Title: new(string(make([]byte, 256))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performTasksRequest(
				handler.Update,
				http.MethodPut,
				"/api/v1/tasks/"+taskID.String(),
				tt.req,
				userID,
				map[string]string{"id": taskID.String()},
				nil,
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, "validation failed", response.Error)

			taskSvc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestTasksHandler_Update_Unauthorized(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	taskID := uuid.New()

	req := dto.UpdateTaskRequest{
		Title: new("Updated Title"),
	}

	w := performTasksRequest(
		handler.Update,
		http.MethodPut,
		"/api/v1/tasks/"+taskID.String(),
		req,
		uuid.Nil,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	taskSvc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_Update_ServiceError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()

	req := dto.UpdateTaskRequest{
		Title: new("Updated Title"),
	}

	dbErr := errors.New("update failed")

	taskSvc.
		On("Update", mock.Anything, userID, taskID, mock.Anything).
		Return(nil, dbErr).
		Once()

	w := performTasksRequest(
		handler.Update,
		http.MethodPut,
		"/api/v1/tasks/"+taskID.String(),
		req,
		userID,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "internal server error", response.Error)

	require.True(t, taskSvc.AssertExpectations(t))
}

// ============================================================================
// TestTasksHandler_GetHistory
// ============================================================================

func TestTasksHandler_GetHistory_Success(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()
	historyID := uuid.New()

	expectedResp := &dto.TaskHistoryResponse{
		TaskID: taskID.String(),
		History: []dto.TaskHistoryItem{
			{
				ID:             historyID.String(),
				TaskID:         taskID.String(),
				ChangedBy:      userID.String(),
				ChangedByEmail: "user@example.com",
				Action:         "created",
				Changes:        map[string]any{"title": "Test Task"},
				ChangedAt:      "2026-08-13T10:30:00Z",
			},
		},
		Total: 1,
	}

	taskSvc.
		On("GetHistory", mock.Anything, userID, taskID).
		Return(expectedResp, nil).
		Once()

	w := performTasksRequest(
		handler.GetHistory,
		http.MethodGet,
		"/api/v1/tasks/"+taskID.String()+"/history",
		nil,
		userID,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TaskHistoryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, taskID.String(), response.TaskID)
	assert.Len(t, response.History, 1)
	assert.Equal(t, 1, response.Total)
	assert.Equal(t, "created", response.History[0].Action)

	require.True(t, taskSvc.AssertExpectations(t))
}

func TestTasksHandler_GetHistory_InvalidTaskID(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()

	w := performTasksRequest(
		handler.GetHistory,
		http.MethodGet,
		"/api/v1/tasks/not-a-uuid/history",
		nil,
		userID,
		map[string]string{"id": "not-a-uuid"},
		nil,
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid task ID", response.Error)
	assert.Contains(t, response.Details, "task ID must be a valid UUID")

	taskSvc.AssertNotCalled(t, "GetHistory", mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_GetHistory_Unauthorized(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	taskID := uuid.New()

	w := performTasksRequest(
		handler.GetHistory,
		http.MethodGet,
		"/api/v1/tasks/"+taskID.String()+"/history",
		nil,
		uuid.Nil,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	taskSvc.AssertNotCalled(t, "GetHistory", mock.Anything, mock.Anything, mock.Anything)
}

func TestTasksHandler_GetHistory_ServiceError(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	userID := uuid.New()
	taskID := uuid.New()

	dbErr := errors.New("query failed")

	taskSvc.
		On("GetHistory", mock.Anything, userID, taskID).
		Return(nil, dbErr).
		Once()

	w := performTasksRequest(
		handler.GetHistory,
		http.MethodGet,
		"/api/v1/tasks/"+taskID.String()+"/history",
		nil,
		userID,
		map[string]string{"id": taskID.String()},
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "internal server error", response.Error)

	require.True(t, taskSvc.AssertExpectations(t))
}

// ============================================================================
// TestTasksHandler_HandleServiceError
// ============================================================================

func TestTasksHandler_HandleServiceError_AllCases(t *testing.T) {
	taskSvc := new(mockTaskService)
	handler := newTestTasksHandler(taskSvc)

	teamID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "ErrTaskNotFound",
			err:            repository.ErrTaskNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "task not found",
		},
		{
			name:           "ErrNotTeamMember",
			err:            service.ErrNotTeamMember,
			expectedStatus: http.StatusForbidden,
			expectedError:  "you are not a member of this team",
		},
		{
			name:           "ErrTaskAccessDenied",
			err:            service.ErrTaskAccessDenied,
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient privileges to modify this task",
		},
		{
			name:           "ErrInvalidTeamID",
			err:            service.ErrInvalidTeamID,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid team ID",
		},
		{
			name:           "ErrInvalidTaskID",
			err:            service.ErrInvalidTaskID,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid task ID",
		},
		{
			name:           "ErrInvalidAssigneeID",
			err:            service.ErrInvalidAssigneeID,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid assignee ID",
		},
		{
			name:           "ErrInvalidStatus",
			err:            service.ErrInvalidStatus,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid task status",
		},
		{
			name:           "ErrAssigneeNotMember",
			err:            service.ErrAssigneeNotMember,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "assignee must be a member of the team",
		},
		{
			name:           "ErrNoFieldsToUpdate",
			err:            service.ErrNoFieldsToUpdate,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "no fields to update",
		},
		{
			name:           "ErrInvalidPagination",
			err:            service.ErrInvalidPagination,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid pagination",
		},
		{
			name:           "ErrInvalidFilter",
			err:            service.ErrInvalidFilter,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid filter",
		},
		{
			name:           "Other error",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
			c.Request = req

			handler.handleServiceError(
				c,
				tt.err,
				"test operation",
				"user_id", userID.String(),
				"team_id", teamID.String(),
				"task_id", taskID.String(),
			)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, tt.expectedError, response.Error)
		})
	}
}

// ============================================================================
// TestParseIntQuery
// ============================================================================

func TestParseIntQuery(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		defaultValue int
		expected     int
		expectError  bool
	}{
		{
			name:         "empty string returns default",
			raw:          "",
			defaultValue: 20,
			expected:     20,
			expectError:  false,
		},
		{
			name:         "valid positive integer",
			raw:          "50",
			defaultValue: 20,
			expected:     50,
			expectError:  false,
		},
		{
			name:         "zero is valid",
			raw:          "0",
			defaultValue: 20,
			expected:     0,
			expectError:  false,
		},
		{
			name:         "negative integer",
			raw:          "-5",
			defaultValue: 20,
			expected:     -5,
			expectError:  false,
		},
		{
			name:         "invalid string",
			raw:          "not-a-number",
			defaultValue: 20,
			expected:     0,
			expectError:  true,
		},
		{
			name:         "float is invalid",
			raw:          "10.5",
			defaultValue: 20,
			expected:     0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIntQuery(tt.raw, tt.defaultValue)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
