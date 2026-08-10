// internal/handler/teams.go
package handler

import (
	"errors"
	"net/http"

	"task-forge/internal/dto"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TeamsHandler processes team management requests.
type TeamsHandler struct {
	service   service.TeamService
	validator *validator.Validator
	logger    zerolog.Logger
}

// NewTeamsHandler creates an instance of TeamsHandler.
func NewTeamsHandler(
	service service.TeamService,
	validator *validator.Validator,
	logger zerolog.Logger,
) *TeamsHandler {
	logger.Debug().
		Bool("service_nil", service == nil).
		Bool("validator_nil", validator == nil).
		Msg("Creating TeamsHandler")
	return &TeamsHandler{
		service:   service,
		validator: validator,
		logger:    logger,
	}
}

// Create processes POST /api/v1/teams — creating a new team.
// The current user automatically becomes the owner.
func (h *TeamsHandler) Create(c *gin.Context) {
	// Извлекаем userID из JWT cookie
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// Parse and validate the request
	var req dto.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invalid create team request")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Create team validation failed")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "team name is required and must be between 1 and 255 characters",
		})
		return
	}

	// We create a team via the service
	response, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Str("team_name", req.Name).
			Msg("Failed to create team")
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to create team",
		})
		return
	}

	h.logger.Info().
		Str("user_id", userID.String()).
		Str("team_id", response.TeamID).
		Str("team_name", response.Name).
		Msg("Team created successfully")

	c.JSON(http.StatusCreated, response)
}

// List processes GET /api/v1/teams — retrieving the list of user teams.
func (h *TeamsHandler) List(c *gin.Context) {
	// Extract the userID from the JWT cookie
	userID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	// We receive the list of commands via the service
	teams, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to list teams")
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to list teams",
		})
		return
	}

	h.logger.Debug().
		Str("user_id", userID.String()).
		Int("count", len(teams)).
		Msg("Teams listed successfully")

	c.JSON(http.StatusOK, gin.H{
		"teams": teams,
		"count": len(teams),
	})
}

// Invite processes POST /api/v1/teams/:id/invite — inviting a user to a team.
// Only the owner and admin can invite new participants.
func (h *TeamsHandler) Invite(c *gin.Context) {
	// Extract the initiator’s userID from the JWT cookie
	inviterID, ok := getAuthenticatedUserID(c, h.logger)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.logger.Warn().
			Err(err).
			Str("team_id", c.Param("id")).
			Msg("Invalid team ID format")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid team ID",
			Details: "team ID must be a valid UUID",
		})
		return
	}

	// Parse and validate the request
	var req dto.InviteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invalid invite request")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invite validation failed")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: "email and role are required, role must be 'admin' or 'member'",
		})
		return
	}

	// We invite the user through the service.
	response, err := h.service.Invite(c.Request.Context(), teamID, inviterID, &req)
	if err != nil {
		h.handleInviteError(
			c,
			err,
			teamID,
			inviterID,
			req.Email,
			req.Role,
		)
		return
	}

	h.logger.Info().
		Str("team_id", teamID.String()).
		Str("inviter_id", inviterID.String()).
		Str("invitee_email", req.Email).
		Str("assigned_role", req.Role).
		Msg("User invited to team successfully")

	c.JSON(http.StatusOK, response)
}

// handleInviteError handles errors in the Invite service and returns the corresponding HTTP status.
func (h *TeamsHandler) handleInviteError(
	c *gin.Context,
	err error,
	teamID uuid.UUID,
	inviterID uuid.UUID,
	inviteeEmail string,
	role string,
) {
	switch {
	case errors.Is(err, service.ErrTeamNotFound):
		h.logger.Warn().
			Str("team_id", teamID.String()).
			Str("inviter_id", inviterID.String()).
			Msg("Team not found during invite")
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "team not found",
		})

	case errors.Is(err, service.ErrUserNotFound):
		h.logger.Warn().
			Str("email", inviteeEmail).
			Str("inviter_id", inviterID.String()).
			Msg("Invitee user not found")
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error: "user not found",
		})

	case errors.Is(err, service.ErrInsufficientPrivilege):
		h.logger.Warn().
			Str("team_id", teamID.String()).
			Str("inviter_id", inviterID.String()).
			Msg("Insufficient privilege for invite")
		c.JSON(http.StatusForbidden, dto.ErrorResponse{
			Error: "insufficient privilege",
		})

	case errors.Is(err, service.ErrAlreadyTeamMember):
		h.logger.Warn().
			Str("team_id", teamID.String()).
			Str("email", inviteeEmail).
			Msg("User is already a team member")
		c.JSON(http.StatusConflict, dto.ErrorResponse{
			Error: "user is already a member of this team",
		})

	case errors.Is(err, service.ErrInvalidRole):
		h.logger.Warn().
			Str("role", role).
			Str("team_id", teamID.String()).
			Str("inviter_id", inviterID.String()).
			Msg("Invalid role for invite")

		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid role",
		})

	case errors.Is(err, service.ErrCannotInviteSelf):
		h.logger.Warn().
			Str("inviter_id", inviterID.String()).
			Msg("User attempted to invite themselves")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "cannot invite yourself",
		})

	default:
		h.logger.Error().
			Err(err).
			Str("team_id", teamID.String()).
			Str("inviter_id", inviterID.String()).
			Str("invitee_email", inviteeEmail).
			Msg("Failed to invite user to team")
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to invite user",
		})
	}
}
