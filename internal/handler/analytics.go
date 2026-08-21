// internal/handler/analytics.go
package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"task-forge/internal/dto"
	"task-forge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
	days := service.DefaultDaysPeriod
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

		if parsed > service.MaxDaysPeriod {
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

// GetTopCreators processes GET /api/v1/analytics/teams/top-creators
//
// Query parameters:
// - months (optional, default 1, max 12) — period in months
// - top (optional, default 3, max 10) — number of top-creators in each the team
func (h *AnalyticsHandler) GetTopCreators(c *gin.Context) {
	months := service.DefaultMonthsPeriod
	if monthsStr := c.Query("months"); monthsStr != "" {
		parsed, err := strconv.Atoi(monthsStr)
		if err != nil {
			h.logger.Warn().
				Err(err).
				Str("months", monthsStr).
				Msg("Invalid months query parameter")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "months must be a positive integer",
			})
			return
		}

		if parsed < 1 {
			h.logger.Warn().
				Int("months", parsed).
				Msg("Months parameter must be at least 1")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "months must be at least 1",
			})
			return
		}

		if parsed > service.MaxMonthsPeriod {
			h.logger.Warn().
				Int("months", parsed).
				Msg("Months parameter exceeds maximum")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: fmt.Sprintf("months cannot exceed %d", service.MaxMonthsPeriod),
			})
			return
		}

		months = parsed
	}

	topN := service.DefaultTopN
	if topStr := c.Query("top"); topStr != "" {
		parsed, err := strconv.Atoi(topStr)
		if err != nil {
			h.logger.Warn().
				Err(err).
				Str("top", topStr).
				Msg("Invalid top query parameter")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "top must be a positive integer",
			})
			return
		}

		if parsed < 1 {
			h.logger.Warn().
				Int("top", parsed).
				Msg("Top parameter must be at least 1")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "top must be at least 1",
			})
			return
		}

		if parsed > service.MaxTopN {
			h.logger.Warn().
				Int("top", parsed).
				Msg("Top parameter exceeds maximum")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: fmt.Sprintf("top cannot exceed %d", service.MaxTopN),
			})
			return
		}

		topN = parsed
	}

	response, err := h.service.GetTopCreators(c.Request.Context(), months, topN)
	if err != nil {
		h.logger.Error().
			Err(err).
			Int("months", months).
			Int("top_n", topN).
			Msg("Failed to get top creators")

		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to retrieve top creators statistics",
		})
		return
	}

	h.logger.Info().
		Int("teams_count", response.TotalTeams).
		Int("months_period", response.MonthsPeriod).
		Int("top_n", response.TopN).
		Msg("Top creators retrieved successfully")

	c.JSON(http.StatusOK, response)
}

// CheckAssigneeIntegrity processes GET /api/v1/analytics/integrity/assignees
//
// Query parameters:
// - limit (optional, default 100, max 1000) — maximum number of violations in the response
func (h *AnalyticsHandler) CheckAssigneeIntegrity(c *gin.Context) {

	limit := service.DefaultIntegrityLimit
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			h.logger.Warn().
				Err(err).
				Str("limit", limitStr).
				Msg("Invalid limit query parameter for integrity check")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "limit must be a positive integer",
			})
			return
		}

		if parsed < 1 {
			h.logger.Warn().
				Int("limit", parsed).
				Msg("Limit parameter must be at least 1")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: "limit must be at least 1",
			})
			return
		}

		if parsed > service.MaxIntegrityLimit {
			h.logger.Warn().
				Int("limit", parsed).
				Msg("Limit parameter exceeds maximum for integrity check")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: fmt.Sprintf("limit cannot exceed %d", service.MaxIntegrityLimit),
			})
			return
		}

		limit = parsed
	}

	response, err := h.service.CheckAssigneeIntegrity(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error().
			Err(err).
			Int("limit", limit).
			Msg("Failed to perform assignee integrity check")

		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "failed to perform integrity check",
		})
		return
	}

	if response.Healthy {
		h.logger.Info().
			Int("limit", limit).
			Msg("Assignee integrity check completed: no violations")
	} else {
		h.logger.Warn().
			Int("violations_count", response.ViolationsCount).
			Int("limit", limit).
			Msg("Assignee integrity check completed: violations found")
	}

	c.JSON(http.StatusOK, response)
}
