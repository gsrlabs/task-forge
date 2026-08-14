// internal/service/teams.go
package service

import (
	"context"
	"errors"
	"fmt"

	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type teamService struct {
	teamRepo    repository.TeamRepository
	userRepo    repository.UserRepository
	emailSender EmailSender
	logger      zerolog.Logger
}

// NewTeamService creates an instance of TeamService.
func NewTeamService(
	teamRepo repository.TeamRepository,
	userRepo repository.UserRepository,
	emailSender EmailSender,
	logger zerolog.Logger,
) TeamService {
	return &teamService{
		teamRepo:    teamRepo,
		userRepo:    userRepo,
		emailSender: emailSender,
		logger:      logger,
	}
}

// Create creates a new command. The current user automatically becomes the owner.
func (s *teamService) Create(
	ctx context.Context,
	userID uuid.UUID,
	req *dto.CreateTeamRequest,
) (*dto.CreateTeamResponse, error) {
	team := &domain.Team{
		ID:   uuid.New(),
		Name: req.Name,
	}

	if err := s.teamRepo.Create(ctx, userID, team); err != nil {
		s.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Str("team_name", req.Name).
			Msg("Failed to create team")
		return nil, fmt.Errorf("create team: %w", err)
	}

	s.logger.Info().
		Str("team_id", team.ID.String()).
		Str("team_name", team.Name).
		Str("owner_id", userID.String()).
		Msg("Team created successfully")

	return &dto.CreateTeamResponse{
		TeamID: team.ID.String(),
		Name:   team.Name,
		Role:   string(domain.TeamRoleOwner),
	}, nil
}

// List returns a list of teams the user is a member of, along with their role.
func (s *teamService) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]dto.TeamListItem, error) {
	teams, err := s.teamRepo.FindByUserID(ctx, userID)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to list user teams")
		return nil, fmt.Errorf("list teams: %w", err)
	}

	result := make([]dto.TeamListItem, 0, len(teams))
	for _, t := range teams {
		result = append(result, dto.TeamListItem{
			ID:        t.ID.String(),
			Name:      t.Name,
			Role:      string(t.UserRole),
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	s.logger.Debug().
		Str("user_id", userID.String()).
		Int("count", len(result)).
		Msg("Teams listed successfully")

	return result, nil
}

// Invite invites a user to the team.
// Checks:
//  1. The existence of the team
//  2. The rights of the initiator (only owner or admin)
//  3. The existence of the invited user via email
//  4. That the invited user is not already part of the team
//  5. The validity of the role (cannot be invited as owner)
//  6. That the initiator is not inviting themselves
func (s *teamService) Invite(
	ctx context.Context,
	teamID, inviterID uuid.UUID,
	req *dto.InviteUserRequest,
) (*dto.InviteUserResponse, error) {

	// 1. Check if the command exists
	team, err := s.teamRepo.FindByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		s.logger.Error().Err(err).Str("team_id", teamID.String()).Msg("Failed to find team")
		return nil, fmt.Errorf("find team: %w", err)
	}

	// 2. Check the initiator’s rights
	inviterRole, err := s.teamRepo.GetUserRole(ctx, teamID, inviterID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamMemberNotFound) {
			s.logger.Warn().
				Str("team_id", teamID.String()).
				Str("inviter_id", inviterID.String()).
				Msg("Non-member attempted to invite user")
			return nil, ErrInsufficientPrivilege
		}
		s.logger.Error().Err(err).Msg("Failed to get inviter role")
		return nil, fmt.Errorf("get inviter role: %w", err)
	}

	if inviterRole != domain.TeamRoleOwner && inviterRole != domain.TeamRoleAdmin {
		s.logger.Warn().
			Str("team_id", teamID.String()).
			Str("inviter_id", inviterID.String()).
			Str("inviter_role", string(inviterRole)).
			Msg("Insufficient privilege for invite")
		return nil, ErrInsufficientPrivilege
	}

	// 3. Find the invited user by email
	invitee, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.Warn().
				Str("email", req.Email).
				Msg("Attempt to invite non-existent user")
			return nil, ErrUserNotFound
		}
		s.logger.Error().Err(err).Str("email", req.Email).Msg("Failed to find invitee")
		return nil, fmt.Errorf("find invitee: %w", err)
	}

	// 4. You can’t invite yourself.
	if invitee.ID == inviterID {
		s.logger.Warn().
			Str("user_id", inviterID.String()).
			Msg("User attempted to invite themselves")
		return nil, ErrCannotInviteSelf
	}

	// 5. Validate the role (you can’t be invited as an owner — that’s the creator’s privilege)
	inviteRole := domain.TeamRole(req.Role)
	if inviteRole != domain.TeamRoleAdmin && inviteRole != domain.TeamRoleMember {
		s.logger.Warn().
			Str("role", req.Role).
			Msg("Invalid role for invite")
		return nil, ErrInvalidRole
	}

	// 6. Check that the user is not yet part of the team
	existingRole, err := s.teamRepo.GetUserRole(ctx, teamID, invitee.ID)
	if err != nil && !errors.Is(err, repository.ErrTeamMemberNotFound) {
		s.logger.Error().Err(err).Msg("Failed to check existing membership")
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if existingRole != "" {
		s.logger.Warn().
			Str("team_id", teamID.String()).
			Str("user_id", invitee.ID.String()).
			Msg("User is already a team member")
		return nil, ErrAlreadyTeamMember
	}

	// 7. Add a participant to the team
	member := &domain.TeamMember{
		TeamID: teamID,
		UserID: invitee.ID,
		Role:   inviteRole,
	}

	if err := s.teamRepo.AddMember(ctx, member); err != nil {
		if errors.Is(err, repository.ErrTeamMemberExists) {
			return nil, ErrAlreadyTeamMember
		}
		s.logger.Error().Err(err).Msg("Failed to add team member")
		return nil, fmt.Errorf("add team member: %w", err)
	}

	s.logger.Info().
		Str("team_id", teamID.String()).
		Str("team_name", team.Name).
		Str("inviter_id", inviterID.String()).
		Str("inviter_role", string(inviterRole)).
		Str("invitee_id", invitee.ID.String()).
		Str("invitee_email", req.Email).
		Str("assigned_role", req.Role).
		Msg("User invited to team successfully")

	if err := s.emailSender.SendMessage(req.Email, team.Name); err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	return &dto.InviteUserResponse{
		Message: "user invited successfully",
		TeamID:  teamID.String(),
		UserID:  invitee.ID.String(),
		Role:    req.Role,
	}, nil

	
}
