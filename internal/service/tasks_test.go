package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

// mockTaskRepository is a mock implementation of repository.TaskRepository.
type mockTaskRepository struct {
	mock.Mock
}

func (m *mockTaskRepository) Create(ctx context.Context, task *domain.Task, history domain.TaskHistory) error {
	args := m.Called(ctx, task, history)
	return args.Error(0)
}

func (m *mockTaskRepository) FindByID(ctx context.Context, taskID uuid.UUID) (*domain.Task, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Task), args.Error(1)
}

func (m *mockTaskRepository) List(ctx context.Context, filter domain.TaskFilter, pagination domain.TaskPagination) (*domain.TaskListResult, error) {
	args := m.Called(ctx, filter, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TaskListResult), args.Error(1)
}

func (m *mockTaskRepository) Update(ctx context.Context, taskID uuid.UUID, changedBy uuid.UUID, update domain.TaskUpdate, history domain.TaskHistory) (*domain.Task, error) {
	args := m.Called(ctx, taskID, changedBy, update, history)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Task), args.Error(1)
}

func (m *mockTaskRepository) GetHistory(ctx context.Context, taskID uuid.UUID) ([]domain.TaskHistoryWithUser, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TaskHistoryWithUser), args.Error(1)
}

func (m *mockTaskRepository) IsTeamMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

// mockCacheService is a mock implementation of TaskCacheService.
type mockCacheService struct {
	mock.Mock
}

func (m *mockCacheService) GetTeamTasks(
	ctx context.Context,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
	dest any,
) error {
	args := m.Called(ctx, teamID, status, assigneeID, limit, offset, dest)
	return args.Error(0)
}

func (m *mockCacheService) SetTeamTasks(
	ctx context.Context,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
	tasks any,
) error {
	args := m.Called(ctx, teamID, status, assigneeID, limit, offset, tasks)
	return args.Error(0)
}

func (m *mockCacheService) InvalidateTeamTasks(ctx context.Context, teamID string) error {
	args := m.Called(ctx, teamID)
	return args.Error(0)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestTaskService creates a taskService with injected mocks.
func newTestTaskService(
	taskRepo *mockTaskRepository,
	teamRepo *mockTeamRepository,
	cacheSvc *mockCacheService,
) *taskService {
	return &taskService{
		taskRepo:     taskRepo,
		teamRepo:     teamRepo,
		cacheService: cacheSvc,
		logger:       zerolog.Nop(),
	}
}

// ============================================================================
// TestTaskService
// ============================================================================

func TestTaskService_Create_Success(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID:      teamID.String(),
		Title:       "Test Task",
		Description: new("Description"),
		AssigneeID:  new(assigneeID.String()),
	}

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, assigneeID).Return(true, nil)

	taskRepo.On("Create", ctx,
		mock.MatchedBy(func(task *domain.Task) bool {
			return task.TeamID == teamID &&
				task.Title == "Test Task" &&
				task.Status == domain.TaskStatusTodo &&
				task.AssigneeID != nil &&
				*task.AssigneeID == assigneeID
		}),
		mock.MatchedBy(func(h domain.TaskHistory) bool {
			return h.Action == domain.TaskHistoryActionCreated
		}),
	).Return(nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).Return(nil)

	resp, err := svc.Create(ctx, userID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Test Task", resp.Title)
	assert.Equal(t, teamID.String(), resp.TeamID)
	assert.Equal(t, "todo", resp.Status)
	assert.NotNil(t, resp.AssigneeID)
	assert.Equal(t, assigneeID.String(), *resp.AssigneeID)

	taskRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
	cacheSvc.AssertExpectations(t)
}

func TestTaskService_Create_NilRequest(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()

	resp, err := svc.Create(ctx, userID, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Nil(t, resp)
}

func TestTaskService_Create_InvalidTeamID(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID: "not-a-uuid",
		Title:  "Task",
	}

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidTeamID)
	assert.Nil(t, resp)
}

