// internal/dto/task.go
package dto

// CreateTaskRequest request to create a task.
type CreateTaskRequest struct {
	TeamID      string  `json:"team_id" validate:"required,uuid"`
	Title       string  `json:"title" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty" validate:"omitempty,uuid"`
}

// UpdateTaskRequest request to update an issue.
type UpdateTaskRequest struct {
	Title       *string `json:"title,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty" validate:"omitempty,oneof=todo in_progress review done"`
	AssigneeID  *string `json:"assignee_id,omitempty" validate:"omitempty,uuid"`
}

// TaskResponse response with the task data.
type TaskResponse struct {
	ID          string  `json:"id"`
	TeamID      string  `json:"team_id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// TaskListResponse response with a list of tasks.
type TaskListResponse struct {
	Tasks  []TaskResponse `json:"tasks"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// TaskHistoryItem element of the revision history.
type TaskHistoryItem struct {
	ID             string                 `json:"id"`
	TaskID         string                 `json:"task_id"`
	ChangedBy      string                 `json:"changed_by"`
	ChangedByEmail string                 `json:"changed_by_email"`
	Action         string                 `json:"action"`
	Changes        map[string]interface{} `json:"changes"`
	ChangedAt      string                 `json:"changed_at"`
}

// TaskHistoryResponse is a response with a history of changes to the issue.
type TaskHistoryResponse struct {
	TaskID  string              `json:"task_id"`
	History []TaskHistoryItem   `json:"history"`
	Total   int                 `json:"total"`
}