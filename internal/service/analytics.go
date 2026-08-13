package service

import (
	"context"
	"fmt"

	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/rs/zerolog"
)

const (
	defaultDaysPeriod = 7
	maxDaysPeriod     = 365
)

type analyticsService struct {
	analyticsRepo repository.AnalyticsRepository
	logger        zerolog.Logger
}

// NewAnalyticsService creates an instance of AnalyticsService.
func NewAnalyticsService(
	analyticsRepo repository.AnalyticsRepository,
	logger zerolog.Logger,
) AnalyticsService {
	return &analyticsService{
		analyticsRepo: analyticsRepo,
		logger:        logger,
	}
}

// GetTeamStats returns statistics of commands with validation of the days parameter.
func (s *analyticsService) GetTeamStats(
	ctx context.Context,
	days int,
) (*dto.TeamStatsResponse, error) {

	if days <= 0 {
		days = defaultDaysPeriod
	}
	if days > maxDaysPeriod {
		days = maxDaysPeriod
	}

	// Requesting data from the repository
	stats, err := s.analyticsRepo.GetTeamStats(ctx, days)
	if err != nil {
		s.logger.Error().
			Err(err).
			Int("days", days).
			Msg("Failed to get team stats")
		return nil, fmt.Errorf("get team stats: %w", err)
	}

	// Mapim domain → DTO
	statsDTO := make([]dto.TeamStatsItem, 0, len(stats))
	for _, stat := range stats {
		statsDTO = append(statsDTO, dto.TeamStatsItem{
			TeamID:         stat.TeamID,
			TeamName:       stat.TeamName,
			MembersCount:   stat.MembersCount,
			DoneTasksCount: stat.DoneTasksCount,
			DoneTasksSince: stat.DoneTasksSince.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	s.logger.Info().
		Int("teams_count", len(statsDTO)).
		Int("days_period", days).
		Msg("Team stats retrieved successfully")

	return &dto.TeamStatsResponse{
		Stats:      statsDTO,
		TotalTeams: len(statsDTO),
		DaysPeriod: days,
	}, nil
}