func TestTaskService_Create_InvalidAssigneeID(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID:     teamID.String(),
		Title:      "Task",
		AssigneeID: new("not-a-uuid"),
	}

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidAssigneeID)
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(
			t,
			"IsTeamMember",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		)
	
		taskRepo.AssertNotCalled(
			t,
			"Create",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		)
}

func TestTaskService_Create_UserNotTeamMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID: teamID.String(),
		Title:  "Task",
	}

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(false, nil)

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotTeamMember)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
}

func TestTaskService_Create_AssigneeNotTeamMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID:     teamID.String(),
		Title:      "Task",
		AssigneeID: new(assigneeID.String()),
	}

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, assigneeID).Return(false, nil)

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrAssigneeNotMember)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
}

func TestTaskService_Create_RepositoryError(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID: teamID.String(),
		Title:  "Task",
	}

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	dbErr := errors.New("database error")
	taskRepo.On("Create", ctx, mock.AnythingOfType("*domain.Task"), mock.AnythingOfType("domain.TaskHistory")).
		Return(dbErr)

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create task")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	cacheSvc.AssertNotCalled(t, "InvalidateTeamTasks", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	taskRepo.AssertExpectations(t)
}

func TestTaskService_Create_CacheInvalidationFails(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()

	req := &dto.CreateTaskRequest{
		TeamID: teamID.String(),
		Title:  "Task",
	}

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)
	taskRepo.On("Create", ctx, mock.AnythingOfType("*domain.Task"), mock.AnythingOfType("domain.TaskHistory")).
		Return(nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).
		Return(errors.New("redis connection failed"))

	resp, err := svc.Create(ctx, userID, req)

	require.NoError(t, err, "Cache invalidation failure should not fail the operation")
	require.NotNil(t, resp)

	cacheSvc.AssertExpectations(t)
}

func TestTaskService_List_Success_CacheMiss(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()
	statusStr := "todo"
	limit := 20
	offset := 0

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, &statusStr, (*string)(nil), limit, offset, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(redis.Nil)

	tasks := []domain.Task{
		{
			ID:        uuid.New(),
			TeamID:    teamID,
			Title:     "Task 1",
			Status:    domain.TaskStatusTodo,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	taskRepo.On("List", ctx,
		mock.MatchedBy(func(f domain.TaskFilter) bool {
			return f.TeamID != nil && *f.TeamID == teamID && f.Status != nil && *f.Status == domain.TaskStatusTodo
		}),
		mock.MatchedBy(func(p domain.TaskPagination) bool {
			return p.Limit == limit && p.Offset == offset
		}),
	).Return(&domain.TaskListResult{Tasks: tasks, Total: 1}, nil)

	cacheSvc.On("SetTeamTasks", ctx, teamIDStr, &statusStr, (*string)(nil), limit, offset, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(nil)

	resp, err := svc.List(ctx, userID, teamIDStr, &statusStr, nil, limit, offset)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Tasks, 1)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "Task 1", resp.Tasks[0].Title)

	taskRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
	cacheSvc.AssertExpectations(t)
}

func TestTaskService_List_CacheHit(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()
	limit := 20
	offset := 0

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cachedResp := dto.TaskListResponse{
		Tasks: []dto.TaskResponse{
			{ID: uuid.New().String(), Title: "Cached Task"},
		},
		Total: 1,
	}

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), limit, offset, mock.AnythingOfType("*dto.TaskListResponse")).
		Run(func(args mock.Arguments) {
			dest := args.Get(6).(*dto.TaskListResponse)
			*dest = cachedResp
		}).
		Return(nil)

	resp, err := svc.List(ctx, userID, teamIDStr, nil, nil, limit, offset)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Tasks, 1)
	assert.Equal(t, "Cached Task", resp.Tasks[0].Title)

	taskRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
	cacheSvc.AssertNotCalled(t, "SetTeamTasks", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	cacheSvc.AssertExpectations(t)
}

