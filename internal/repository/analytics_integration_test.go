//go:build integration
// +build integration

// internal/repository/analytics_integration_test.go
package repository

import (
	"testing"
	"time"

	"task-forge/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsRepository_FindAssigneeIntegrityViolations_NoViolations(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())

	team := env.createTestTeam(t, owner.ID, "Valid Team")
	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	assigneeID := assignee.ID

	env.createTestTaskWithHistoryAndAssignee(
		t,
		team.ID,
		owner.ID,
		"Valid task",
		domain.TaskStatusDone,
		&assigneeID,
	)

	violations, err := env.analyticsRepo.FindAssigneeIntegrityViolations(
		env.ctx,
		100,
	)

	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestAnalyticsRepository_GetTeamStats_Empty(t *testing.T) {
	env := setupPostgres(t)

	sinceDate := time.Now().AddDate(0, 0, -7)

	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, 7, sinceDate)
	require.NoError(t, err)
	assert.NotNil(t, stats, "Should return empty slice, not nil")
	assert.Empty(t, stats)
}

func TestAnalyticsRepository_GetTeamStats_SingleTeam(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Alpha Team")

	member1 := env.createTestUser(t, randomEmail())
	member2 := env.createTestUser(t, randomEmail())
	env.addTeamMember(t, team.ID, member1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, member2.ID, domain.TeamRoleAdmin)

	task1 := env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Task 1", domain.TaskStatusTodo, nil)
	task2 := env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Task 2", domain.TaskStatusTodo, nil)
	task3 := env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Task 3", domain.TaskStatusTodo, nil)

	env.updateTaskStatus(t, task1.ID, owner.ID, domain.TaskStatusDone)
	env.updateTaskStatus(t, task2.ID, owner.ID, domain.TaskStatusDone)
	env.updateTaskStatus(t, task3.ID, owner.ID, domain.TaskStatusDone)

	sinceDate := time.Now().AddDate(0, 0, -7)
	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, 7, sinceDate)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	s := stats[0]
	assert.Equal(t, team.ID.String(), s.TeamID)
	assert.Equal(t, "Alpha Team", s.TeamName)
	assert.Equal(t, 3, s.MembersCount, "Owner + 2 members = 3")
	assert.Equal(t, 3, s.DoneTasksCount, "All 3 tasks should be done")
}

func TestAnalyticsRepository_GetTeamStats_MultipleTeams(t *testing.T) {
	env := setupPostgres(t)

	ownerA := env.createTestUser(t, randomEmail())
	teamA := env.createTestTeam(t, ownerA.ID, "Alpha Team")
	memberA1 := env.createTestUser(t, randomEmail())
	memberA2 := env.createTestUser(t, randomEmail())
	env.addTeamMember(t, teamA.ID, memberA1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, teamA.ID, memberA2.ID, domain.TeamRoleMember)

	taskA1 := env.createTestTaskWithHistory(t, teamA.ID, ownerA.ID, "A1", domain.TaskStatusTodo, nil)
	taskA2 := env.createTestTaskWithHistory(t, teamA.ID, ownerA.ID, "A2", domain.TaskStatusTodo, nil)
	env.updateTaskStatus(t, taskA1.ID, ownerA.ID, domain.TaskStatusDone)
	env.updateTaskStatus(t, taskA2.ID, ownerA.ID, domain.TaskStatusDone)

	ownerB := env.createTestUser(t, randomEmail())
	teamB := env.createTestTeam(t, ownerB.ID, "Beta Team")
	taskB1 := env.createTestTaskWithHistory(t, teamB.ID, ownerB.ID, "B1", domain.TaskStatusTodo, nil)
	env.updateTaskStatus(t, taskB1.ID, ownerB.ID, domain.TaskStatusDone)

	sinceDate := time.Now().AddDate(0, 0, -7)
	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, 7, sinceDate)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	assert.Equal(t, "Alpha Team", stats[0].TeamName)
	assert.Equal(t, "Beta Team", stats[1].TeamName)

	assert.Equal(t, 3, stats[0].MembersCount)
	assert.Equal(t, 2, stats[0].DoneTasksCount)

	assert.Equal(t, 1, stats[1].MembersCount)
	assert.Equal(t, 1, stats[1].DoneTasksCount)
}

