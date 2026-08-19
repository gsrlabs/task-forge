//go:build integration
// +build integration

// internal/repository/tasks_integration_test.go
package repository

import (
	"encoding/json"
	"testing"
	"time"

	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepository_Create_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "Test Task",
		Status:    domain.TaskStatusTodo,
		CreatedBy: owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{"title": "Test Task"}`),
	}

	assert.True(t, task.CreatedAt.IsZero())
	assert.True(t, task.UpdatedAt.IsZero())

	err := env.taskRepo.Create(env.ctx, task, history)
	require.NoError(t, err, "Create should succeed")

	assert.False(t, task.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, task.UpdatedAt.IsZero(), "UpdatedAt should be set")

	found, err := env.taskRepo.FindByID(env.ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, found.ID)
	assert.Equal(t, "Test Task", found.Title)
	assert.Equal(t, domain.TaskStatusTodo, found.Status)
}

func TestTaskRepository_Create_WithAssignee(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	task := &domain.Task{
		ID:         uuid.New(),
		TeamID:     team.ID,
		Title:      "Assigned Task",
		Status:     domain.TaskStatusTodo,
		AssigneeID: &assignee.ID,
		CreatedBy:  owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{"title": "Assigned Task"}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.NoError(t, err)

	found, err := env.taskRepo.FindByID(env.ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, found.AssigneeID)
	assert.Equal(t, assignee.ID, *found.AssigneeID)
}

func TestTaskRepository_Create_TransactionAtomicity(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "Atomic Task",
		Status:    domain.TaskStatusTodo,
		CreatedBy: owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.NoError(t, err)

	foundTask, err := env.taskRepo.FindByID(env.ctx, task.ID)
	require.NoError(t, err, "Task should exist")
	assert.NotNil(t, foundTask)

	histories, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err, "History should exist")
	require.Len(t, histories, 1, "Should have exactly one history record")
	assert.Equal(t, domain.TaskHistoryActionCreated, histories[0].Action)
}

func TestTaskRepository_Create_CreatorNotTeamMember(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "Invalid Task",
		Status:    domain.TaskStatusTodo,
		CreatedBy: nonMember.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: nonMember.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.Error(t, err, "Should fail with FK violation")
}

func TestTaskRepository_Create_AssigneeNotTeamMember(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:         uuid.New(),
		TeamID:     team.ID,
		Title:      "Invalid Assignee Task",
		Status:     domain.TaskStatusTodo,
		AssigneeID: &nonMember.ID,
		CreatedBy:  owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.Error(t, err, "Should fail with FK violation")
}

func TestTaskRepository_Create_EmptyTitle(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "",
		Status:    domain.TaskStatusTodo,
		CreatedBy: owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.Error(t, err, "Should fail with CHECK violation on empty title")
}

func TestTaskRepository_Create_WhitespaceTitle(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "   ",
		Status:    domain.TaskStatusTodo,
		CreatedBy: owner.ID,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   json.RawMessage(`{}`),
	}

	err := env.taskRepo.Create(env.ctx, task, history)
	require.Error(t, err, "Should fail: whitespace-only title violates CHECK")
}

func TestTaskRepository_FindByID_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	created, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Find Me", domain.TaskStatusTodo, nil)

	found, err := env.taskRepo.FindByID(env.ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Find Me", found.Title)
	assert.Equal(t, team.ID, found.TeamID)
	assert.Equal(t, owner.ID, found.CreatedBy)
	assert.Equal(t, domain.TaskStatusTodo, found.Status)
	assert.False(t, found.CreatedAt.IsZero())
	assert.False(t, found.UpdatedAt.IsZero())
}

func TestTaskRepository_FindByID_NotFound(t *testing.T) {
	env := setupPostgres(t)

	nonExistentID := uuid.New()

	found, err := env.taskRepo.FindByID(env.ctx, nonExistentID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTaskNotFound)
	assert.Nil(t, found)
}

func TestTaskRepository_FindByID_NilUUID(t *testing.T) {
	env := setupPostgres(t)

	found, err := env.taskRepo.FindByID(env.ctx, uuid.Nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTaskNotFound)
	assert.Nil(t, found)
}

func TestTaskRepository_List_Empty(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Empty Team")

	filter := domain.TaskFilter{TeamID: &team.ID}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotNil(t, result.Tasks, "Tasks should be empty slice, not nil")
	assert.Empty(t, result.Tasks)
	assert.Equal(t, 0, result.Total)
}

func TestTaskRepository_List_WithTasks(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	for i := range 5 {
		env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
			"Task "+string(rune('A'+i)), domain.TaskStatusTodo, nil)
	}

	filter := domain.TaskFilter{TeamID: &team.ID}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)

	assert.Len(t, result.Tasks, 5)
	assert.Equal(t, 5, result.Total)
}

func TestTaskRepository_List_FilterByStatus(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Todo 1", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Todo 2", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Done 1", domain.TaskStatusDone, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "In Progress", domain.TaskStatusInProgress, nil)

	todoStatus := domain.TaskStatusTodo
	filter := domain.TaskFilter{TeamID: &team.ID, Status: &todoStatus}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Total, "Should find exactly 2 todo tasks")
	for _, task := range result.Tasks {
		assert.Equal(t, domain.TaskStatusTodo, task.Status)
	}
}

func TestTaskRepository_List_FilterByAssignee(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee1 := env.createTestUser(t, randomEmail())
	assignee2 := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	env.addTeamMember(t, team.ID, assignee1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, assignee2.ID, domain.TeamRoleMember)

	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Task 1", domain.TaskStatusTodo, &assignee1.ID)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Task 2", domain.TaskStatusTodo, &assignee1.ID)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Task 3", domain.TaskStatusTodo, &assignee2.ID)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Unassigned", domain.TaskStatusTodo, nil)

	filter := domain.TaskFilter{TeamID: &team.ID, AssigneeID: &assignee1.ID}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Total)
	for _, task := range result.Tasks {
		require.NotNil(t, task.AssigneeID)
		assert.Equal(t, assignee1.ID, *task.AssigneeID)
	}
}

func TestTaskRepository_List_CombinedFilters(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Todo 1", domain.TaskStatusTodo, &assignee.ID)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Todo 2", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Done 1", domain.TaskStatusDone, &assignee.ID)

	todoStatus := domain.TaskStatusTodo
	filter := domain.TaskFilter{
		TeamID:     &team.ID,
		Status:     &todoStatus,
		AssigneeID: &assignee.ID,
	}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Total, "Should find exactly 1 task matching both filters")
}

func TestTaskRepository_List_Pagination(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	for i := range 10 {
		env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
			"Task "+string(rune('0'+i)), domain.TaskStatusTodo, nil)
		time.Sleep(5 * time.Millisecond)
	}

	filter := domain.TaskFilter{TeamID: &team.ID}

	page1, err := env.taskRepo.List(env.ctx, filter, domain.TaskPagination{Limit: 3, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, page1.Tasks, 3)
	assert.Equal(t, 10, page1.Total)

	page2, err := env.taskRepo.List(env.ctx, filter, domain.TaskPagination{Limit: 3, Offset: 3})
	require.NoError(t, err)
	assert.Len(t, page2.Tasks, 3)
	assert.Equal(t, 10, page2.Total)

	page1IDs := make(map[uuid.UUID]bool)
	for _, task := range page1.Tasks {
		page1IDs[task.ID] = true
	}
	for _, task := range page2.Tasks {
		assert.False(t, page1IDs[task.ID], "Tasks should not overlap between pages")
	}

	emptyPage, err := env.taskRepo.List(env.ctx, filter, domain.TaskPagination{Limit: 3, Offset: 100})
	require.NoError(t, err)
	assert.Empty(t, emptyPage.Tasks)
	assert.Equal(t, 10, emptyPage.Total)
}

func TestTaskRepository_List_Order(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "First", domain.TaskStatusTodo, nil)
	time.Sleep(10 * time.Millisecond)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Second", domain.TaskStatusTodo, nil)
	time.Sleep(10 * time.Millisecond)
	env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID, "Third", domain.TaskStatusTodo, nil)

	filter := domain.TaskFilter{TeamID: &team.ID}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)
	require.Len(t, result.Tasks, 3)

	assert.Equal(t, "Third", result.Tasks[0].Title)
	assert.Equal(t, "Second", result.Tasks[1].Title)
	assert.Equal(t, "First", result.Tasks[2].Title)
}

func TestTaskRepository_List_TeamIsolation(t *testing.T) {
	env := setupPostgres(t)

	owner1 := env.createTestUser(t, randomEmail())
	owner2 := env.createTestUser(t, randomEmail())
	team1 := env.createTestTeam(t, owner1.ID, "Team 1")
	team2 := env.createTestTeam(t, owner2.ID, "Team 2")

	env.createTestTaskWithHistoryAndAssignee(t, team1.ID, owner1.ID, "Team1 Task 1", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team1.ID, owner1.ID, "Team1 Task 2", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistoryAndAssignee(t, team2.ID, owner2.ID, "Team2 Task", domain.TaskStatusTodo, nil)

	filter := domain.TaskFilter{TeamID: &team1.ID}
	pagination := domain.TaskPagination{Limit: 20, Offset: 0}

	result, err := env.taskRepo.List(env.ctx, filter, pagination)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Total, "Should only see team1's tasks")
	for _, task := range result.Tasks {
		assert.Equal(t, team1.ID, task.TeamID)
	}
}

func TestTaskRepository_Update_TitleOnly(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Original Title", domain.TaskStatusTodo, nil)

	newTitle := "Updated Title"
	update := domain.TaskUpdate{
		Title:    &newTitle,
		TitleSet: true,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"title": {"from": "Original Title", "to": "Updated Title"}}`),
	}

	updated, err := env.taskRepo.Update(env.ctx, task.ID, owner.ID, update, history)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, domain.TaskStatusTodo, updated.Status)
	assert.Nil(t, updated.AssigneeID)

	assert.True(t, updated.UpdatedAt.After(task.CreatedAt) ||
		updated.UpdatedAt.Equal(task.CreatedAt))
}