func TestTaskService_List_CacheError_FallbackToDB(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()
	limit := 20
	offset := 0

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), limit, offset, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(errors.New("redis timeout"))

	taskRepo.On("List", ctx, mock.AnythingOfType("domain.TaskFilter"), mock.AnythingOfType("domain.TaskPagination")).
		Return(&domain.TaskListResult{Tasks: []domain.Task{}, Total: 0}, nil)

	cacheSvc.On("SetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), limit, offset, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(nil)

	resp, err := svc.List(ctx, userID, teamIDStr, nil, nil, limit, offset)

	require.NoError(t, err, "Cache error should not fail the operation")
	require.NotNil(t, resp)

	taskRepo.AssertExpectations(t)
	cacheSvc.AssertExpectations(t)
}

func TestTaskService_List_InvalidTeamID(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()

	resp, err := svc.List(ctx, userID, "not-a-uuid", nil, nil, 20, 0)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidTeamID)
	assert.Nil(t, resp)
}

func TestTaskService_List_InvalidStatus(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()
	invalidStatus := "invalid_status"

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, &invalidStatus, (*string)(nil), 20, 0, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(redis.Nil)

	resp, err := svc.List(ctx, userID, teamIDStr, &invalidStatus, nil, 20, 0)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidStatus)
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTaskService_List_InvalidAssigneeID(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()
	invalidAssignee := "not-a-uuid"

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, (*string)(nil), &invalidAssignee, 20, 0, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(redis.Nil)

	resp, err := svc.List(ctx, userID, teamIDStr, nil, &invalidAssignee, 20, 0)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidAssigneeID)
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTaskService_List_InvalidPagination_NegativeOffset(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)

	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()

	resp, err := svc.List(
		ctx,
		userID,
		teamIDStr,
		nil,
		nil,
		20,
		-1,
	)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidPagination)
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(
		t,
		"IsTeamMember",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)

	cacheSvc.AssertNotCalled(
		t,
		"GetTeamTasks",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)

	taskRepo.AssertNotCalled(
		t,
		"List",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)
}

func TestTaskService_List_UserNotTeamMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(false, nil)

	resp, err := svc.List(ctx, userID, teamIDStr, nil, nil, 20, 0)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotTeamMember)
	assert.Nil(t, resp)

	// Cache and repository should not be called
	cacheSvc.AssertNotCalled(t, "GetTeamTasks", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	taskRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
}

func TestTaskService_List_RepositoryError(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), 20, 0, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(redis.Nil)

	dbErr := errors.New("query timeout")
	taskRepo.On("List", ctx, mock.AnythingOfType("domain.TaskFilter"), mock.AnythingOfType("domain.TaskPagination")).
		Return(nil, dbErr)

	resp, err := svc.List(ctx, userID, teamIDStr, nil, nil, 20, 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list tasks")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
	taskRepo.AssertExpectations(t)
}

