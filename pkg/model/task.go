package model

import "time"

// Task represents a scheduled task bound to a specific MAS within a workspace.
// The composite unique index (workspace_id, mas_id, name) ensures one task definition per MAS.
// Status transitions: scheduled → running → scheduled (on success) or failed (on error/timeout).
type Task struct {
	ID               string     `gorm:"primaryKey;type:uuid"`
	Name             string     `gorm:"not null;uniqueIndex:idx_task_ws_mas_name"`
	Schedule         string     `gorm:"not null"`
	Enabled          bool       `gorm:"not null;default:true"`
	Status           string     `gorm:"not null;default:'scheduled'"`
	WorkspaceID      string     `gorm:"type:text;uniqueIndex:idx_task_ws_mas_name"`
	MASID            string     `gorm:"column:mas_id;type:text;uniqueIndex:idx_task_ws_mas_name"`
	NextRunTime      time.Time  `gorm:"not null"`
	LastRunTime      *time.Time
	LastStatus       *string
	CallbackDeadline *time.Time `gorm:"type:timestamptz"`
	UpdatedAt        time.Time  `gorm:"not null;autoUpdateTime"`
}

func (Task) TableName() string {
	return "tasks"
}
