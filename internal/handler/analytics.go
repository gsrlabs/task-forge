package handler

import (
	"net/http"
	"strconv"

	"task-forge/internal/dto"
	"task-forge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	defaultAnalyticsDays = 7
	maxAnalyticsDays     = 365
)

// AnalyticsHandler handles analytical queries.
type AnalyticsHandler struct {
	service service.AnalyticsService
	logger  zerolog.Logger
}

// NewAnalyticsHandler creates an instance of AnalyticsHandler.
func NewAnalyticsHandler(
	service service.AnalyticsService,
	logger zerolog.Logger,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		service: service,
		logger:  logger,
	}
}

// GetTeamStats processes GET /api/v1/analytics/teams/stats
// Query parameters:
//   - days (optional, default 7, max 365) — the period in days for task statistics
func (h *AnalyticsHandler) GetTeamStats(c *gin.Context) {
	// Парсим query параметр days
	days := defaultAnalyticsDays
	if daysStr := c.Query("days"); daysStr != "" {
		parsed, err := strconv.Atoi(daysStr)
		if err != nil {
			h.logger.Warn().
				Err(err).
				Str("days", daysStr).
				Msg("Invalid days query parameter")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "days must be a positive integer",
			})
			return
		}

		if parsed < 1 {
			h.logger.Warn().
				Int("days", parsed).
				Msg("Days parameter must be positive")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "days must be at least 1",
			})
			return
		}

		if parsed > maxAnalyticsDays {
			h.logger.Warn().
				Int("days", parsed).
				Msg("Days parameter exceeds maximum")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "days cannot exceed 365",
			})
			return
		}

		days = parsed
	}

	// Requesting statistics through the service
	response, err := h.service.GetTeamStats(c.Request.Context(), days)
	if err != nil {
		h.logger.Error().
			Err(err).
			Int("days", days).
			Msg("Failed to get team stats")

		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to retrieve team statistics",
		})
		return
	}

	h.logger.Info().
		Int("teams_count", response.TotalTeams).
		Int("days_period", response.DaysPeriod).
		Msg("Team stats retrieved successfully")

	c.JSON(http.StatusOK, response)
}