func TestAnalyticsRepository_GetTeamStats_TeamWithNoMembersExceptOwner(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Solo Team")

	sinceDate := time.Now().AddDate(0, 0, -7)

	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, 7, sinceDate)

	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, team.ID.String(), stats[0].TeamID)
	assert.Equal(t, "Solo Team", stats[0].TeamName)
	assert.Equal(t, 1, stats[0].MembersCount)
	assert.Equal(t, 0, stats[0].DoneTasksCount)
}

func TestAnalyticsRepository_GetTeamStats_DateFiltering(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Filtered Team")

	recentTask := env.createTestTaskWithHistory(
		t,
		team.ID,
		owner.ID,
		"Recent",
		domain.TaskStatusTodo,
		nil,
	)

	env.updateTaskStatus(
		t,
		recentTask.ID,
		owner.ID,
		domain.TaskStatusDone,
	)

	oldDate := time.Now().AddDate(0, 0, -30)

	env.createOldDoneTask(
		t,
		team.ID,
		owner.ID,
		"Old",
		oldDate,
	)

	sinceDate := time.Now().AddDate(0, 0, -7)

	stats, err := env.analyticsRepo.GetTeamStats(
		env.ctx,
		7,
		sinceDate,
	)

	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(
		t,
		1,
		stats[0].DoneTasksCount,
		"Only recent done task should be counted",
	)
}

func TestAnalyticsRepository_GetTeamStats_NonDoneTasksNotCounted(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "In Progress Team")

	env.createTestTaskWithHistory(t, team.ID, owner.ID, "Todo", domain.TaskStatusTodo, nil)
	env.createTestTaskWithHistory(t, team.ID, owner.ID, "In Progress", domain.TaskStatusInProgress, nil)
	env.createTestTaskWithHistory(t, team.ID, owner.ID, "Review", domain.TaskStatusReview, nil)

	sinceDate := time.Now().AddDate(0, 0, -7)
	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, 7, sinceDate)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, 0, stats[0].DoneTasksCount,
		"Non-done tasks should not be counted")
}

func TestAnalyticsRepository_GetTopCreators_Empty(t *testing.T) {
	env := setupPostgres(t)

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	assert.NotNil(t, creators, "Should return empty slice, not nil")
	assert.Empty(t, creators)
}

func TestAnalyticsRepository_GetTopCreators_SingleTeam(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	user1 := env.createTestUser(t, randomEmail())
	user2 := env.createTestUser(t, randomEmail())
	user3 := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Dev Team")

	env.addTeamMember(t, team.ID, user1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, user2.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, user3.ID, domain.TeamRoleMember)

	for i := 0; i < 5; i++ {
		env.createTestTaskWithHistory(t, team.ID, user1.ID,
			"Task "+string(rune('A'+i)), domain.TaskStatusTodo, nil)
	}

	for i := 0; i < 3; i++ {
		env.createTestTaskWithHistory(t, team.ID, user2.ID,
			"Task "+string(rune('F'+i)), domain.TaskStatusTodo, nil)
	}

	env.createTestTaskWithHistory(t, team.ID, user3.ID, "Task Z", domain.TaskStatusTodo, nil)

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, creators, 3, "Should return top 3")

	assert.Equal(t, user1.ID.String(), creators[0].UserID)
	assert.Equal(t, 5, creators[0].TasksCreated)
	assert.Equal(t, 1, creators[0].Rank)

	assert.Equal(t, user2.ID.String(), creators[1].UserID)
	assert.Equal(t, 3, creators[1].TasksCreated)
	assert.Equal(t, 2, creators[1].Rank)

	assert.Equal(t, user3.ID.String(), creators[2].UserID)
	assert.Equal(t, 1, creators[2].TasksCreated)
	assert.Equal(t, 3, creators[2].Rank)

	assert.NotEmpty(t, creators[0].UserEmail)
	assert.NotEmpty(t, creators[1].UserEmail)
	assert.NotEmpty(t, creators[2].UserEmail)
}

