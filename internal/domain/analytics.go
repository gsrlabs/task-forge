package domain

import "time"

// TeamStats team statistics with aggregated data.
type TeamStats struct {
	TeamID         string    `json:"team_id"`
	TeamName       string    `json:"team_name"`
	MembersCount   int       `json:"members_count"`
	DoneTasksCount int       `json:"done_tasks_count"`
	DoneTasksSince time.Time `json:"done_tasks_since"`
}
