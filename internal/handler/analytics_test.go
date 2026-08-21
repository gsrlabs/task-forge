package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-forge/internal/dto"
	"task-forge/internal/middleware"
	"task-forge/internal/service"

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
// Mock AnalyticsService
// ============================================================================

type mockAnalyticsService struct {
	mock.Mock
}

func (m *mockAnalyticsService) GetTeamStats(
	ctx context.Context,
	days int,
) (*dto.TeamStatsResponse, error) {
	args := m.Called(ctx, days)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TeamStatsResponse), args.Error(1)
}

func (m *mockAnalyticsService) GetTopCreators(
	ctx context.Context,
	months, topN int,
) (*dto.TopCreatorsResponse, error) {
	args := m.Called(ctx, months, topN)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.TopCreatorsResponse), args.Error(1)
}

func (m *mockAnalyticsService) CheckAssigneeIntegrity(
	ctx context.Context,
	limit int,
) (*dto.IntegrityCheckResponse, error) {
	args := m.Called(ctx, limit)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.IntegrityCheckResponse), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

func newTestAnalyticsHandler(svc *mockAnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		service: svc,
		logger:  zerolog.Nop(),
	}
}

func performAnalyticsRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	userID uuid.UUID,
	queryParams map[string]string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = req

	if userID != uuid.Nil {
		middleware.SetUserIdentity(c, userID, "user@example.com")
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
// TestAnalyticsHandler_GetTeamStats
// ============================================================================

func TestAnalyticsHandler_GetTeamStats_Success_DefaultDays(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	expectedResp := &dto.TeamStatsResponse{
		Stats: []dto.TeamStatsItem{
			{
				TeamID:         uuid.New().String(),
				TeamName:       "Alpha Team",
				MembersCount:   5,
				DoneTasksCount: 10,
				DoneTasksSince: "2026-08-06T10:30:00Z",
			},
		},
		TotalTeams: 1,
		DaysPeriod: service.DefaultDaysPeriod,
	}

	svc.
		On("GetTeamStats", mock.Anything, service.DefaultDaysPeriod).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.GetTeamStats,
		http.MethodGet,
		"/api/v1/analytics/teams/stats",
		userID,
		nil, // No query params — use default
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TeamStatsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, 1, response.TotalTeams)
	assert.Equal(t, service.DefaultDaysPeriod, response.DaysPeriod)
	assert.Len(t, response.Stats, 1)
	assert.Equal(t, "Alpha Team", response.Stats[0].TeamName)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_GetTeamStats_Success_CustomDays(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()
	days := 30

	expectedResp := &dto.TeamStatsResponse{
		Stats:      []dto.TeamStatsItem{},
		TotalTeams: 0,
		DaysPeriod: days,
	}

	svc.
		On("GetTeamStats", mock.Anything, days).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.GetTeamStats,
		http.MethodGet,
		"/api/v1/analytics/teams/stats",
		userID,
		map[string]string{"days": "30"},
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TeamStatsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, days, response.DaysPeriod)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_GetTeamStats_InvalidDays(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTeamStats,
		http.MethodGet,
		"/api/v1/analytics/teams/stats",
		userID,
		map[string]string{"days": "not-a-number"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "days must be a positive integer")

	svc.AssertNotCalled(t, "GetTeamStats", mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTeamStats_DaysTooSmall(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	tests := []struct {
		name string
		days string
	}{
		{"zero", "0"},
		{"negative", "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performAnalyticsRequest(
				handler.GetTeamStats,
				http.MethodGet,
				"/api/v1/analytics/teams/stats",
				userID,
				map[string]string{"days": tt.days},
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, "validation failed", response.Error)
			assert.Contains(t, response.Details, "days must be at least 1")

			svc.AssertNotCalled(t, "GetTeamStats", mock.Anything, mock.Anything)
		})
	}
}

func TestAnalyticsHandler_GetTeamStats_DaysTooLarge(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTeamStats,
		http.MethodGet,
		"/api/v1/analytics/teams/stats",
		userID,
		map[string]string{"days": "500"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "days cannot exceed 365")

	svc.AssertNotCalled(t, "GetTeamStats", mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTeamStats_ServiceError(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	dbErr := errors.New("database query failed")

	svc.
		On("GetTeamStats", mock.Anything, service.DefaultDaysPeriod).
		Return(nil, dbErr).
		Once()

	w := performAnalyticsRequest(
		handler.GetTeamStats,
		http.MethodGet,
		"/api/v1/analytics/teams/stats",
		userID,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to retrieve team statistics", response.Error)

	require.True(t, svc.AssertExpectations(t))
}

// ============================================================================
// TestAnalyticsHandler_GetTopCreators
// ============================================================================

func TestAnalyticsHandler_GetTopCreators_Success_Defaults(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	expectedResp := &dto.TopCreatorsResponse{
		Teams: []dto.TeamTopCreators{
			{
				TeamID:   uuid.New().String(),
				TeamName: "Alpha Team",
				Creators: []dto.CreatorStats{
					{
						UserID:       uuid.New().String(),
						UserEmail:    "alice@example.com",
						TasksCreated: 15,
						Rank:         1,
					},
				},
			},
		},
		TotalTeams:   1,
		MonthsPeriod: service.DefaultMonthsPeriod,
		TopN:         service.DefaultTopN,
		SinceDate:    "2026-07-20T10:30:00Z",
	}

	svc.
		On("GetTopCreators", mock.Anything, service.DefaultMonthsPeriod, service.DefaultTopN).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		nil, // Use defaults
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TopCreatorsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, 1, response.TotalTeams)
	assert.Equal(t, service.DefaultMonthsPeriod, response.MonthsPeriod)
	assert.Equal(t, service.DefaultTopN, response.TopN)
	assert.Len(t, response.Teams, 1)
	assert.Equal(t, "Alpha Team", response.Teams[0].TeamName)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_GetTopCreators_Success_CustomParams(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()
	months := 3
	topN := 5

	expectedResp := &dto.TopCreatorsResponse{
		Teams:        []dto.TeamTopCreators{},
		TotalTeams:   0,
		MonthsPeriod: months,
		TopN:         topN,
		SinceDate:    "2026-05-20T10:30:00Z",
	}

	svc.
		On("GetTopCreators", mock.Anything, months, topN).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{
			"months": "3",
			"top":    "5",
		},
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.TopCreatorsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, months, response.MonthsPeriod)
	assert.Equal(t, topN, response.TopN)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_GetTopCreators_InvalidMonths(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"months": "not-a-number"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "months must be a positive integer")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_MonthsTooSmall(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"months": "0"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "months must be at least 1")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_MonthsTooLarge(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"months": "15"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "months cannot exceed 12")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_InvalidTop(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"top": "not-a-number"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "top must be a positive integer")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_TopTooSmall(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"top": "0"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "top must be at least 1")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_TopTooLarge(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{"top": "15"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "top cannot exceed 10")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_ValidMonths_InvalidTop(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	// months валиден, top невалиден — должен упасть на проверке top
	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		map[string]string{
			"months": "3",
			"top":    "invalid",
		},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Contains(t, response.Details, "top must be a positive integer")

	svc.AssertNotCalled(t, "GetTopCreators", mock.Anything, mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_GetTopCreators_ServiceError(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	dbErr := errors.New("complex query failed")

	svc.
		On("GetTopCreators", mock.Anything, service.DefaultMonthsPeriod, service.DefaultTopN).
		Return(nil, dbErr).
		Once()

	w := performAnalyticsRequest(
		handler.GetTopCreators,
		http.MethodGet,
		"/api/v1/analytics/teams/top-creators",
		userID,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to retrieve top creators statistics", response.Error)

	require.True(t, svc.AssertExpectations(t))
}

// ============================================================================
// TestAnalyticsHandler_CheckAssigneeIntegrity
// ============================================================================

func TestAnalyticsHandler_CheckAssigneeIntegrity_Success_DefaultLimit(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	expectedResp := &dto.IntegrityCheckResponse{
		Healthy:         true,
		Violations:      []dto.IntegrityViolationItem{},
		ViolationsCount: 0,
		CheckedAt:       "2026-08-13T10:30:00Z",
	}

	svc.
		On("CheckAssigneeIntegrity", mock.Anything, service.DefaultIntegrityLimit).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		nil,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.IntegrityCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.True(t, response.Healthy)
	assert.Equal(t, 0, response.ViolationsCount)
	assert.Empty(t, response.Violations)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_Success_CustomLimit(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()
	limit := 500

	expectedResp := &dto.IntegrityCheckResponse{
		Healthy:         true,
		Violations:      []dto.IntegrityViolationItem{},
		ViolationsCount: 0,
		CheckedAt:       "2026-08-13T10:30:00Z",
	}

	svc.
		On("CheckAssigneeIntegrity", mock.Anything, limit).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		map[string]string{"limit": "500"},
	)

	require.Equal(t, http.StatusOK, w.Code)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_Unhealthy(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	// Response с нарушениями — но HTTP статус всё равно 200
	expectedResp := &dto.IntegrityCheckResponse{
		Healthy: false,
		Violations: []dto.IntegrityViolationItem{
			{
				TaskID:         uuid.New().String(),
				TeamID:         uuid.New().String(),
				TeamName:       "Violated Team",
				TaskTitle:      "Violated Task",
				TaskStatus:     "in_progress",
				AssigneeID:     uuid.New().String(),
				AssigneeEmail:  "assignee@example.com",
				CreatedByID:    uuid.New().String(),
				CreatedByEmail: "creator@example.com",
				CreatedAt:      "2026-08-13T10:30:00Z",
				UpdatedAt:      "2026-08-13T11:45:00Z",
			},
		},
		ViolationsCount: 1,
		CheckedAt:       "2026-08-13T12:00:00Z",
	}

	svc.
		On("CheckAssigneeIntegrity", mock.Anything, service.DefaultIntegrityLimit).
		Return(expectedResp, nil).
		Once()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		nil,
	)

	// Важно: даже при unhealthy ответ должен быть 200 OK
	require.Equal(t, http.StatusOK, w.Code)

	var response dto.IntegrityCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.False(t, response.Healthy, "Should be unhealthy when violations exist")
	assert.Equal(t, 1, response.ViolationsCount)
	assert.Len(t, response.Violations, 1)
	assert.Equal(t, "Violated Team", response.Violations[0].TeamName)

	require.True(t, svc.AssertExpectations(t))
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_InvalidLimit(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		map[string]string{"limit": "not-a-number"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "limit must be a positive integer")

	svc.AssertNotCalled(t, "CheckAssigneeIntegrity", mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_LimitTooSmall(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		map[string]string{"limit": "0"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "limit must be at least 1")

	svc.AssertNotCalled(t, "CheckAssigneeIntegrity", mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_LimitTooLarge(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		map[string]string{"limit": "1500"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "validation failed", response.Error)
	assert.Contains(t, response.Details, "limit cannot exceed 1000")

	svc.AssertNotCalled(t, "CheckAssigneeIntegrity", mock.Anything, mock.Anything)
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_ServiceError(t *testing.T) {
	svc := new(mockAnalyticsService)
	handler := newTestAnalyticsHandler(svc)

	userID := uuid.New()

	dbErr := errors.New("integrity check query failed")

	svc.
		On("CheckAssigneeIntegrity", mock.Anything, service.DefaultIntegrityLimit).
		Return(nil, dbErr).
		Once()

	w := performAnalyticsRequest(
		handler.CheckAssigneeIntegrity,
		http.MethodGet,
		"/api/v1/analytics/integrity/assignees",
		userID,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to perform integrity check", response.Error)

	require.True(t, svc.AssertExpectations(t))
}

// ============================================================================
// Table-driven tests for parameter validation
// ============================================================================

func TestAnalyticsHandler_GetTeamStats_AllBoundaryValues(t *testing.T) {
	tests := []struct {
		name           string
		days           string
		expectedStatus int
		expectedDetail string
	}{
		{
			name:           "minimum valid (1)",
			days:           "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "maximum valid (365)",
			days:           "365",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "just above max (366)",
			days:           "366",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "days cannot exceed 365",
		},
		{
			name:           "just below min (0)",
			days:           "0",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "days must be at least 1",
		},
		{
			name:           "negative",
			days:           "-1",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "days must be at least 1",
		},
		{
			name:           "float",
			days:           "10.5",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "days must be a positive integer",
		},
		{
			name:           "empty string (uses default)",
			days:           "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mockAnalyticsService)
			handler := newTestAnalyticsHandler(svc)

			userID := uuid.New()

			if tt.expectedStatus == http.StatusOK {
				svc.
					On("GetTeamStats", mock.Anything, mock.Anything).
					Return(&dto.TeamStatsResponse{
						Stats:      []dto.TeamStatsItem{},
						TotalTeams: 0,
						DaysPeriod: 1,
					}, nil).
					Maybe()
			}

			queryParams := map[string]string{}
			if tt.days != "" {
				queryParams["days"] = tt.days
			}

			w := performAnalyticsRequest(
				handler.GetTeamStats,
				http.MethodGet,
				"/api/v1/analytics/teams/stats",
				userID,
				queryParams,
			)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusBadRequest {
				var response dto.ErrorResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Contains(t, response.Details, tt.expectedDetail)
			}
		})
	}
}

func TestAnalyticsHandler_GetTopCreators_AllBoundaryValues(t *testing.T) {
	tests := []struct {
		name           string
		months         string
		top            string
		expectedStatus int
		expectedDetail string
	}{
		{
			name:           "both at minimum",
			months:         "1",
			top:            "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "both at maximum",
			months:         "12",
			top:            "10",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "months just above max",
			months:         "13",
			top:            "3",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "months cannot exceed 12",
		},
		{
			name:           "top just above max",
			months:         "1",
			top:            "11",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "top cannot exceed 10",
		},
		{
			name:           "both empty (defaults)",
			months:         "",
			top:            "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mockAnalyticsService)
			handler := newTestAnalyticsHandler(svc)

			userID := uuid.New()

			if tt.expectedStatus == http.StatusOK {
				svc.
					On("GetTopCreators", mock.Anything, mock.Anything, mock.Anything).
					Return(&dto.TopCreatorsResponse{
						Teams:        []dto.TeamTopCreators{},
						TotalTeams:   0,
						MonthsPeriod: 1,
						TopN:         3,
						SinceDate:    "2026-07-20T10:30:00Z",
					}, nil).
					Maybe()
			}

			queryParams := map[string]string{}
			if tt.months != "" {
				queryParams["months"] = tt.months
			}
			if tt.top != "" {
				queryParams["top"] = tt.top
			}

			w := performAnalyticsRequest(
				handler.GetTopCreators,
				http.MethodGet,
				"/api/v1/analytics/teams/top-creators",
				userID,
				queryParams,
			)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusBadRequest {
				var response dto.ErrorResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Contains(t, response.Details, tt.expectedDetail)
			}
		})
	}
}

func TestAnalyticsHandler_CheckAssigneeIntegrity_AllBoundaryValues(t *testing.T) {
	tests := []struct {
		name           string
		limit          string
		expectedStatus int
		expectedDetail string
	}{
		{
			name:           "minimum valid (1)",
			limit:          "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "maximum valid (1000)",
			limit:          "1000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "just above max (1001)",
			limit:          "1001",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "limit cannot exceed 1000",
		},
		{
			name:           "just below min (0)",
			limit:          "0",
			expectedStatus: http.StatusBadRequest,
			expectedDetail: "limit must be at least 1",
		},
		{
			name:           "empty string (uses default)",
			limit:          "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mockAnalyticsService)
			handler := newTestAnalyticsHandler(svc)

			userID := uuid.New()

			if tt.expectedStatus == http.StatusOK {
				svc.
					On("CheckAssigneeIntegrity", mock.Anything, mock.Anything).
					Return(&dto.IntegrityCheckResponse{
						Healthy:         true,
						Violations:      []dto.IntegrityViolationItem{},
						ViolationsCount: 0,
						CheckedAt:       "2026-08-13T10:30:00Z",
					}, nil).
					Maybe()
			}

			queryParams := map[string]string{}
			if tt.limit != "" {
				queryParams["limit"] = tt.limit
			}

			w := performAnalyticsRequest(
				handler.CheckAssigneeIntegrity,
				http.MethodGet,
				"/api/v1/analytics/integrity/assignees",
				userID,
				queryParams,
			)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusBadRequest {
				var response dto.ErrorResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Contains(t, response.Details, tt.expectedDetail)
			}
		})
	}
}
