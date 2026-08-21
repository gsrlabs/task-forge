// internal/handler/teams_test.go
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
// Mock TeamService
// ============================================================================

type mockTeamService struct {
	mock.Mock
}

func (m *mockTeamService) Create(
	ctx context.Context,
	userID uuid.UUID,
	req *dto.CreateTeamRequest,
) (*dto.CreateTeamResponse, error) {
	args := m.Called(ctx, userID, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.CreateTeamResponse), args.Error(1)
}

func (m *mockTeamService) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]dto.TeamListItem, error) {
	args := m.Called(ctx, userID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]dto.TeamListItem), args.Error(1)
}

func (m *mockTeamService) Invite(
	ctx context.Context,
	teamID, inviterID uuid.UUID,
	req *dto.InviteUserRequest,
) (*dto.InviteUserResponse, error) {
	args := m.Called(ctx, teamID, inviterID, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.InviteUserResponse), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestTeamsHandler creates TeamsHandler with mocked service and real validator.
func newTestTeamsHandler(teamSvc *mockTeamService) *TeamsHandler {
	return &TeamsHandler{
		service:   teamSvc,
		validator: validator.NewValidator(),
		logger:    zerolog.Nop(),
	}
}

// performTeamsRequest executes a Gin handler with authentication context.
func performTeamsRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body any,
	userID uuid.UUID,
	pathParams map[string]string,
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

	// Set authenticated user
	if userID != uuid.Nil {
		middleware.SetUserIdentity(c, userID, "user@example.com")
	}

	// Set path parameters
	if len(pathParams) > 0 {
		for key, value := range pathParams {
			c.Params = append(c.Params, gin.Param{Key: key, Value: value})
		}
	}

	handler(c)

	return w
}

// performRawTeamsRequest is used for malformed/empty request bodies.
func performRawTeamsRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body string,
	userID uuid.UUID,
	pathParams map[string]string,
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

	handler(c)

	return w
}

// ============================================================================
// TestTeamsHandler_Create
// ============================================================================