func TestTaskRepository_Update_MultipleFields(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Task", domain.TaskStatusTodo, nil)

	newTitle := "New Title"
	newStatus := domain.TaskStatusInProgress
	newDesc := "New description"

	update := domain.TaskUpdate{
		Title:          &newTitle,
		TitleSet:       true,
		Status:         &newStatus,
		StatusSet:      true,
		Description:    &newDesc,
		DescriptionSet: true,
		AssigneeID:     &assignee.ID,
		AssigneeIDSet:  true,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"title": {"to": "New Title"}}`),
	}

	updated, err := env.taskRepo.Update(env.ctx, task.ID, owner.ID, update, history)
	require.NoError(t, err)

	assert.Equal(t, "New Title", updated.Title)
	assert.Equal(t, domain.TaskStatusInProgress, updated.Status)
	require.NotNil(t, updated.Description)
	assert.Equal(t, "New description", *updated.Description)
	require.NotNil(t, updated.AssigneeID)
	assert.Equal(t, assignee.ID, *updated.AssigneeID)
}

func TestTaskRepository_Update_Unassign(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Assigned Task", domain.TaskStatusTodo, &assignee.ID)

	require.NotNil(t, task.AssigneeID, "Task should have assignee initially")

	update := domain.TaskUpdate{
		AssigneeID:    nil,
		AssigneeIDSet: true,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"assignee_id": {"to": null}}`),
	}

	updated, err := env.taskRepo.Update(env.ctx, task.ID, owner.ID, update, history)
	require.NoError(t, err)
	assert.Nil(t, updated.AssigneeID, "Assignee should be nil after unassign")
}

