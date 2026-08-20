package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

type mockTeamRepository struct {
	mock.Mock
}

func (m *mockTeamRepository) Create(ctx context.Context, userID uuid.UUID, team *domain.Team) error {
	args := m.Called(ctx, userID, team)
	return args.Error(0)
}

func (m *mockTeamRepository) FindByID(ctx context.Context, teamID uuid.UUID) (*domain.Team, error) {
	args := m.Called(ctx, teamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Team), args.Error(1)
}

func (m *mockTeamRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.TeamWithRole, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TeamWithRole), args.Error(1)
}

func (m *mockTeamRepository) AddMember(ctx context.Context, member *domain.TeamMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *mockTeamRepository) GetUserRole(ctx context.Context, teamID, userID uuid.UUID) (domain.TeamRole, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Get(0).(domain.TeamRole), args.Error(1)
}

func (m *mockTeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	args := m.Called(ctx, teamID, userID)
	return args.Error(0)
}

func (m *mockTeamRepository) IsTeamMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, teamID, userID)
	return args.Bool(0), args.Error(1)
}

// mockUserRepository is a mock implementation of repository.UserRepository.
type mockUserRepository struct {
	mock.Mock
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestTeamService creates a teamService with injected mocks.
func newTestTeamService(
	teamRepo *mockTeamRepository,
	userRepo *mockUserRepository,
	emailSender *mockEmailSender,
) *teamService {
	return &teamService{
		teamRepo:    teamRepo,
		userRepo:    userRepo,
		emailSender: emailSender,
		logger:      zerolog.Nop(),
	}
}

// ============================================================================
// TestTeamService
// ============================================================================

func TestTeamService_Create_Success(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()
	req := &dto.CreateTeamRequest{
		Name: "Engineering Team",
	}

	teamRepo.On("Create", ctx, userID, mock.MatchedBy(func(team *domain.Team) bool {
		return team.Name == "Engineering Team" && team.ID != uuid.Nil
	})).Return(nil)

	resp, err := svc.Create(ctx, userID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.TeamID, "TeamID should not be empty")
	assert.Equal(t, "Engineering Team", resp.Name)
	assert.Equal(t, string(domain.TeamRoleOwner), resp.Role,
		"Creator should always become owner")

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Create_RepositoryError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()
	req := &dto.CreateTeamRequest{Name: "Failed Team"}

	dbErr := errors.New("database connection lost")
	teamRepo.On("Create", ctx, userID, mock.AnythingOfType("*domain.Team")).
		Return(dbErr)

	resp, err := svc.Create(ctx, userID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "create team")
	assert.True(t, errors.Is(err, dbErr),
		"Original error should be preserved in chain")
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_List_EmptyList(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()

	teamRepo.On("FindByUserID", ctx, userID).
		Return([]domain.TeamWithRole{}, nil)

	result, err := svc.List(ctx, userID)

	require.NoError(t, err)
	require.NotNil(t, result, "Should return empty slice, not nil")
	assert.Empty(t, result)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_List_WithTeams(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()

	createdAt1 := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	createdAt2 := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)

	teams := []domain.TeamWithRole{
		{
			Team: domain.Team{
				ID:        uuid.New(),
				Name:      "Engineering Team",
				CreatedBy: uuid.New(),
				CreatedAt: createdAt1,
				UpdatedAt: createdAt1,
			},
			UserRole: domain.TeamRoleOwner,
		},
		{
			Team: domain.Team{
				ID:        uuid.New(),
				Name:      "Marketing Team",
				CreatedBy: uuid.New(),
				CreatedAt: createdAt2,
				UpdatedAt: createdAt2,
			},
			UserRole: domain.TeamRoleMember,
		},
	}

	teamRepo.On("FindByUserID", ctx, userID).
		Return(teams, nil)

	result, err := svc.List(ctx, userID)

	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, teams[0].ID.String(), result[0].ID)
	assert.Equal(t, "Engineering Team", result[0].Name)
	assert.Equal(t, "owner", result[0].Role)
	assert.Equal(t, "2026-01-15T10:30:00Z", result[0].CreatedAt,
		"CreatedAt should be formatted in ISO 8601")

	assert.Equal(t, teams[1].ID.String(), result[1].ID)
	assert.Equal(t, "Marketing Team", result[1].Name)
	assert.Equal(t, "member", result[1].Role)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_List_RepositoryError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()

	dbErr := errors.New("query timeout")
	teamRepo.On("FindByUserID", ctx, userID).
		Return(nil, dbErr)

	result, err := svc.List(ctx, userID)

	require.Error(t, err)
	assert.ErrorContains(t, err, "list teams")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, result)

	teamRepo.AssertExpectations(t)
}