func TestAnalyticsRepository_GetTopCreators_MultipleTeams(t *testing.T) {
	env := setupPostgres(t)

	ownerA := env.createTestUser(t, randomEmail())
	userA1 := env.createTestUser(t, randomEmail())
	userA2 := env.createTestUser(t, randomEmail())
	teamA := env.createTestTeam(t, ownerA.ID, "Alpha")
	env.addTeamMember(t, teamA.ID, userA1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, teamA.ID, userA2.ID, domain.TeamRoleMember)

	for i := 0; i < 5; i++ {
		env.createTestTaskWithHistory(t, teamA.ID, userA1.ID, "A1", domain.TaskStatusTodo, nil)
	}
	for i := 0; i < 2; i++ {
		env.createTestTaskWithHistory(t, teamA.ID, userA2.ID, "A2", domain.TaskStatusTodo, nil)
	}

	ownerB := env.createTestUser(t, randomEmail())
	userB1 := env.createTestUser(t, randomEmail())
	teamB := env.createTestTeam(t, ownerB.ID, "Beta")
	env.addTeamMember(t, teamB.ID, userB1.ID, domain.TeamRoleMember)

	for i := 0; i < 10; i++ {
		env.createTestTaskWithHistory(t, teamB.ID, userB1.ID, "B1", domain.TaskStatusTodo, nil)
	}

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 2)
	require.NoError(t, err)

	require.Len(t, creators, 3)

	teamsMap := make(map[string][]domain.TopCreator)
	for _, c := range creators {
		teamsMap[c.TeamName] = append(teamsMap[c.TeamName], c)
	}

	alphaCreators := teamsMap["Alpha"]
	require.Len(t, alphaCreators, 2)
	assert.Equal(t, 1, alphaCreators[0].Rank)
	assert.Equal(t, 5, alphaCreators[0].TasksCreated)
	assert.Equal(t, 2, alphaCreators[1].Rank)
	assert.Equal(t, 2, alphaCreators[1].TasksCreated)

	betaCreators := teamsMap["Beta"]
	require.Len(t, betaCreators, 1)
	assert.Equal(t, 1, betaCreators[0].Rank)
	assert.Equal(t, 10, betaCreators[0].TasksCreated)
}

func TestAnalyticsRepository_GetTopCreators_RankTies(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	user1 := env.createTestUser(t, randomEmail())
	user2 := env.createTestUser(t, randomEmail())
	user3 := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Ties Team")

	env.addTeamMember(t, team.ID, user1.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, user2.ID, domain.TeamRoleMember)
	env.addTeamMember(t, team.ID, user3.ID, domain.TeamRoleMember)

	for i := 0; i < 5; i++ {
		env.createTestTaskWithHistory(t, team.ID, user1.ID, "T1", domain.TaskStatusTodo, nil)
	}

	for i := 0; i < 3; i++ {
		env.createTestTaskWithHistory(t, team.ID, user2.ID, "T2", domain.TaskStatusTodo, nil)
		env.createTestTaskWithHistory(t, team.ID, user3.ID, "T3", domain.TaskStatusTodo, nil)
	}

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, creators, 3)

	assert.Equal(t, 1, creators[0].Rank)
	assert.Equal(t, 5, creators[0].TasksCreated)

	assert.Equal(t, 2, creators[1].Rank, "Both tied users should have rank 2")
	assert.Equal(t, 3, creators[1].TasksCreated)

	assert.Equal(t, 2, creators[2].Rank, "Both tied users should have rank 2")
	assert.Equal(t, 3, creators[2].TasksCreated)

	assert.True(t, creators[1].UserID < creators[2].UserID ||
		creators[1].UserID != creators[2].UserID,
		"Tied creators should be ordered by user_id ASC")
}

func TestAnalyticsRepository_GetTopCreators_TopNFiltering(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "TopN Team")

	users := make([]*domain.User, 5)
	for i := 0; i < 5; i++ {
		users[i] = env.createTestUser(t, randomEmail())
		env.addTeamMember(t, team.ID, users[i].ID, domain.TeamRoleMember)

		taskCount := 5 - i
		for j := 0; j < taskCount; j++ {
			env.createTestTaskWithHistory(t, team.ID, users[i].ID,
				"Task", domain.TaskStatusTodo, nil)
		}
	}

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, creators, 2, "Should return only top 2")

	assert.Equal(t, 1, creators[0].Rank)
	assert.Equal(t, 5, creators[0].TasksCreated)
	assert.Equal(t, 2, creators[1].Rank)
	assert.Equal(t, 4, creators[1].TasksCreated)
}

