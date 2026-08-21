//go:build integration
// +build integration

// internal/repository/teams_integration_test.go
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

func TestTeamRepository_Create_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	team := &domain.Team{
		ID:   uuid.New(),
		Name: "Engineering Team",
	}

	assert.True(t, team.CreatedAt.IsZero())
	assert.True(t, team.UpdatedAt.IsZero())

	err := env.teamRepo.Create(env.ctx, owner.ID, team)
	require.NoError(t, err, "Create should succeed")

	assert.False(t, team.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, team.UpdatedAt.IsZero(), "UpdatedAt should be set")

	found, err := env.teamRepo.FindByID(env.ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, team.ID, found.ID)
	assert.Equal(t, "Engineering Team", found.Name)
	assert.Equal(t, owner.ID, found.CreatedBy)

	role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TeamRoleOwner, role,
		"Creator should automatically become owner")
}

func TestTeamRepository_Create_DifferentOwner(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	team := &domain.Team{
		ID:   uuid.New(),
		Name: "Test Team",
	}

	err := env.teamRepo.Create(env.ctx, owner.ID, team)
	require.NoError(t, err)

	found, err := env.teamRepo.FindByID(env.ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, found.CreatedBy,
		"created_by should match the provided userID")
}

func TestTeamRepository_FindByID_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	created := env.createTeam(t, owner.ID, "Test Team")

	found, err := env.teamRepo.FindByID(env.ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Test Team", found.Name)
	assert.Equal(t, owner.ID, found.CreatedBy)
	assert.False(t, found.CreatedAt.IsZero())
	assert.False(t, found.UpdatedAt.IsZero())
}

func TestTeamRepository_FindByID_NotFound(t *testing.T) {
	env := setupPostgres(t)

	nonExistentID := uuid.New()

	found, err := env.teamRepo.FindByID(env.ctx, nonExistentID)
	require.Error(t, err, "Should return error for non-existent team")
	require.ErrorIs(t, err, ErrTeamNotFound,
		"Error should be ErrTeamNotFound")
	assert.Nil(t, found)
}

func TestTeamRepository_FindByID_NilUUID(t *testing.T) {
	env := setupPostgres(t)

	found, err := env.teamRepo.FindByID(env.ctx, uuid.Nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamNotFound)
	assert.Nil(t, found)
}

func TestTeamRepository_FindByUserID_EmptyList(t *testing.T) {
	env := setupPostgres(t)
	user := env.createTestUser(t, randomEmail())

	teams, err := env.teamRepo.FindByUserID(env.ctx, user.ID)
	require.NoError(t, err)

	require.NotNil(t, teams, "Should return empty slice, not nil")
	assert.Empty(t, teams, "User without teams should get empty list")
	assert.Equal(t, 0, len(teams))

	jsonData, err := json.Marshal(teams)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(jsonData), "Should serialize to empty array, not null")
}

func TestTeamRepository_FindByUserID_SingleTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, user.ID, "My Team")

	teams, err := env.teamRepo.FindByUserID(env.ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, teams, 1)

	assert.Equal(t, team.ID, teams[0].ID)
	assert.Equal(t, "My Team", teams[0].Name)
	assert.Equal(t, domain.TeamRoleOwner, teams[0].UserRole)
}

func TestTeamRepository_FindByUserID_MultipleTeams(t *testing.T) {
	env := setupPostgres(t)

	owner1 := env.createTestUser(t, randomEmail())
	owner2 := env.createTestUser(t, randomEmail())
	owner3 := env.createTestUser(t, randomEmail())

	user := env.createTestUser(t, randomEmail())

	team1 := env.createTeam(t, owner1.ID, "Team Alpha")
	team2 := env.createTeam(t, owner2.ID, "Team Beta")
	team3 := env.createTeam(t, owner3.ID, "Team Gamma")

	time.Sleep(10 * time.Millisecond)
	env.addMember(t, team1.ID, user.ID, domain.TeamRoleMember)

	time.Sleep(10 * time.Millisecond)
	env.addMember(t, team2.ID, user.ID, domain.TeamRoleAdmin)

	team4 := env.createTeam(t, user.ID, "Team Delta")

	env.addMember(t, team3.ID, user.ID, domain.TeamRoleMember)

	teams, err := env.teamRepo.FindByUserID(env.ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, teams, 4, "User should be in 4 teams")

	teamIDs := make(map[uuid.UUID]domain.TeamRole)
	for _, t := range teams {
		teamIDs[t.ID] = t.UserRole
	}

	assert.Equal(t, domain.TeamRoleMember, teamIDs[team1.ID])
	assert.Equal(t, domain.TeamRoleAdmin, teamIDs[team2.ID])
	assert.Equal(t, domain.TeamRoleMember, teamIDs[team3.ID])
	assert.Equal(t, domain.TeamRoleOwner, teamIDs[team4.ID])

	for i := 1; i < len(teams); i++ {
		assert.True(
			t,
			teams[i-1].CreatedAt.After(teams[i].CreatedAt) ||
				teams[i-1].CreatedAt.Equal(teams[i].CreatedAt),
			"Teams should be ordered by created_at DESC",
		)
	}
}