func TestTeamService_Invite_Success_AsOwner(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{
		Email: "invitee@example.com",
		Role:  string(domain.TeamRoleMember),
	}

	team := &domain.Team{ID: teamID, Name: "Test Team"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}
	inviter := &domain.User{ID: inviterID, Email: "owner@example.com"}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.MatchedBy(func(m *domain.TeamMember) bool {
		return m.TeamID == teamID &&
			m.UserID == inviteeID &&
			m.Role == domain.TeamRoleMember
	})).Return(nil)
	userRepo.On("FindByID", ctx, inviterID).Return(inviter, nil)
	emailSender.On("SendInvitation", mock.MatchedBy(func(data dto.InvitationEmailData) bool {
		return data.RecipientEmail == req.Email &&
			data.TeamName == "Test Team" &&
			data.InviterName == "owner@example.com" &&
			data.Role == string(domain.TeamRoleMember)
	})).Return(nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "user invited successfully", resp.Message)
	assert.Equal(t, teamID.String(), resp.TeamID)
	assert.Equal(t, inviteeID.String(), resp.UserID)
	assert.Equal(t, string(domain.TeamRoleMember), resp.Role)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	emailSender.AssertExpectations(t)
}

func TestTeamService_Invite_Success_AsAdmin(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{
		Email: "newadmin@example.com",
		Role:  string(domain.TeamRoleAdmin),
	}

	team := &domain.Team{ID: teamID, Name: "Admin Team"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}
	inviter := &domain.User{ID: inviterID, Email: "admin@example.com"}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleAdmin, nil) // admin, not owner
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.MatchedBy(func(m *domain.TeamMember) bool {
		return m.Role == domain.TeamRoleAdmin
	})).Return(nil)
	userRepo.On("FindByID", ctx, inviterID).Return(inviter, nil)
	emailSender.On("SendInvitation", mock.Anything).Return(nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, string(domain.TeamRoleAdmin), resp.Role)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	emailSender.AssertExpectations(t)
}