func TestTaskService_List_CacheWriteFails(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	teamID := uuid.New()
	teamIDStr := teamID.String()

	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	cacheSvc.On("GetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), 20, 0, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(redis.Nil)

	taskRepo.On("List", ctx, mock.AnythingOfType("domain.TaskFilter"), mock.AnythingOfType("domain.TaskPagination")).
		Return(&domain.TaskListResult{Tasks: []domain.Task{}, Total: 0}, nil)

	cacheSvc.On("SetTeamTasks", ctx, teamIDStr, (*string)(nil), (*string)(nil), 20, 0, mock.AnythingOfType("*dto.TaskListResponse")).
		Return(errors.New("redis write failed"))

	resp, err := svc.List(ctx, userID, teamIDStr, nil, nil, 20, 0)

	require.NoError(t, err, "Cache write failure should not fail the operation")
	require.NotNil(t, resp)

	cacheSvc.AssertExpectations(t)
}

func TestTaskService_Update_Title(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{
		ID:     taskID,
		TeamID: teamID,
		Title:  "Old Title",
		Status: domain.TaskStatusTodo,
	}

	req := &dto.UpdateTaskRequest{
		Title: new("New Title"),
	}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	updatedTask := *existingTask
	updatedTask.Title = "New Title"
	taskRepo.On("Update", ctx, taskID, userID,
		mock.MatchedBy(func(u domain.TaskUpdate) bool {
			return u.TitleSet && *u.Title == "New Title"
		}),
		mock.MatchedBy(func(h domain.TaskHistory) bool {
			return h.Action == domain.TaskHistoryActionUpdated
		}),
	).Return(&updatedTask, nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).Return(nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "New Title", resp.Title)

	taskRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
	cacheSvc.AssertExpectations(t)
}

func TestTaskService_Update_MultipleFields(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()

	existingTask := &domain.Task{
		ID:     taskID,
		TeamID: teamID,
		Title:  "Old Title",
		Status: domain.TaskStatusTodo,
	}

	newStatus := "in_progress"
	req := &dto.UpdateTaskRequest{
		Title:      new("New Title"),
		Status:     &newStatus,
		AssigneeID: new(assigneeID.String()),
	}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleOwner, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, assigneeID).Return(true, nil)

	updatedTask := *existingTask
	updatedTask.Title = "New Title"
	updatedTask.Status = domain.TaskStatusInProgress
	updatedTask.AssigneeID = &assigneeID

	taskRepo.On("Update", ctx, taskID, userID,
		mock.MatchedBy(func(u domain.TaskUpdate) bool {
			return u.TitleSet && u.StatusSet && u.AssigneeIDSet
		}),
		mock.AnythingOfType("domain.TaskHistory"),
	).Return(&updatedTask, nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).Return(nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "New Title", resp.Title)
	assert.Equal(t, "in_progress", resp.Status)
	assert.NotNil(t, resp.AssigneeID)

	taskRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
}

func TestTaskService_Update_Unassign(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()

	existingTask := &domain.Task{
		ID:         taskID,
		TeamID:     teamID,
		Title:      "Task",
		AssigneeID: &assigneeID,
	}

	req := &dto.UpdateTaskRequest{
		AssigneeID: new(""),
	}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	updatedTask := *existingTask
	updatedTask.AssigneeID = nil

	taskRepo.On("Update", ctx, taskID, userID,
		mock.MatchedBy(func(u domain.TaskUpdate) bool {
			return u.AssigneeIDSet && u.AssigneeID == nil
		}),
		mock.AnythingOfType("domain.TaskHistory"),
	).Return(&updatedTask, nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).Return(nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.AssigneeID, "Assignee should be nil after unassign")

	taskRepo.AssertExpectations(t)
}

func TestTaskService_Update_NilRequest(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()

	resp, err := svc.Update(ctx, userID, taskID, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Nil(t, resp)
}

func TestTaskService_Update_TaskNotFound(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()

	req := &dto.UpdateTaskRequest{Title: new("New")}

	taskRepo.On("FindByID", ctx, taskID).Return(nil, repository.ErrTaskNotFound)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrTaskNotFound)
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "GetUserRole", mock.Anything, mock.Anything, mock.Anything)
	taskRepo.AssertExpectations(t)
}

func TestTaskService_Update_UserNotTeamMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{Title: new("New")}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotTeamMember)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTaskService_Update_AccessDenied(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{Title: new("New")}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)

	teamRepo.On("GetUserRole", ctx, teamID, userID).
		Return(domain.TeamRole(""), nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrTaskAccessDenied)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTaskService_Update_InvalidStatus(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	invalidStatus := "invalid"
	req := &dto.UpdateTaskRequest{Status: &invalidStatus}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidStatus)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTaskService_Update_InvalidAssigneeID(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{AssigneeID: new("not-a-uuid")}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidAssigneeID)
	assert.Nil(t, resp)
}

func TestTaskService_Update_AssigneeNotMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()
	assigneeID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{AssigneeID: new(assigneeID.String())}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, assigneeID).Return(false, nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrAssigneeNotMember)
	assert.Nil(t, resp)
}

func TestTaskService_Update_EmptyTitle(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{Title: new("   ")} // Whitespace only

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.Nil(t, resp)
}

