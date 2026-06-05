package database

import (
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/prometheus"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/metric"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
)

// Database wraps a GORM DB connection to PostgreSQL.
type Database struct {
	*gorm.DB
}

// New opens a PostgreSQL connection using the provided config and registers Prometheus metrics.
func New(cfg config.Database) (*Database, error) {
	dsn := cfg.DSN()

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, errors.New(err)
	}

	err = gdb.Use(prometheus.New(prometheus.Config{
		DBName:          cfg.Name,
		RefreshInterval: 15,
		StartServer:     false,
	}))
	if err != nil {
		return nil, errors.New(err)
	}

	// TODO: Configure connection pool settings (MaxOpenConns, MaxIdleConns, ConnMaxLifetime, ConnMaxIdleTime) to avoid default GORM/database/sql pool defaults if needed.
	return &Database{DB: gdb}, nil
}

// Close closes the underlying database connection.
func (db *Database) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping verifies the database connection is alive.
func (db *Database) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// MigrateUp runs all auto-migrations (audit tables, metrics tables, otel_spans, etc.).
func (db *Database) MigrateUp() error {
	if err := audit.MigrateUp(db.DB); err != nil {
		return err
	}

	// Migrate MAS metrics table
	if err := metric.MigrateUp(db.DB); err != nil {
		return err
	}

	// Migrate CE metrics table
	if err := metric.MigrateCEMetricsUp(db.DB); err != nil {
		return err
	}

	if err := otelreceiver.MigrateUp(db.DB); err != nil {
		return err
	}

	if err := db.DB.AutoMigrate(&model.Task{}, &model.TaskExecutionHistory{}); err != nil {
		return err
	}

	// Partial index for efficient due-task polling — only indexes rows the scheduler queries.
	db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_task_next_run_time
		ON task (next_run_time)
		WHERE status = 'scheduled'`)

	// Composite index for looking up execution history by task ordered by recency.
	db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_task_execution_history_task_id
		ON task_execution_history (task_id, started_at DESC)`)

	return nil
}

// BulkInsertOtelSpans inserts a batch of OtelSpans in a single DB call.
func (db *Database) BulkInsertOtelSpans(spans []otelreceiver.OtelSpan) error {
	return db.Create(&spans).Error
}

// Find_User_By_IDPUserID_And_Issuer looks up a user by IDP user ID and issuer.
func (db *Database) Find_User_By_IDPUserID_And_Issuer(idpUserID string,
	idpIssuer string) (*model.UserType, error) {

	var user model.UserType
	q := db.DB.
		Where(&model.UserType{
			IDPUserID: idpUserID,
			IDPIssuer: idpIssuer,
		}).Find(&user)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil
	}

	return &user, nil
}

// Create_User inserts a new user record.
func (db *Database) Create_User(u *model.UserType) error {
	q := db.DB.Create(u)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

// Create_Session inserts a new session record.
func (db *Database) Create_Session(s *model.SessionType) error {
	q := db.DB.Create(s)
	if q.Error != nil {
		return q.Error
	}
	return nil
}

// CreateAuditEvent inserts a new audit event.
func (db *Database) CreateAuditEvent(a *audit.Audit) error {
	return audit.CreateAuditEvent(db.DB, a)
}

// GetAuditEventByID retrieves a single audit event by UUID.
func (db *Database) GetAuditEventByID(id uuid.UUID) (*audit.Audit, error) {
	return audit.GetAuditEventByID(db.DB, id)
}

// ListAuditEvents returns audit events with optional resource_type and audit_type filters.
func (db *Database) ListAuditEvents(resourceType, auditType string, page, pageSize int) (*audit.AuditListResponse, error) {
	return audit.ListAuditEvents(db.DB, resourceType, auditType, page, pageSize)
}

// DeleteAuditEventByID deletes a single audit event by UUID.
func (db *Database) DeleteAuditEventByID(id uuid.UUID) error {
	return audit.DeleteAuditEventByID(db.DB, id)
}

// FindDueTasks returns tasks in 'scheduled' or 'failed' status whose next_run_time has passed.
func (db *Database) FindDueTasks() ([]model.Task, error) {
	var tasks []model.Task
	err := db.DB.
		Where("status IN ? AND next_run_time <= ?", []string{"scheduled", "failed"}, time.Now()).
		Find(&tasks).Error
	return tasks, err
}

// UpsertTask creates or updates a task based on the unique (workspace_id, mas_id, ce_id) key.
func (db *Database) UpsertTask(task *model.Task) error {
	return db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "mas_id"}, {Name: "ce_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "schedule", "next_run_time", "updated_at"}),
	}).Create(task).Error
}

