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

// TopCreator is information about the user with the number of created tasks.
type TopCreator struct {
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
	UserID       string `json:"user_id"`
	UserEmail    string `json:"user_email"`
	TasksCreated int    `json:"tasks_created"`
	Rank         int    `json:"rank"`
}

// IntegrityViolation represents a violation of data integrity:
// a task where the assignee is not a member of the task team.
type IntegrityViolation struct {
	TaskID        string    `json:"task_id"`
	TeamID        string    `json:"team_id"`
	TeamName      string    `json:"team_name"`
	TaskTitle     string    `json:"task_title"`
	TaskStatus    string    `json:"task_status"`
	AssigneeID    string    `json:"assignee_id"`
	AssigneeEmail string    `json:"assignee_email"`
	CreatedByID   string    `json:"created_by_id"`
	CreatedByEmail string   `json:"created_by_email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}