func TestTaskService_Update_NoFieldsToUpdate(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Task"}
	req := &dto.UpdateTaskRequest{}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoFieldsToUpdate)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTaskService_Update_RepositoryError(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Old"}
	req := &dto.UpdateTaskRequest{Title: new("New")}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)

	dbErr := errors.New("update failed")
	taskRepo.On("Update", ctx, taskID, userID, mock.AnythingOfType("domain.TaskUpdate"), mock.AnythingOfType("domain.TaskHistory")).
		Return(nil, dbErr)

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update task")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	cacheSvc.AssertNotCalled(t, "InvalidateTeamTasks", mock.Anything, mock.Anything)
}

func TestTaskService_Update_CacheInvalidationFails(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID, Title: "Old"}
	updatedTask := *existingTask
	updatedTask.Title = "New"

	req := &dto.UpdateTaskRequest{Title: new("New")}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("GetUserRole", ctx, teamID, userID).Return(domain.TeamRoleMember, nil)
	taskRepo.On("Update", ctx, taskID, userID, mock.AnythingOfType("domain.TaskUpdate"), mock.AnythingOfType("domain.TaskHistory")).
		Return(&updatedTask, nil)

	cacheSvc.On("InvalidateTeamTasks", ctx, teamID.String()).
		Return(errors.New("redis failed"))

	resp, err := svc.Update(ctx, userID, taskID, req)

	require.NoError(t, err, "Cache invalidation failure should not fail the operation")
	require.NotNil(t, resp)
	assert.Equal(t, "New", resp.Title)
}

func TestTaskService_GetHistory_Success(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID}

	changesJSON, _ := json.Marshal(map[string]any{"title": "Test"})
	history := []domain.TaskHistoryWithUser{
		{
			TaskHistory: domain.TaskHistory{
				ID:        uuid.New(),
				TaskID:    taskID,
				ChangedBy: userID,
				Action:    domain.TaskHistoryActionCreated,
				Changes:   changesJSON,
				ChangedAt: time.Now(),
			},
			ChangedByEmail: "user@example.com",
		},
	}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)
	taskRepo.On("GetHistory", ctx, taskID).Return(history, nil)

	resp, err := svc.GetHistory(ctx, userID, taskID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, taskID.String(), resp.TaskID)
	assert.Len(t, resp.History, 1)
	assert.Equal(t, "created", resp.History[0].Action)
	assert.Equal(t, "user@example.com", resp.History[0].ChangedByEmail)

	taskRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
}

func TestTaskService_GetHistory_TaskNotFound(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()

	taskRepo.On("FindByID", ctx, taskID).Return(nil, repository.ErrTaskNotFound)

	resp, err := svc.GetHistory(ctx, userID, taskID)

	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrTaskNotFound)
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "IsTeamMember", mock.Anything, mock.Anything, mock.Anything)
}

func TestTaskService_GetHistory_UserNotTeamMember(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(false, nil)

	resp, err := svc.GetHistory(ctx, userID, taskID)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotTeamMember)
	assert.Nil(t, resp)

	taskRepo.AssertNotCalled(t, "GetHistory", mock.Anything, mock.Anything)
}

func TestTaskService_GetHistory_RepositoryError(t *testing.T) {
	taskRepo := new(mockTaskRepository)
	teamRepo := new(mockTeamRepository)
	cacheSvc := new(mockCacheService)
	svc := newTestTaskService(taskRepo, teamRepo, cacheSvc)

	ctx := context.Background()
	userID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()

	existingTask := &domain.Task{ID: taskID, TeamID: teamID}

	taskRepo.On("FindByID", ctx, taskID).Return(existingTask, nil)
	teamRepo.On("IsTeamMember", ctx, teamID, userID).Return(true, nil)

	dbErr := errors.New("history query failed")
	taskRepo.On("GetHistory", ctx, taskID).Return(nil, dbErr)

	resp, err := svc.GetHistory(ctx, userID, taskID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get task history")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)
}
