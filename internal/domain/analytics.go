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