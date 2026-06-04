package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskExecutionHistory records each individual execution attempt of a scheduled task.
// A new row is inserted when the scheduler dispatches a task to CE, and updated when
// the callback arrives or the execution times out.
// WorkspaceID, MasID, CeID, and TaskName are denormalized from the Task row for query convenience.
type TaskExecutionHistory struct {
	ID          string         `gorm:"primaryKey;type:uuid"`
	TaskID      string         `gorm:"not null"`
	TaskName    string         `gorm:"not null"`
	WorkspaceID *string        `gorm:"type:text"`
	MasID       *string        `gorm:"column:mas_id;type:text"`
	CeID        *string        `gorm:"column:ce_id;type:text"` // Optional: set for CE-scoped task executions
	Status      string         `gorm:"not null"`
	Metadata    datatypes.JSON `gorm:"type:jsonb"`
	StartedAt   time.Time      `gorm:"not null"`
	FinishedAt  *time.Time     `gorm:"type:timestamptz"`
	Result      *string        `gorm:"type:text"`
	Error       *string        `gorm:"type:text"`
	CreatedAt   time.Time      `gorm:"not null;autoCreateTime"`
}

func (TaskExecutionHistory) TableName() string {
	return "task_execution_history"
}