func TestTaskRepository_Update_NotFound(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonExistentID := uuid.New()

	newTitle := "Won't Happen"
	update := domain.TaskUpdate{
		Title:    &newTitle,
		TitleSet: true,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    nonExistentID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{}`),
	}

	updated, err := env.taskRepo.Update(env.ctx, nonExistentID, owner.ID, update, history)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTaskNotFound)
	assert.Nil(t, updated)
}

func TestTaskRepository_Update_HistoryCreated(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Task", domain.TaskStatusTodo, nil)

	historiesBefore, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, historiesBefore, 1)
	assert.Equal(t, domain.TaskHistoryActionCreated, historiesBefore[0].Action)

	newTitle := "Updated"
	update := domain.TaskUpdate{
		Title:    &newTitle,
		TitleSet: true,
	}

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"title": {"to": "Updated"}}`),
	}

	_, err = env.taskRepo.Update(env.ctx, task.ID, owner.ID, update, history)
	require.NoError(t, err)

	historiesAfter, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, historiesAfter, 2)

	assert.Equal(t, domain.TaskHistoryActionUpdated, historiesAfter[0].Action)
	assert.Equal(t, domain.TaskHistoryActionCreated, historiesAfter[1].Action)
}

func TestTaskRepository_GetHistory_SingleRecord(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Task", domain.TaskStatusTodo, nil)

	histories, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, histories, 1)

	h := histories[0]
	assert.Equal(t, task.ID, h.TaskID)
	assert.Equal(t, owner.ID, h.ChangedBy)
	assert.Equal(t, owner.Email, h.ChangedByEmail, "Should include email via JOIN")
	assert.Equal(t, domain.TaskHistoryActionCreated, h.Action)
	assert.False(t, h.ChangedAt.IsZero())
	assert.NotEmpty(t, h.Changes)
}

