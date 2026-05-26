package model

import "time"

type Task struct {
	ID               uint       `gorm:"primaryKey;autoIncrement"`
	WorkspaceID      string     `gorm:"size:36;not null;uniqueIndex:idx_task_ws_mas_name"`
	MASID            string     `gorm:"column:mas_id;size:36;not null;uniqueIndex:idx_task_ws_mas_name"`
	Name             string     `gorm:"size:128;not null;uniqueIndex:idx_task_ws_mas_name"`
	Schedule         string     `gorm:"size:64;not null"`
	Enabled          bool       `gorm:"not null;default:true"`
	Status           string     `gorm:"size:32;not null;default:'scheduled'"`
	NextRunTime      *time.Time `gorm:"index"`
	CallbackDeadline *time.Time
	CreatedAt        time.Time  `gorm:"not null;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"not null;autoUpdateTime"`
}

func (Task) TableName() string {
	return "tasks"
}
