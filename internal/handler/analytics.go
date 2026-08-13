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

const (
	defaultAnalyticsDays   = 7
	maxAnalyticsDays       = 365
	defaultAnalyticsMonths = 1
	maxAnalyticsMonths     = 12
	defaultAnalyticsTopN   = 3
	maxAnalyticsTopN       = 10
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


// GetTopCreators processes GET /api/v1/analytics/teams/top-creators
//
// Query parameters:
// - months (optional, default 1, max 12) — period in months
// - top (optional, default 3, max 10) — number of top-creators in each the team
func (h *AnalyticsHandler) GetTopCreators(c *gin.Context) {
	// Parsing and validating the months parameter
	months := defaultAnalyticsMonths
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

		if parsed > maxAnalyticsMonths {
			h.logger.Warn().
				Int("months", parsed).
				Msg("Months parameter exceeds maximum")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: fmt.Sprintf("months cannot exceed %d", maxAnalyticsMonths),
			})
			return
		}

		months = parsed
	}

	// Парсим и валидируем параметр top
	topN := defaultAnalyticsTopN
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

		if parsed > maxAnalyticsTopN {
			h.logger.Warn().
				Int("top", parsed).
				Msg("Top parameter exceeds maximum")

			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "validation failed",
				Details: fmt.Sprintf("top cannot exceed %d", maxAnalyticsTopN),
			})
			return
		}

		topN = parsed
	}

	// Requesting data through the service
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