func TestTeamRepository_FindByUserID_DoesNotReturnOtherUsersTeams(t *testing.T) {
	env := setupPostgres(t)

	user1 := env.createTestUser(t, randomEmail())
	user2 := env.createTestUser(t, randomEmail())

	env.createTeam(t, user1.ID, "User1 Team")

	env.createTeam(t, user2.ID, "User2 Team")

	teams1, err := env.teamRepo.FindByUserID(env.ctx, user1.ID)
	require.NoError(t, err)
	require.Len(t, teams1, 1)
	assert.Equal(t, "User1 Team", teams1[0].Name)

	teams2, err := env.teamRepo.FindByUserID(env.ctx, user2.ID)
	require.NoError(t, err)
	require.Len(t, teams2, 1)
	assert.Equal(t, "User2 Team", teams2[0].Name)
}

func TestTeamRepository_AddMember_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	member := env.createTestUser(t, randomEmail())

	team := env.createTeam(t, owner.ID, "Test Team")

	newMember := &domain.TeamMember{
		TeamID: team.ID,
		UserID: member.ID,
		Role:   domain.TeamRoleMember,
	}

	err := env.teamRepo.AddMember(env.ctx, newMember)
	require.NoError(t, err, "AddMember should succeed")

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, isMember)

	role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TeamRoleMember, role)
}

func TestTeamRepository_AddMember_AdminRole(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	admin := env.createTestUser(t, randomEmail())

	team := env.createTeam(t, owner.ID, "Test Team")

	err := env.teamRepo.AddMember(env.ctx, &domain.TeamMember{
		TeamID: team.ID,
		UserID: admin.ID,
		Role:   domain.TeamRoleAdmin,
	})
	require.NoError(t, err)

	role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TeamRoleAdmin, role)
}

func TestTeamRepository_AddMember_Duplicate(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	member := env.createTestUser(t, randomEmail())

	team := env.createTeam(t, owner.ID, "Test Team")

	err := env.teamRepo.AddMember(env.ctx, &domain.TeamMember{
		TeamID: team.ID,
		UserID: member.ID,
		Role:   domain.TeamRoleMember,
	})
	require.NoError(t, err)

	err = env.teamRepo.AddMember(env.ctx, &domain.TeamMember{
		TeamID: team.ID,
		UserID: member.ID,
		Role:   domain.TeamRoleAdmin,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamMemberExists,
		"Should return ErrTeamMemberExists on duplicate")
}

func TestTeamRepository_AddMember_NonExistentTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	nonExistentTeamID := uuid.New()

	err := env.teamRepo.AddMember(env.ctx, &domain.TeamMember{
		TeamID: nonExistentTeamID,
		UserID: user.ID,
		Role:   domain.TeamRoleMember,
	})
	require.Error(t, err, "Should fail with FK violation")
	assert.NotErrorIs(t, err, ErrTeamMemberExists)
}

func TestTeamRepository_AddMember_NonExistentUser(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, owner.ID, "Test Team")

	nonExistentUserID := uuid.New()

	err := env.teamRepo.AddMember(env.ctx, &domain.TeamMember{
		TeamID: team.ID,
		UserID: nonExistentUserID,
		Role:   domain.TeamRoleMember,
	})
	require.Error(t, err, "Should fail with FK violation")
}

func TestTeamRepository_GetUserRole_AllRoles(t *testing.T) {
	env := setupPostgres(t)

	roles := []domain.TeamRole{
		domain.TeamRoleOwner,
		domain.TeamRoleAdmin,
		domain.TeamRoleMember,
	}

	for _, expectedRole := range roles {
		t.Run(string(expectedRole), func(t *testing.T) {
			owner := env.createTestUser(t, randomEmail())
			user := env.createTestUser(t, randomEmail())
			team := env.createTeam(t, owner.ID, "Test Team "+string(expectedRole))

			if expectedRole != domain.TeamRoleOwner {
				env.addMember(t, team.ID, user.ID, expectedRole)
			} else {
				user = owner
			}

			role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, user.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedRole, role)
		})
	}
}