func TestTeamsHandler_Create_Success(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()
	teamID := uuid.New()

	req := dto.CreateTeamRequest{
		Name: "Engineering Team",
	}

	expectedResp := &dto.CreateTeamResponse{
		TeamID: teamID.String(),
		Name:   "Engineering Team",
		Role:   "owner",
	}

	teamSvc.
		On("Create", mock.Anything, userID, mock.MatchedBy(func(r *dto.CreateTeamRequest) bool {
			return r.Name == req.Name
		})).
		Return(expectedResp, nil).
		Once()

	w := performTeamsRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/teams",
		req,
		userID,
		nil,
	)

	require.Equal(t, http.StatusCreated, w.Code)

	var response dto.CreateTeamResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, teamID.String(), response.TeamID)
	assert.Equal(t, "Engineering Team", response.Name)
	assert.Equal(t, "owner", response.Role)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Create_Unauthorized(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	req := dto.CreateTeamRequest{
		Name: "Engineering Team",
	}

	// Pass uuid.Nil to simulate missing user identity
	w := performTeamsRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/teams",
		req,
		uuid.Nil, // Not authenticated
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "internal server error", response.Error)

	// Service should not be called
	teamSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsHandler_Create_InvalidJSON(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()

	w := performRawTeamsRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/teams",
		"{invalid json",
		userID,
		nil,
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)

	teamSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsHandler_Create_ValidationError(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()

	tests := []struct {
		name string
		req  dto.CreateTeamRequest
	}{
		{
			name: "empty name",
			req: dto.CreateTeamRequest{
				Name: "",
			},
		},
		{
			name: "name too long",
			req: dto.CreateTeamRequest{
				Name: string(make([]byte, 256)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performTeamsRequest(
				handler.Create,
				http.MethodPost,
				"/api/v1/teams",
				tt.req,
				userID,
				nil,
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, "validation failed", response.Error)
			assert.Contains(
				t,
				response.Details,
				"team name is required and must be between 1 and 255 characters",
			)

			teamSvc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestTeamsHandler_Create_ServiceError(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()

	req := dto.CreateTeamRequest{
		Name: "Engineering Team",
	}

	dbErr := errors.New("database connection failed")

	teamSvc.
		On("Create", mock.Anything, userID, mock.Anything).
		Return(nil, dbErr).
		Once()

	w := performTeamsRequest(
		handler.Create,
		http.MethodPost,
		"/api/v1/teams",
		req,
		userID,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to create team", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

// ============================================================================
// TestTeamsHandler_List
// ============================================================================

func TestTeamsHandler_List_Success_WithTeams(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()
	teamID1 := uuid.New()
	teamID2 := uuid.New()

	teams := []dto.TeamListItem{
		{
			ID:        teamID1.String(),
			Name:      "Engineering Team",
			Role:      "owner",
			CreatedAt: "2026-08-09T10:30:00Z",
		},
		{
			ID:        teamID2.String(),
			Name:      "Marketing Team",
			Role:      "member",
			CreatedAt: "2026-08-07T15:20:00Z",
		},
	}

	teamSvc.
		On("List", mock.Anything, userID).
		Return(teams, nil).
		Once()

	w := performTeamsRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/teams",
		nil,
		userID,
		nil,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	// Check count
	count, ok := response["count"].(float64)
	require.True(t, ok)
	assert.Equal(t, 2.0, count)

	// Check teams array
	teamsRaw, ok := response["teams"].([]any)
	require.True(t, ok)
	assert.Len(t, teamsRaw, 2)

	// Check first team
	firstTeam, ok := teamsRaw[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, teamID1.String(), firstTeam["id"])
	assert.Equal(t, "Engineering Team", firstTeam["name"])
	assert.Equal(t, "owner", firstTeam["role"])

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_List_Success_EmptyList(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()

	teamSvc.
		On("List", mock.Anything, userID).
		Return([]dto.TeamListItem{}, nil).
		Once()

	w := performTeamsRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/teams",
		nil,
		userID,
		nil,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	count, ok := response["count"].(float64)
	require.True(t, ok)
	assert.Equal(t, 0.0, count)

	teamsRaw, ok := response["teams"].([]any)
	require.True(t, ok)
	assert.Empty(t, teamsRaw)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_List_Unauthorized(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	w := performTeamsRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/teams",
		nil,
		uuid.Nil, // Not authenticated
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	teamSvc.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
}

func TestTeamsHandler_List_ServiceError(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	userID := uuid.New()

	dbErr := errors.New("query timeout")

	teamSvc.
		On("List", mock.Anything, userID).
		Return(nil, dbErr).
		Once()

	w := performTeamsRequest(
		handler.List,
		http.MethodGet,
		"/api/v1/teams",
		nil,
		userID,
		nil,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to list teams", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

// ============================================================================
// TestTeamsHandler_Invite
// ============================================================================

func TestTeamsHandler_Invite_Success(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()
	inviteeID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	expectedResp := &dto.InviteUserResponse{
		Message: "user invited successfully",
		TeamID:  teamID.String(),
		UserID:  inviteeID.String(),
		Role:    "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.MatchedBy(func(r *dto.InviteUserRequest) bool {
			return r.Email == req.Email && r.Role == req.Role
		})).
		Return(expectedResp, nil).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.InviteUserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "user invited successfully", response.Message)
	assert.Equal(t, teamID.String(), response.TeamID)
	assert.Equal(t, inviteeID.String(), response.UserID)
	assert.Equal(t, "member", response.Role)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_AdminRole_Success(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()
	inviteeID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "lead@example.com",
		Role:  "admin",
	}

	expectedResp := &dto.InviteUserResponse{
		Message: "user invited successfully",
		TeamID:  teamID.String(),
		UserID:  inviteeID.String(),
		Role:    "admin",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(expectedResp, nil).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.InviteUserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "admin", response.Role)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_Unauthorized(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		uuid.Nil, // Not authenticated
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	teamSvc.AssertNotCalled(t, "Invite", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsHandler_Invite_InvalidTeamID(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/not-a-uuid/invite",
		req,
		inviterID,
		map[string]string{"id": "not-a-uuid"},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid team ID", response.Error)
	assert.Contains(t, response.Details, "team ID must be a valid UUID")

	// Service should not be called
	teamSvc.AssertNotCalled(t, "Invite", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsHandler_Invite_InvalidJSON(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	w := performRawTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		"{invalid json",
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)

	teamSvc.AssertNotCalled(t, "Invite", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsHandler_Invite_ValidationError(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	tests := []struct {
		name string
		req  dto.InviteUserRequest
	}{
		{
			name: "missing email",
			req: dto.InviteUserRequest{
				Email: "",
				Role:  "member",
			},
		},
		{
			name: "invalid email format",
			req: dto.InviteUserRequest{
				Email: "not-an-email",
				Role:  "member",
			},
		},
		{
			name: "missing role",
			req: dto.InviteUserRequest{
				Email: "colleague@example.com",
				Role:  "",
			},
		},
		{
			name: "invalid role",
			req: dto.InviteUserRequest{
				Email: "colleague@example.com",
				Role:  "invalid-role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performTeamsRequest(
				handler.Invite,
				http.MethodPost,
				"/api/v1/teams/"+teamID.String()+"/invite",
				tt.req,
				inviterID,
				map[string]string{"id": teamID.String()},
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, "validation failed", response.Error)

			teamSvc.AssertNotCalled(
				t,
				"Invite",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			)
		})
	}
}

func TestTeamsHandler_Invite_TeamNotFound(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrTeamNotFound).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusNotFound, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "team not found", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_UserNotFound(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "nonexistent@example.com",
		Role:  "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrUserNotFound).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusNotFound, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "user not found", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_InsufficientPrivilege(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrInsufficientPrivilege).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusForbidden, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "insufficient privilege", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_AlreadyTeamMember(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "existing@example.com",
		Role:  "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrAlreadyTeamMember).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusConflict, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "user is already a member of this team", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_InvalidRole(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	// Note: validation happens at handler level, but service also checks.
	// This test covers the service-level ErrInvalidRole (e.g., trying to invite as owner).
	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "admin", // Will pass handler validation but fail at service
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrInvalidRole).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid role", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_CannotInviteSelf(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "user@example.com",
		Role:  "member",
	}

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, service.ErrCannotInviteSelf).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "cannot invite yourself", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

func TestTeamsHandler_Invite_ServiceError(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	inviterID := uuid.New()
	teamID := uuid.New()

	req := dto.InviteUserRequest{
		Email: "colleague@example.com",
		Role:  "member",
	}

	dbErr := errors.New("unexpected database error")

	teamSvc.
		On("Invite", mock.Anything, teamID, inviterID, mock.Anything).
		Return(nil, dbErr).
		Once()

	w := performTeamsRequest(
		handler.Invite,
		http.MethodPost,
		"/api/v1/teams/"+teamID.String()+"/invite",
		req,
		inviterID,
		map[string]string{"id": teamID.String()},
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "failed to invite user", response.Error)

	require.True(t, teamSvc.AssertExpectations(t))
}

// ============================================================================
// TestTeamsHandler_HandleInviteError (isolated tests)
// ============================================================================

func TestTeamsHandler_HandleInviteError_AllCases(t *testing.T) {
	teamSvc := new(mockTeamService)
	handler := newTestTeamsHandler(teamSvc)

	teamID := uuid.New()
	inviterID := uuid.New()

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "ErrTeamNotFound",
			err:            service.ErrTeamNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "team not found",
		},
		{
			name:           "ErrUserNotFound",
			err:            service.ErrUserNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "user not found",
		},
		{
			name:           "ErrInsufficientPrivilege",
			err:            service.ErrInsufficientPrivilege,
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient privilege",
		},
		{
			name:           "ErrAlreadyTeamMember",
			err:            service.ErrAlreadyTeamMember,
			expectedStatus: http.StatusConflict,
			expectedError:  "user is already a member of this team",
		},
		{
			name:           "ErrInvalidRole",
			err:            service.ErrInvalidRole,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid role",
		},
		{
			name:           "ErrCannotInviteSelf",
			err:            service.ErrCannotInviteSelf,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "cannot invite yourself",
		},
		{
			name:           "Other error",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "failed to invite user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("POST", "/api/v1/teams/"+teamID.String()+"/invite", nil)
			c.Request = req

			handler.handleInviteError(
				c,
				tt.err,
				teamID,
				inviterID,
				"test@example.com",
				"member",
			)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response dto.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

			assert.Equal(t, tt.expectedError, response.Error)
		})
	}
}
