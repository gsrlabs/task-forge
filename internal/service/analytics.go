// internal/service/analytics.go
package service

import (
	"context"
	"fmt"
	"time"

	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/rs/zerolog"
)

const (
	defaultDaysPeriod   = 7
	maxDaysPeriod       = 365
	defaultMonthsPeriod = 1
	maxMonthsPeriod     = 12
	defaultTopN         = 3
	maxTopN             = 10
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

	// Calculating the start date of the period
	sinceDate := time.Now().AddDate(0, 0, -days)

	// Requesting data from the repository
	stats, err := s.analyticsRepo.GetTeamStats(ctx, days, sinceDate)
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

// GetTopCreators returns the top N task creators in each team.
//
// Business logic:
// 1. Validation and normalization of parameters (months, topN)
// 2. Requesting a flat list from the repository
// 3. Grouping the result by teams
// 4. Creating a DTO with meta information (period, start date)
func (s *analyticsService) GetTopCreators(
	ctx context.Context,
	months int,
	topN int,
) (*dto.TopCreatorsResponse, error) {

	// Normalize the parameters
	if months <= 0 {
		months = defaultMonthsPeriod
	}
	if months > maxMonthsPeriod {
		months = maxMonthsPeriod
	}

	if topN <= 0 {
		topN = defaultTopN
	}
	if topN > maxTopN {
		topN = maxTopN
	}

	// Requesting data from the repository
	creators, err := s.analyticsRepo.GetTopCreators(ctx, months, topN)
	if err != nil {
		s.logger.Error().
			Err(err).
			Int("months", months).
			Int("top_n", topN).
			Msg("Failed to get top creators")
		return nil, fmt.Errorf("get top creators: %w", err)
	}

	// Grouping the flat list by commands (keeping the order from SQL)
	teamsMap := make(map[string]*dto.TeamTopCreators)
	var teamsOrder []string

	for _, creator := range creators {
		team, exists := teamsMap[creator.TeamID]
		if !exists {
			team = &dto.TeamTopCreators{
				TeamID:   creator.TeamID,
				TeamName: creator.TeamName,
				Creators: make([]dto.CreatorStats, 0, topN),
			}
			teamsMap[creator.TeamID] = team
			teamsOrder = append(teamsOrder, creator.TeamID)
		}

		team.Creators = append(team.Creators, dto.CreatorStats{
			UserID:       creator.UserID,
			UserEmail:    creator.UserEmail,
			TasksCreated: creator.TasksCreated,
			Rank:         creator.Rank,
		})
	}

	// Forming the final slice of the teams in the correct order
	teamsDTO := make([]dto.TeamTopCreators, 0, len(teamsOrder))
	for _, teamID := range teamsOrder {
		teamsDTO = append(teamsDTO, *teamsMap[teamID])
	}

	// Calculating the start date of the response period
	sinceDate := time.Now().AddDate(0, -months, 0)

	s.logger.Info().
		Int("teams_count", len(teamsDTO)).
		Int("total_creators", len(creators)).
		Int("months_period", months).
		Int("top_n", topN).
		Msg("Top creators retrieved successfully")

	return &dto.TopCreatorsResponse{
		Teams:        teamsDTO,
		TotalTeams:   len(teamsDTO),
		MonthsPeriod: months,
		TopN:         topN,
		SinceDate:    sinceDate.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
