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

// TopCreatorsResponse is a response with a list of top task creators grouped by teams.
type TopCreatorsResponse struct {
	Teams        []TeamTopCreators `json:"teams"`
	TotalTeams   int               `json:"total_teams"`
	MonthsPeriod int               `json:"months_period"`
	TopN         int               `json:"top_n"`
	SinceDate    string            `json:"since_date"`
}

// TeamTopCreators are top creators of tasks on the same team.
type TeamTopCreators struct {
	TeamID   string         `json:"team_id"`
	TeamName string         `json:"team_name"`
	Creators []CreatorStats `json:"creators"`
}

// CreatorStats statistics of a single task creator.
type CreatorStats struct {
	UserID       string `json:"user_id"`
	UserEmail    string `json:"user_email"`
	TasksCreated int    `json:"tasks_created"`
	Rank         int    `json:"rank"`
}

// IntegrityCheckResponse is the response with the result of the integrity check.
type IntegrityCheckResponse struct {
	Healthy         bool                     `json:"healthy"`
	Violations      []IntegrityViolationItem `json:"violations"`
	ViolationsCount int                      `json:"violations_count"`
	CheckedAt       string                   `json:"checked_at"`
}

// IntegrityViolationItem is an element of the integrity violations list.
type IntegrityViolationItem struct {
	TaskID         string `json:"task_id"`
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	TaskTitle      string `json:"task_title"`
	TaskStatus     string `json:"task_status"`
	AssigneeID     string `json:"assignee_id"`
	AssigneeEmail  string `json:"assignee_email"`
	CreatedByID    string `json:"created_by_id"`
	CreatedByEmail string `json:"created_by_email"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}
