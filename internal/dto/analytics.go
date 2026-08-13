package dto

// TeamStatsResponse response with team statistics.
type TeamStatsResponse struct {
	Stats      []TeamStatsItem `json:"stats"`
	TotalTeams int             `json:"total_teams"`
	DaysPeriod int             `json:"days_period"`
}

// TeamStatsItem is a single team's statistics element.
type TeamStatsItem struct {
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	MembersCount   int    `json:"members_count"`
	DoneTasksCount int    `json:"done_tasks_count"`
	DoneTasksSince string `json:"done_tasks_since"`
}