func TestTeamRepository_GetUserRole_NotFound(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, owner.ID, "Test Team")

	role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, nonMember.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamMemberNotFound,
		"Should return ErrTeamMemberNotFound for non-member")
	assert.Equal(t, domain.TeamRole(""), role,
		"Role should be empty on error")
}

func TestTeamRepository_GetUserRole_NonExistentTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	nonExistentTeamID := uuid.New()

	role, err := env.teamRepo.GetUserRole(env.ctx, nonExistentTeamID, user.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamMemberNotFound)
	assert.Equal(t, domain.TeamRole(""), role)
}

func TestTeamRepository_RemoveMember_Success(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	member := env.createTestUser(t, randomEmail())

	team := env.createTeam(t, owner.ID, "Test Team")
	env.addMember(t, team.ID, member.ID, domain.TeamRoleMember)

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.True(t, isMember)

	err = env.teamRepo.RemoveMember(env.ctx, team.ID, member.ID)
	require.NoError(t, err, "RemoveMember should succeed")

	isMember, err = env.teamRepo.IsTeamMember(env.ctx, team.ID, member.ID)
	require.NoError(t, err)
	assert.False(t, isMember, "User should no longer be a team member")
}

func TestTeamRepository_RemoveMember_NotFound(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, owner.ID, "Test Team")

	err := env.teamRepo.RemoveMember(env.ctx, team.ID, nonMember.ID)
	require.Error(t, err, "Should return error for non-member")
	require.ErrorIs(t, err, ErrTeamMemberNotFound,
		"Error should be ErrTeamMemberNotFound")
}

func TestTeamRepository_RemoveMember_NonExistentTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	nonExistentTeamID := uuid.New()

	err := env.teamRepo.RemoveMember(env.ctx, nonExistentTeamID, user.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTeamMemberNotFound)
}

func TestTeamRepository_IsTeamMember_True(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, owner.ID, "Test Team")

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isMember, "Owner should be a team member")
}

func TestTeamRepository_IsTeamMember_False(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())
	nonMember := env.createTestUser(t, randomEmail())
	team := env.createTeam(t, owner.ID, "Test Team")

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, nonMember.ID)
	require.NoError(t, err)
	assert.False(t, isMember, "Non-member should return false")
}

func TestTeamRepository_IsTeamMember_NonExistentTeam(t *testing.T) {
	env := setupPostgres(t)

	user := env.createTestUser(t, randomEmail())
	nonExistentTeamID := uuid.New()

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, nonExistentTeamID, user.ID)
	require.NoError(t, err, "Should not error for non-existent team")
	assert.False(t, isMember)
}

func TestTeamRepository_Create_TransactionAtomicity(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	team := &domain.Team{
		ID:   uuid.New(),
		Name: "Atomic Team",
	}

	err := env.teamRepo.Create(env.ctx, owner.ID, team)
	require.NoError(t, err)

	foundTeam, err := env.teamRepo.FindByID(env.ctx, team.ID)
	require.NoError(t, err, "Team should exist")
	assert.NotNil(t, foundTeam)

	role, err := env.teamRepo.GetUserRole(env.ctx, team.ID, owner.ID)
	require.NoError(t, err, "Owner membership should exist")
	assert.Equal(t, domain.TeamRoleOwner, role)

	isMember, err := env.teamRepo.IsTeamMember(env.ctx, team.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestTeamRepository_EmptyTeamName(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	team := &domain.Team{
		ID:   uuid.New(),
		Name: "",
	}

	err := env.teamRepo.Create(env.ctx, owner.ID, team)

	if err == nil {
		found, findErr := env.teamRepo.FindByID(env.ctx, team.ID)
		require.NoError(t, findErr)
		assert.Equal(t, "", found.Name)
	}

}

func TestTeamRepository_VeryLongTeamName(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	longName := "A"
	for i := 0; i < 254; i++ {
		longName += "A"
	}

	team := &domain.Team{
		ID:   uuid.New(),
		Name: longName,
	}

	err := env.teamRepo.Create(env.ctx, owner.ID, team)
	require.NoError(t, err, "Should accept 255 char name")

	found, err := env.teamRepo.FindByID(env.ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, longName, found.Name)
}

func TestTeamRepository_NameTooLong(t *testing.T) {
	env := setupPostgres(t)

	owner := env.createTestUser(t, randomEmail())

	tooLongName := "A"
	for i := 0; i < 255; i++ {
		tooLongName += "A"
	}

	team := &domain.Team{
		ID:   uuid.New(),
		Name: tooLongName,
	}

	err := env.teamRepo.Create(env.ctx, owner.ID, team)
	require.Error(t, err, "Should fail with name too long")
}
