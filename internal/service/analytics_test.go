package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-forge/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

// mockAnalyticsRepository is a mock implementation of repository.AnalyticsRepository.
type mockAnalyticsRepository struct {
	mock.Mock
}

func (m *mockAnalyticsRepository) GetTeamStats(
	ctx context.Context,
	days int,
	sinceDate time.Time,
) ([]domain.TeamStats, error) {
	args := m.Called(ctx, days, sinceDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TeamStats), args.Error(1)
}

func (m *mockAnalyticsRepository) GetTopCreators(
	ctx context.Context,
	months int,
	topN int,
) ([]domain.TopCreator, error) {
	args := m.Called(ctx, months, topN)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TopCreator), args.Error(1)
}

func (m *mockAnalyticsRepository) FindAssigneeIntegrityViolations(
	ctx context.Context,
	limit int,
) ([]domain.IntegrityViolation, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.IntegrityViolation), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestAnalyticsService creates an analyticsService with injected mocks.
func newTestAnalyticsService(repo *mockAnalyticsRepository) *analyticsService {
	return &analyticsService{
		analyticsRepo: repo,
		logger:        zerolog.Nop(),
	}
}

// ============================================================================
// TestAnalyticsService_GetTeamStats
// ============================================================================

func TestAnalyticsService_GetTeamStats_Success(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	days := 7

	// Prepare test data
	now := time.Now()
	stats := []domain.TeamStats{
		{
			TeamID:         "team-1",
			TeamName:       "Alpha Team",
			MembersCount:   5,
			DoneTasksCount: 10,
			DoneTasksSince: now,
		},
		{
			TeamID:         "team-2",
			TeamName:       "Beta Team",
			MembersCount:   3,
			DoneTasksCount: 7,
			DoneTasksSince: now,
		},
	}

	// Mock repository call
	repo.On("GetTeamStats", ctx, days, mock.MatchedBy(func(t time.Time) bool {
		// sinceDate should be approximately now - 7 days
		expected := time.Now().AddDate(0, 0, -7)
		return t.Sub(expected).Abs() < 2*time.Second
	})).Return(stats, nil)

	// Act
	resp, err := svc.GetTeamStats(ctx, days)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Stats, 2)
	assert.Equal(t, 2, resp.TotalTeams)
	assert.Equal(t, days, resp.DaysPeriod)

	// Check first team
	assert.Equal(t, "team-1", resp.Stats[0].TeamID)
	assert.Equal(t, "Alpha Team", resp.Stats[0].TeamName)
	assert.Equal(t, 5, resp.Stats[0].MembersCount)
	assert.Equal(t, 10, resp.Stats[0].DoneTasksCount)
	assert.NotEmpty(t, resp.Stats[0].DoneTasksSince, "Date should be formatted")

	// Check second team
	assert.Equal(t, "team-2", resp.Stats[1].TeamID)
	assert.Equal(t, "Beta Team", resp.Stats[1].TeamName)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_DefaultDays(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// days=0 should use default (7)
	repo.On("GetTeamStats", ctx, DefaultDaysPeriod, mock.AnythingOfType("time.Time")).
		Return([]domain.TeamStats{}, nil)

	resp, err := svc.GetTeamStats(ctx, 0)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DefaultDaysPeriod, resp.DaysPeriod,
		"days=0 should normalize to default (7)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_NegativeDays(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// days=-5 should use default (7)
	repo.On("GetTeamStats", ctx, DefaultDaysPeriod, mock.AnythingOfType("time.Time")).
		Return([]domain.TeamStats{}, nil)

	resp, err := svc.GetTeamStats(ctx, -5)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DefaultDaysPeriod, resp.DaysPeriod,
		"Negative days should normalize to default (7)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_ExceedsMax(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// days=400 should be capped to max (365)
	repo.On("GetTeamStats", ctx, MaxDaysPeriod, mock.AnythingOfType("time.Time")).
		Return([]domain.TeamStats{}, nil)

	resp, err := svc.GetTeamStats(ctx, 400)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, MaxDaysPeriod, resp.DaysPeriod,
		"days=400 should be capped to max (365)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_RepositoryError(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	days := 7

	dbErr := errors.New("database connection failed")
	repo.On("GetTeamStats", ctx, days, mock.AnythingOfType("time.Time")).
		Return(nil, dbErr)

	resp, err := svc.GetTeamStats(ctx, days)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get team stats")
	assert.True(t, errors.Is(err, dbErr),
		"Original error should be preserved in chain")
	assert.Nil(t, resp)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_EmptyResult(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	days := 7

	repo.On("GetTeamStats", ctx, days, mock.AnythingOfType("time.Time")).
		Return([]domain.TeamStats{}, nil)

	resp, err := svc.GetTeamStats(ctx, days)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Stats, "Should return empty slice")
	assert.Equal(t, 0, resp.TotalTeams)
	assert.Equal(t, days, resp.DaysPeriod)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTeamStats_DateFormatting(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	days := 7

	// Use a fixed time for predictable formatting
	fixedTime := time.Date(2026, 8, 13, 10, 30, 0, 0, time.UTC)
	stats := []domain.TeamStats{
		{
			TeamID:         "team-1",
			TeamName:       "Test Team",
			MembersCount:   1,
			DoneTasksCount: 1,
			DoneTasksSince: fixedTime,
		},
	}

	repo.On("GetTeamStats", ctx, days, mock.AnythingOfType("time.Time")).
		Return(stats, nil)

	resp, err := svc.GetTeamStats(ctx, days)

	require.NoError(t, err)
	require.Len(t, resp.Stats, 1)

	// Check that date is formatted in ISO 8601
	assert.Equal(t, "2026-08-13T10:30:00Z", resp.Stats[0].DoneTasksSince,
		"Date should be formatted as ISO 8601")

	repo.AssertExpectations(t)
}

// ============================================================================
// TestAnalyticsService_GetTopCreators
// ============================================================================

func TestAnalyticsService_GetTopCreators_Success(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	months := 1
	topN := 3

	// Prepare test data: 2 teams with multiple creators
	creators := []domain.TopCreator{
		{
			TeamID:       "team-1",
			TeamName:     "Alpha Team",
			UserID:       "user-1",
			UserEmail:    "alice@example.com",
			TasksCreated: 10,
			Rank:         1,
		},
		{
			TeamID:       "team-1",
			TeamName:     "Alpha Team",
			UserID:       "user-2",
			UserEmail:    "bob@example.com",
			TasksCreated: 7,
			Rank:         2,
		},
		{
			TeamID:       "team-2",
			TeamName:     "Beta Team",
			UserID:       "user-3",
			UserEmail:    "carol@example.com",
			TasksCreated: 15,
			Rank:         1,
		},
	}

	repo.On("GetTopCreators", ctx, months, topN).
		Return(creators, nil)

	resp, err := svc.GetTopCreators(ctx, months, topN)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Teams, 2, "Should have 2 teams")
	assert.Equal(t, 2, resp.TotalTeams)
	assert.Equal(t, months, resp.MonthsPeriod)
	assert.Equal(t, topN, resp.TopN)
	assert.NotEmpty(t, resp.SinceDate, "SinceDate should be formatted")

	// Check first team (Alpha)
	alphaTeam := resp.Teams[0]
	assert.Equal(t, "team-1", alphaTeam.TeamID)
	assert.Equal(t, "Alpha Team", alphaTeam.TeamName)
	assert.Len(t, alphaTeam.Creators, 2)
	assert.Equal(t, "user-1", alphaTeam.Creators[0].UserID)
	assert.Equal(t, 10, alphaTeam.Creators[0].TasksCreated)
	assert.Equal(t, 1, alphaTeam.Creators[0].Rank)
	assert.Equal(t, "user-2", alphaTeam.Creators[1].UserID)
	assert.Equal(t, 7, alphaTeam.Creators[1].TasksCreated)
	assert.Equal(t, 2, alphaTeam.Creators[1].Rank)

	// Check second team (Beta)
	betaTeam := resp.Teams[1]
	assert.Equal(t, "team-2", betaTeam.TeamID)
	assert.Equal(t, "Beta Team", betaTeam.TeamName)
	assert.Len(t, betaTeam.Creators, 1)
	assert.Equal(t, "user-3", betaTeam.Creators[0].UserID)
	assert.Equal(t, 15, betaTeam.Creators[0].TasksCreated)
	assert.Equal(t, 1, betaTeam.Creators[0].Rank)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_DefaultMonths(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// months=0 should use default (1)
	repo.On("GetTopCreators", ctx, DefaultMonthsPeriod, DefaultTopN).
		Return([]domain.TopCreator{}, nil)

	resp, err := svc.GetTopCreators(ctx, 0, 3)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DefaultMonthsPeriod, resp.MonthsPeriod,
		"months=0 should normalize to default (1)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_ExceedsMaxMonths(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// months=15 should be capped to max (12)
	repo.On("GetTopCreators", ctx, MaxMonthsPeriod, DefaultTopN).
		Return([]domain.TopCreator{}, nil)

	resp, err := svc.GetTopCreators(ctx, 15, 3)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, MaxMonthsPeriod, resp.MonthsPeriod,
		"months=15 should be capped to max (12)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_DefaultTopN(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// topN=0 should use default (3)
	repo.On("GetTopCreators", ctx, DefaultMonthsPeriod, DefaultTopN).
		Return([]domain.TopCreator{}, nil)

	resp, err := svc.GetTopCreators(ctx, 1, 0)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DefaultTopN, resp.TopN,
		"topN=0 should normalize to default (3)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_ExceedsMaxTopN(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// topN=20 should be capped to max (10)
	repo.On("GetTopCreators", ctx, DefaultMonthsPeriod, MaxTopN).
		Return([]domain.TopCreator{}, nil)

	resp, err := svc.GetTopCreators(ctx, 1, 20)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, MaxTopN, resp.TopN,
		"topN=20 should be capped to max (10)")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_RepositoryError(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	months := 1
	topN := 3

	dbErr := errors.New("query timeout")
	repo.On("GetTopCreators", ctx, months, topN).
		Return(nil, dbErr)

	resp, err := svc.GetTopCreators(ctx, months, topN)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get top creators")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_EmptyResult(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	months := 1
	topN := 3

	repo.On("GetTopCreators", ctx, months, topN).
		Return([]domain.TopCreator{}, nil)

	resp, err := svc.GetTopCreators(ctx, months, topN)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Teams, "Should return empty teams slice")
	assert.Equal(t, 0, resp.TotalTeams)
	assert.Equal(t, months, resp.MonthsPeriod)
	assert.Equal(t, topN, resp.TopN)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_GetTopCreators_PreservesTeamOrder(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// Creators from 3 teams in specific order
	creators := []domain.TopCreator{
		{TeamID: "team-3", TeamName: "Zebra Team", UserID: "user-1", Rank: 1},
		{TeamID: "team-1", TeamName: "Alpha Team", UserID: "user-2", Rank: 1},
		{TeamID: "team-3", TeamName: "Zebra Team", UserID: "user-3", Rank: 2},
		{TeamID: "team-2", TeamName: "Beta Team", UserID: "user-4", Rank: 1},
	}

	repo.On("GetTopCreators", ctx, 1, 3).
		Return(creators, nil)

	resp, err := svc.GetTopCreators(ctx, 1, 3)

	require.NoError(t, err)
	require.Len(t, resp.Teams, 3)

	// Order should be preserved: team-3, team-1, team-2
	assert.Equal(t, "team-3", resp.Teams[0].TeamID, "First team should be team-3")
	assert.Equal(t, "team-1", resp.Teams[1].TeamID, "Second team should be team-1")
	assert.Equal(t, "team-2", resp.Teams[2].TeamID, "Third team should be team-2")

	// team-3 should have 2 creators
	assert.Len(t, resp.Teams[0].Creators, 2)

	repo.AssertExpectations(t)
}

// ============================================================================
// TestAnalyticsService_CheckAssigneeIntegrity
// ============================================================================

func TestAnalyticsService_CheckAssigneeIntegrity_Healthy(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	limit := 100

	// No violations
	repo.On("FindAssigneeIntegrityViolations", ctx, limit).
		Return([]domain.IntegrityViolation{}, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, limit)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Healthy, "Should be healthy when no violations")
	assert.Empty(t, resp.Violations)
	assert.Equal(t, 0, resp.ViolationsCount)
	assert.NotEmpty(t, resp.CheckedAt, "CheckedAt should be formatted")

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_ViolationsDetected(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	limit := 100

	now := time.Now()
	violations := []domain.IntegrityViolation{
		{
			TaskID:         "task-1",
			TeamID:         "team-1",
			TeamName:       "Alpha Team",
			TaskTitle:      "Task 1",
			TaskStatus:     "todo",
			AssigneeID:     "user-1",
			AssigneeEmail:  "alice@example.com",
			CreatedByID:    "user-2",
			CreatedByEmail: "bob@example.com",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			TaskID:         "task-2",
			TeamID:         "team-2",
			TeamName:       "Beta Team",
			TaskTitle:      "Task 2",
			TaskStatus:     "in_progress",
			AssigneeID:     "user-3",
			AssigneeEmail:  "carol@example.com",
			CreatedByID:    "user-4",
			CreatedByEmail: "dave@example.com",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	repo.On("FindAssigneeIntegrityViolations", ctx, limit).
		Return(violations, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, limit)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Healthy, "Should NOT be healthy when violations exist")
	assert.Len(t, resp.Violations, 2)
	assert.Equal(t, 2, resp.ViolationsCount)

	// Check first violation
	assert.Equal(t, "task-1", resp.Violations[0].TaskID)
	assert.Equal(t, "team-1", resp.Violations[0].TeamID)
	assert.Equal(t, "Alpha Team", resp.Violations[0].TeamName)
	assert.Equal(t, "Task 1", resp.Violations[0].TaskTitle)
	assert.Equal(t, "todo", resp.Violations[0].TaskStatus)
	assert.Equal(t, "user-1", resp.Violations[0].AssigneeID)
	assert.Equal(t, "alice@example.com", resp.Violations[0].AssigneeEmail)
	assert.Equal(t, "user-2", resp.Violations[0].CreatedByID)
	assert.Equal(t, "bob@example.com", resp.Violations[0].CreatedByEmail)
	assert.NotEmpty(t, resp.Violations[0].CreatedAt)
	assert.NotEmpty(t, resp.Violations[0].UpdatedAt)

	// Check second violation
	assert.Equal(t, "task-2", resp.Violations[1].TaskID)
	assert.Equal(t, "team-2", resp.Violations[1].TeamID)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_DefaultLimit(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// limit=0 should use default (100)
	repo.On("FindAssigneeIntegrityViolations", ctx, DefaultIntegrityLimit).
		Return([]domain.IntegrityViolation{}, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, 0)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Healthy)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_NegativeLimit(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// limit=-10 should use default (100)
	repo.On("FindAssigneeIntegrityViolations", ctx, DefaultIntegrityLimit).
		Return([]domain.IntegrityViolation{}, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, -10)

	require.NoError(t, err)
	require.NotNil(t, resp)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_ExceedsMaxLimit(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()

	// limit=2000 should be capped to max (1000)
	repo.On("FindAssigneeIntegrityViolations", ctx, MaxIntegrityLimit).
		Return([]domain.IntegrityViolation{}, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, 2000)

	require.NoError(t, err)
	require.NotNil(t, resp)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_RepositoryError(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	limit := 100

	dbErr := errors.New("query failed")
	repo.On("FindAssigneeIntegrityViolations", ctx, limit).
		Return(nil, dbErr)

	resp, err := svc.CheckAssigneeIntegrity(ctx, limit)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check assignee integrity")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	repo.AssertExpectations(t)
}

func TestAnalyticsService_CheckAssigneeIntegrity_DateFormatting(t *testing.T) {
	repo := new(mockAnalyticsRepository)
	svc := newTestAnalyticsService(repo)

	ctx := context.Background()
	limit := 100

	// Use fixed time for predictable formatting
	fixedTime := time.Date(2026, 8, 13, 10, 30, 0, 0, time.UTC)
	violations := []domain.IntegrityViolation{
		{
			TaskID:         "task-1",
			TeamID:         "team-1",
			TeamName:       "Test Team",
			TaskTitle:      "Test Task",
			TaskStatus:     "todo",
			AssigneeID:     "user-1",
			AssigneeEmail:  "test@example.com",
			CreatedByID:    "user-2",
			CreatedByEmail: "creator@example.com",
			CreatedAt:      fixedTime,
			UpdatedAt:      fixedTime,
		},
	}

	repo.On("FindAssigneeIntegrityViolations", ctx, limit).
		Return(violations, nil)

	resp, err := svc.CheckAssigneeIntegrity(ctx, limit)

	require.NoError(t, err)
	require.Len(t, resp.Violations, 1)

	// Check that dates are formatted in ISO 8601
	assert.Equal(t, "2026-08-13T10:30:00Z", resp.Violations[0].CreatedAt,
		"CreatedAt should be formatted as ISO 8601")
	assert.Equal(t, "2026-08-13T10:30:00Z", resp.Violations[0].UpdatedAt,
		"UpdatedAt should be formatted as ISO 8601")

	// CheckedAt should also be formatted
	assert.NotEmpty(t, resp.CheckedAt)
	_, parseErr := time.Parse("2006-01-02T15:04:05Z07:00", resp.CheckedAt)
	assert.NoError(t, parseErr, "CheckedAt should be valid ISO 8601")

	repo.AssertExpectations(t)
}

// ============================================================================
// TestAnalyticsService_EdgeCases
// ============================================================================

func TestAnalyticsService_GetTeamStats_AllParametersNormalized(t *testing.T) {
	testCases := []struct {
		name         string
		inputDays    int
		expectedDays int
	}{
		{"zero", 0, DefaultDaysPeriod},
		{"negative", -10, DefaultDaysPeriod},
		{"valid", 30, 30},
		{"at max", MaxDaysPeriod, MaxDaysPeriod},
		{"above max", 500, MaxDaysPeriod},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockAnalyticsRepository)
			svc := newTestAnalyticsService(repo)

			ctx := context.Background()

			repo.On("GetTeamStats", ctx, tc.expectedDays, mock.AnythingOfType("time.Time")).
				Return([]domain.TeamStats{}, nil)

			resp, err := svc.GetTeamStats(ctx, tc.inputDays)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedDays, resp.DaysPeriod)

			repo.AssertExpectations(t)
		})
	}
}

func TestAnalyticsService_GetTopCreators_AllParametersNormalized(t *testing.T) {
	testCases := []struct {
		name           string
		inputMonths    int
		inputTopN      int
		expectedMonths int
		expectedTopN   int
	}{
		{"both defaults", 0, 0, DefaultMonthsPeriod, DefaultTopN},
		{"months negative", -5, 3, DefaultMonthsPeriod, 3},
		{"topN negative", 1, -3, 1, DefaultTopN},
		{"both valid", 6, 5, 6, 5},
		{"both at max", MaxMonthsPeriod, MaxTopN, MaxMonthsPeriod, MaxTopN},
		{"both above max", 20, 15, MaxMonthsPeriod, MaxTopN},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockAnalyticsRepository)
			svc := newTestAnalyticsService(repo)

			ctx := context.Background()

			repo.On("GetTopCreators", ctx, tc.expectedMonths, tc.expectedTopN).
				Return([]domain.TopCreator{}, nil)

			resp, err := svc.GetTopCreators(ctx, tc.inputMonths, tc.inputTopN)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedMonths, resp.MonthsPeriod)
			assert.Equal(t, tc.expectedTopN, resp.TopN)

			repo.AssertExpectations(t)
		})
	}
}

func TestAnalyticsService_CheckAssigneeIntegrity_AllLimitsNormalized(t *testing.T) {
	testCases := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{"zero", 0, DefaultIntegrityLimit},
		{"negative", -50, DefaultIntegrityLimit},
		{"valid", 500, 500},
		{"at max", MaxIntegrityLimit, MaxIntegrityLimit},
		{"above max", 2000, MaxIntegrityLimit},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockAnalyticsRepository)
			svc := newTestAnalyticsService(repo)

			ctx := context.Background()

			repo.On("FindAssigneeIntegrityViolations", ctx, tc.expectedLimit).
				Return([]domain.IntegrityViolation{}, nil)

			resp, err := svc.CheckAssigneeIntegrity(ctx, tc.inputLimit)

			require.NoError(t, err)
			require.NotNil(t, resp)

			repo.AssertExpectations(t)
		})
	}
}
