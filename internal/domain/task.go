// internal/domain/task.go
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Enums
// ============================================================================

// TaskStatus is the status of the task.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
)

// IsValid verifies that the status is valid.
func (s TaskStatus) IsValid() bool {
	switch s {
	case TaskStatusTodo,
		TaskStatusInProgress,
		TaskStatusReview,
		TaskStatusDone:
		return true
	default:
		return false
	}
}

// TaskHistoryAction is the type of action in the history.
type TaskHistoryAction string

const (
	TaskHistoryActionCreated TaskHistoryAction = "created"
	TaskHistoryActionUpdated TaskHistoryAction = "updated"
	TaskHistoryActionDeleted TaskHistoryAction = "deleted"
)

// ============================================================================
// Models
// ============================================================================

// Task is the main task model.
type Task struct {
	ID          uuid.UUID  `json:"id"`
	TeamID      uuid.UUID  `json:"team_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskHistory records the history of task changes.
type TaskHistory struct {
	ID        uuid.UUID         `json:"id"`
	TaskID    uuid.UUID         `json:"task_id"`
	ChangedBy uuid.UUID         `json:"changed_by"`
	Action    TaskHistoryAction `json:"action"`
	Changes   json.RawMessage   `json:"changes"`
	ChangedAt time.Time         `json:"changed_at"`
}

// TaskHistoryWithUser contains history together with user information.
type TaskHistoryWithUser struct {
	TaskHistory
	ChangedByEmail string `json:"changed_by_email"`
}

// ============================================================================
// Task update command
// ============================================================================

// TaskUpdate describes a prepared update operation.
// The service is responsible for deciding which fields are allowed to change.
// The repository only persists this already-validated command.
type TaskUpdate struct {
	Title       *string
	TitleSet    bool

	Description *string
	DescriptionSet bool

	Status    *TaskStatus
	StatusSet bool

	AssigneeID    *uuid.UUID
	AssigneeIDSet bool
}

// ============================================================================
// Audit
// ============================================================================

// TaskAudit contains already prepared audit information.
// The service decides what changed and what should be recorded.
// The repository only persists it atomically with the task update.
type TaskAudit struct {
	Action  TaskHistoryAction
	Changes json.RawMessage
}

// ============================================================================
// Filter & Pagination
// ============================================================================

// TaskFilter filters the task list.
type TaskFilter struct {
	TeamID     *uuid.UUID
	Status     *TaskStatus
	AssigneeID *uuid.UUID
}

// TaskPagination contains database pagination parameters.
type TaskPagination struct {
	Limit  int
	Offset int
}

// DefaultTaskPagination returns default pagination values.
func DefaultTaskPagination() TaskPagination {
	return TaskPagination{
		Limit:  20,
		Offset: 0,
	}
}

// TaskListResult contains tasks and their total count.
type TaskListResult struct {
	Tasks []Task
	Total int
}

// HasChanges reports whether at least one field was explicitly changed.
func (u TaskUpdate) HasChanges() bool {
	return u.TitleSet ||
		u.DescriptionSet ||
		u.StatusSet ||
		u.AssigneeIDSet
}

