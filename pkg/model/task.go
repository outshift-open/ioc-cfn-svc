package model

import (
	"time"

	"gorm.io/datatypes"
)

// Task represents a scheduled task for a specific CE within a MAS.
// Each CE with a schedule gets exactly one task row.
// The composite unique index (workspace_id, mas_id, ce_id) ensures one task per CE per MAS.
// Name is descriptive only (typically the CE name) - not part of the unique key.
// Status transitions: scheduled → running → scheduled (on success) or failed (on error/timeout).
//
// Schedule may be nil for tasks whose NextRunTime is managed externally (e.g. event-driven triggers).
// When Schedule is set, NextRunTime is auto-computed from the cron expression after each run.
type Task struct {
	ID               string     `gorm:"primaryKey;type:uuid"`
	Name             string     `gorm:"not null"`                                                // CE name (descriptive only)
	Schedule         *string    `gorm:"type:text"`                                               // Cron expression (nil = externally scheduled)
	Status           string     `gorm:"not null;default:'scheduled'"`                            // scheduled, running, or failed
	WorkspaceID      string     `gorm:"type:text;uniqueIndex:idx_task_ws_mas_ce"`               // Workspace this task belongs to
	MASID            string     `gorm:"column:mas_id;type:text;uniqueIndex:idx_task_ws_mas_ce"` // MAS this task belongs to
	CEID             string     `gorm:"column:ce_id;type:text;uniqueIndex:idx_task_ws_mas_ce"`  // CE this task executes
	NextRunTime      time.Time  `gorm:"not null"`                                                // Next scheduled execution (set by cron or externally)
	LastRunTime      *time.Time                                                                  // Most recent execution start
	LastStatus       *string                                                                     // Most recent execution result
	InputConfig      datatypes.JSON `gorm:"type:jsonb"`                                              // Optional developer-defined input (endpoint, payload, metadata)
	CallbackDeadline *time.Time     `gorm:"type:timestamptz"`                                        // Timeout deadline for current execution
	UpdatedAt        time.Time      `gorm:"not null;autoUpdateTime"`                                 // Last modification time
}

func (Task) TableName() string {
	return "tasks"
}