// UpdateTaskStatus updates a task's status and any extra fields atomically.
func (db *Database) UpdateTaskStatus(taskID string, status string, fields map[string]interface{}) error {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["status"] = status
	return db.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(fields).Error
}

// RecoverExpiredCallbacks marks tasks whose callback deadline has passed as failed,
// and updates their corresponding execution history records.
func (db *Database) RecoverExpiredCallbacks() (int64, error) {
	now := time.Now()

	var expiredTasks []model.Task
	if err := db.DB.Where("status = ? AND callback_deadline < ?", "running", now).Find(&expiredTasks).Error; err != nil {
		return 0, err
	}
	if len(expiredTasks) == 0 {
		return 0, nil
	}

	taskIDs := make([]string, len(expiredTasks))
	for i, t := range expiredTasks {
		taskIDs[i] = t.ID
	}

	db.DB.Model(&model.TaskExecutionHistory{}).
		Where("task_id IN ? AND status = ?", taskIDs, "running").
		Updates(map[string]interface{}{"status": "timeout", "finished_at": now})

	result := db.DB.Model(&model.Task{}).
		Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{
			"status":            "failed",
			"callback_deadline": nil,
			"last_run_time":     now,
			"last_status":       "timeout",
		})

	return result.RowsAffected, result.Error
}

// InsertTaskExecutionHistory creates a new execution history record.
func (db *Database) InsertTaskExecutionHistory(h *model.TaskExecutionHistory) error {
	return db.DB.Create(h).Error
}

// UpdateTaskExecutionHistory updates an execution history record by ID.
func (db *Database) UpdateTaskExecutionHistory(id string, fields map[string]interface{}) error {
	return db.DB.Model(&model.TaskExecutionHistory{}).Where("id = ?", id).Updates(fields).Error
}

// UpdateLatestExecutionHistoryByTaskID updates the most recent execution history for a task.
func (db *Database) UpdateLatestExecutionHistoryByTaskID(taskID string, fields map[string]interface{}) error {
	var hist model.TaskExecutionHistory
	err := db.DB.Where("task_id = ?", taskID).Order("started_at DESC").First(&hist).Error
	if err != nil {
		return err
	}
	return db.DB.Model(&hist).Updates(fields).Error
}

// DeleteTasksNotInSet hard-deletes tasks whose (workspace_id, mas_id, ce_id) is not in activeKeys.
// Called during config sync to clean up tasks for deleted CE schedule entries.
// activeKeys format: "workspace_id|mas_id|ce_id"
// Only deletes tasks in 'scheduled' or 'failed' status to avoid breaking in-flight executions.
// Tasks in 'running' status are skipped and will be cleaned up on their next status transition.
// Execution history is preserved in task_execution_history table for audit.
func (db *Database) DeleteTasksNotInSet(activeKeys map[string]bool) ([]model.Task, error) {
	var allTasks []model.Task
	if err := db.DB.Where("status IN ?", []string{"scheduled", "failed"}).Find(&allTasks).Error; err != nil {
		return nil, err
	}
	var deleted []model.Task
	for _, t := range allTasks {
		key := t.WorkspaceID + "|" + t.MASID + "|" + t.CEID
		if !activeKeys[key] {
			if err := db.DB.Delete(&t).Error; err == nil {
				deleted = append(deleted, t)
			}
		}
	}
	return deleted, nil
}

// FindTaskByKey looks up a task by its unique key (workspace_id, mas_id, ce_id).
func (db *Database) FindTaskByKey(workspaceID, masID, ceID string) (*model.Task, error) {
	var task model.Task
	err := db.DB.
		Where("workspace_id = ? AND mas_id = ? AND ce_id = ?", workspaceID, masID, ceID).
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}