func TestAnalyticsRepository_GetTopCreators_DateFiltering(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Date Filtered Team")

	recentTask := env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Recent", domain.TaskStatusTodo, nil)
	oldTask := env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Old", domain.TaskStatusTodo, nil)

	_, err := env.pool.Exec(env.ctx,
		`UPDATE tasks SET created_at = $1 WHERE id = $2`,
		time.Now().AddDate(0, -6, 0),
		oldTask.ID,
	)
	require.NoError(t, err)
	_ = recentTask

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, creators, 1, "Only recent task should be counted")

	assert.Equal(t, 1, creators[0].TasksCreated,
		"Only 1 recent task should be counted (old task is 6 months ago)")
}

func TestAnalyticsRepository_GetTopCreators_Ordering(t *testing.T) {
	env := setupPostgres(t)

	ownerA := env.createTestUser(t, randomEmail())
	userA := env.createTestUser(t, randomEmail())
	teamA := env.createTestTeam(t, ownerA.ID, "Zebra Team")
	env.addTeamMember(t, teamA.ID, userA.ID, domain.TeamRoleMember)
	env.createTestTaskWithHistory(t, teamA.ID, userA.ID, "ZA1", domain.TaskStatusTodo, nil)

	ownerB := env.createTestUser(t, randomEmail())
	userB := env.createTestUser(t, randomEmail())
	teamB := env.createTestTeam(t, ownerB.ID, "Alpha Team")
	env.addTeamMember(t, teamB.ID, userB.ID, domain.TeamRoleMember)
	env.createTestTaskWithHistory(t, teamB.ID, userB.ID, "BA1", domain.TaskStatusTodo, nil)

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, creators, 2)

	assert.Equal(t, "Alpha Team", creators[0].TeamName)
	assert.Equal(t, "Zebra Team", creators[1].TeamName)
}

func TestAnalyticsRepository_FindAssigneeIntegrityViolations_Healthy(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	assignee := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Healthy Team")
	env.addTeamMember(t, team.ID, assignee.ID, domain.TeamRoleMember)

	env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Valid Task", domain.TaskStatusTodo, &assignee.ID)

	violations, err := env.analyticsRepo.FindAssigneeIntegrityViolations(env.ctx, 100)
	require.NoError(t, err)
	assert.NotNil(t, violations)
	assert.Empty(t, violations, "No violations should be detected")
}

func TestAnalyticsRepository_FindAssigneeIntegrityViolations_Unassigned(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTestTeam(t, owner.ID, "Unassigned Team")

	env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"No Assignee", domain.TaskStatusTodo, nil)

	violations, err := env.analyticsRepo.FindAssigneeIntegrityViolations(env.ctx, 100)
	require.NoError(t, err)
	assert.Empty(t, violations, "Unassigned tasks should not be violations")
}

func TestAnalyticsRepository_GetTeamStats_DoneTasksSinceField(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	env.createTestTeam(t, owner.ID, "Since Field Team")

	days := 10
	sinceDate := time.Now().AddDate(0, 0, -days)

	stats, err := env.analyticsRepo.GetTeamStats(env.ctx, days, sinceDate)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.WithinDuration(t, sinceDate, stats[0].DoneTasksSince, time.Second,
		"DoneTasksSince should match the passed sinceDate")
}

func TestAnalyticsRepository_GetTopCreators_UserWithEmail(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, "creator@example.com")
	team := env.createTestTeam(t, owner.ID, "Email Team")

	env.createTestTaskWithHistory(t, team.ID, owner.ID,
		"Task", domain.TaskStatusTodo, nil)

	creators, err := env.analyticsRepo.GetTopCreators(env.ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, creators, 1)

	assert.Equal(t, "creator@example.com", creators[0].UserEmail,
		"User email should be correctly joined")
}