func TestTeamService_Invite_TeamNotFound(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "x@y.com", Role: "member"}

	teamRepo.On("FindByID", ctx, teamID).
		Return(nil, repository.ErrTeamNotFound)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamNotFound)
	assert.Nil(t, resp)

	userRepo.AssertNotCalled(t, "FindByEmail", mock.Anything, mock.Anything)
	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)
	emailSender.AssertNotCalled(t, "SendInvitation", mock.Anything)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_FindByIDUnexpectedError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "x@y.com", Role: "member"}

	dbErr := errors.New("connection timeout")
	teamRepo.On("FindByID", ctx, teamID).Return(nil, dbErr)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "find team")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_InviterNotMember(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "x@y.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInsufficientPrivilege)
	assert.Nil(t, resp)

	userRepo.AssertNotCalled(t, "FindByEmail", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_InsufficientPrivilege_MemberRole(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "x@y.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleMember, nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInsufficientPrivilege,
		"Regular members should not be able to invite")
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_GetUserRoleUnexpectedError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "x@y.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	dbErr := errors.New("db error")

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRole(""), dbErr)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "get inviter role")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_InviteeNotFound(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "nonexistent@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_FindByEmailUnexpectedError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "error@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	dbErr := errors.New("user db error")

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(nil, dbErr)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "find invitee")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_CannotInviteSelf(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	req := &dto.InviteUserRequest{Email: "self@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviterID, Email: req.Email}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrCannotInviteSelf,
		"Should prevent self-invitation")
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_InvalidRole_Owner(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{
		Email: "wannabe-owner@example.com",
		Role:  string(domain.TeamRoleOwner),
	}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidRole,
		"Cannot invite as owner — that's the creator's privilege")
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_AlreadyMember(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "member@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRoleMember, nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrAlreadyTeamMember,
		"Should detect existing membership")
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_AlreadyMember_RaceCondition(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "race@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).
		Return(repository.ErrTeamMemberExists)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrAlreadyTeamMember,
		"Should map repository.ErrTeamMemberExists to service.ErrAlreadyTeamMember")
	assert.Nil(t, resp)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_AddMemberUnexpectedError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "err@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}
	dbErr := errors.New("insert failed")

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).
		Return(dbErr)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "add team member")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	emailSender.AssertNotCalled(t, "SendInvitation", mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_EmailSenderFails_StillSucceeds(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "user@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test Team"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}
	inviter := &domain.User{ID: inviterID, Email: "owner@example.com"}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).
		Return(nil)
	userRepo.On("FindByID", ctx, inviterID).Return(inviter, nil)
	emailSender.On("SendInvitation", mock.Anything).
		Return(errors.New("SMTP connection refused"))

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.NoError(t, err, "Email failure should NOT fail the invite operation")
	require.NotNil(t, resp)
	assert.Equal(t, "user invited successfully", resp.Message)
	assert.Equal(t, inviteeID.String(), resp.UserID)

	emailSender.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Invite_FindInviterByIDFails_StillSucceeds(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "user@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test Team"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), repository.ErrTeamMemberNotFound)
	teamRepo.On("AddMember", ctx, mock.AnythingOfType("*domain.TeamMember")).
		Return(nil)
	userRepo.On("FindByID", ctx, inviterID).
		Return(nil, errors.New("user not found"))
	emailSender.On("SendInvitation", mock.MatchedBy(func(data dto.InvitationEmailData) bool {
		return data.InviterName == ""
	})).Return(nil)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.NoError(t, err, "Inviter lookup failure should NOT fail the invite")
	require.NotNil(t, resp)

	emailSender.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	teamRepo.AssertExpectations(t)
}

func TestTeamService_Invite_MembershipCheckUnexpectedError(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	teamID := uuid.New()
	inviterID := uuid.New()
	inviteeID := uuid.New()
	req := &dto.InviteUserRequest{Email: "user@example.com", Role: "member"}

	team := &domain.Team{ID: teamID, Name: "Test"}
	invitee := &domain.User{ID: inviteeID, Email: req.Email}
	dbErr := errors.New("membership check failed")

	teamRepo.On("FindByID", ctx, teamID).Return(team, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviterID).
		Return(domain.TeamRoleOwner, nil)
	userRepo.On("FindByEmail", ctx, req.Email).Return(invitee, nil)
	teamRepo.On("GetUserRole", ctx, teamID, inviteeID).
		Return(domain.TeamRole(""), dbErr)

	resp, err := svc.Invite(ctx, teamID, inviterID, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "check membership")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	teamRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)

	teamRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestTeamService_Create_GeneratesUUID(t *testing.T) {
	teamRepo := new(mockTeamRepository)
	userRepo := new(mockUserRepository)
	emailSender := new(mockEmailSender)
	svc := newTestTeamService(teamRepo, userRepo, emailSender)

	ctx := context.Background()
	userID := uuid.New()
	req := &dto.CreateTeamRequest{Name: "UUID Test Team"}

	var capturedTeam *domain.Team
	teamRepo.On("Create", ctx, userID, mock.AnythingOfType("*domain.Team")).
		Return(nil).
		Run(func(args mock.Arguments) {
			capturedTeam = args.Get(2).(*domain.Team)
		})

	resp, err := svc.Create(ctx, userID, req)
	require.NoError(t, err)
	require.NotNil(t, capturedTeam)

	assert.NotEqual(t, uuid.Nil, capturedTeam.ID,
		"Create should generate a non-nil UUID for the team")
	assert.Equal(t, capturedTeam.ID.String(), resp.TeamID,
		"Response TeamID should match generated UUID")

	teamRepo.AssertExpectations(t)
}