func TestTaskRepository_GetHistory_MultipleRecords(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	task, _ := env.createTestTaskWithHistoryAndAssignee(t, team.ID, owner.ID,
		"Task", domain.TaskStatusTodo, nil)

	time.Sleep(10 * time.Millisecond)
	update1 := domain.TaskUpdate{
		Status:    func() *domain.TaskStatus { s := domain.TaskStatusInProgress; return &s }(),
		StatusSet: true,
	}
	history1 := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"status": {"to": "in_progress"}}`),
	}
	_, err := env.taskRepo.Update(env.ctx, task.ID, owner.ID, update1, history1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	update2 := domain.TaskUpdate{
		Status:    func() *domain.TaskStatus { s := domain.TaskStatusDone; return &s }(),
		StatusSet: true,
	}
	history2 := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   json.RawMessage(`{"status": {"to": "done"}}`),
	}
	_, err = env.taskRepo.Update(env.ctx, task.ID, owner.ID, update2, history2)
	require.NoError(t, err)

	histories, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, histories, 3)

	assert.Equal(t, domain.TaskHistoryActionUpdated, histories[0].Action)
	assert.Equal(t, domain.TaskHistoryActionUpdated, histories[1].Action)
	assert.Equal(t, domain.TaskHistoryActionCreated, histories[2].Action)

	assert.True(t, histories[0].ChangedAt.After(histories[1].ChangedAt) ||
		histories[0].ChangedAt.Equal(histories[1].ChangedAt))
	assert.True(t, histories[1].ChangedAt.After(histories[2].ChangedAt) ||
		histories[1].ChangedAt.Equal(histories[2].ChangedAt))
}

func TestTaskRepository_GetHistory_EmptyForNonExistent(t *testing.T) {
	env := setupPostgres(t)

	nonExistentID := uuid.New()
	histories, err := env.taskRepo.GetHistory(env.ctx, nonExistentID)
	require.NoError(t, err)
	assert.NotNil(t, histories)
	assert.Empty(t, histories)
}

func TestTaskRepository_GetHistory_JSONBChanges(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    team.ID,
		Title:     "Task",
		Status:    domain.TaskStatusTodo,
		CreatedBy: owner.ID,
	}

	complexChanges := map[string]any{
		"title":  "Task",
		"status": "todo",
		"nested": map[string]any{
			"field": "value",
			"count": 42,
		},
		"array": []int{1, 2, 3},
	}
	changesJSON, err := json.Marshal(complexChanges)
	require.NoError(t, err)

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: owner.ID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   changesJSON,
	}

	require.NoError(t, env.taskRepo.Create(env.ctx, task, history))

	histories, err := env.taskRepo.GetHistory(env.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, histories, 1)

	var retrieved map[string]any
	err = json.Unmarshal(histories[0].Changes, &retrieved)
	require.NoError(t, err)

	assert.Equal(t, "Task", retrieved["title"])
	assert.Equal(t, "todo", retrieved["status"])

	nested, ok := retrieved["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", nested["field"])
}

func TestTaskRepository_IsTeamMember_Owner(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isMember, "Owner should be a team member")
}

func TestTaskRepository_IsTeamMember_Member(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	member := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	env.addTeamMember(t, team.ID, member.ID, domain.TeamRoleMember)

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestTaskRepository_IsTeamMember_Admin(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	admin := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")
	env.addTeamMember(t, team.ID, admin.ID, domain.TeamRoleAdmin)

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, admin.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestTaskRepository_IsTeamMember_NotMember(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Test Team")

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, nonMember.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}

func TestTaskRepository_IsTeamMember_NonExistentTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	nonExistentTeamID := uuid.New()

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, nonExistentTeamID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}