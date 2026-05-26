package model

import (
	"time"

	"gorm.io/datatypes"
)

type TaskExecutionHistory struct {
	ID         uint           `gorm:"primaryKey;autoIncrement"`
	TaskID     uint           `gorm:"not null;index"`
	Status     string         `gorm:"size:32;not null"`
	StartedAt  time.Time      `gorm:"not null"`
	FinishedAt *time.Time
	Result     *string        `gorm:"type:text"`
	Error      *string        `gorm:"type:text"`
	Metadata   datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"not null;autoCreateTime"`
}

func (TaskExecutionHistory) TableName() string {
	return "task_execution_history"
}
