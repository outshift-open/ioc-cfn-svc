// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/prometheus"

	"github.com/outshift-open/ioc-cfn-svc/pkg/audit"
	"github.com/outshift-open/ioc-cfn-svc/pkg/config"
	"github.com/outshift-open/ioc-cfn-svc/pkg/metric"
	"github.com/outshift-open/ioc-cfn-svc/pkg/model"
	"github.com/outshift-open/ioc-cfn-svc/pkg/otelreceiver"
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

	if err := db.DB.AutoMigrate(&model.Task{}, &model.TaskExecutionHistory{}, &model.OtelTraceIngestionState{}); err != nil {
		return err
	}

	// Partial index for readiness scans that wait after the last received span batch.
	// Serves MarkInactiveTracesReady: WHERE status='pending' AND last_span_time IS NOT NULL AND updated_at < cutoff
	db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_otel_ingestion_pending_updated_at
		ON otel_trace_ingestion_state (status, updated_at)
		WHERE status = 'pending' AND last_span_time IS NOT NULL`)

	// Partial index for efficient task payload claims (ready traces that passed the delay).
	db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_otel_ingestion_ready_timeout
		ON otel_trace_ingestion_state (workspace_id, mas_id, status, last_span_time, created_at)
		WHERE status = 'ready' AND last_span_time IS NOT NULL`)

	// Partial index for efficient due-task polling — covers both statuses the scheduler queries.
	db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_task_due
		ON task (next_run_time)
		WHERE status IN ('scheduled', 'failed')`)

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

// UpsertPendingOtelTrace inserts or updates trace activity while spans are still arriving.
// Late spans move a ready or running trace back to pending so ingestion waits for a fresh inactivity window.
func (db *Database) UpsertPendingOtelTrace(workspaceID, masID, traceID string, lastSpanTime time.Time) error {
	state := &model.OtelTraceIngestionState{
		ID:           uuid.New().String(),
		WorkspaceID:  workspaceID,
		MasID:        masID,
		TraceID:      traceID,
		Status:       "pending",
		LastSpanTime: &lastSpanTime,
	}
	result := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "mas_id"}, {Name: "trace_id"}},
		DoNothing: true,
	}).Create(state)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}

	return db.DB.Model(&model.OtelTraceIngestionState{}).
		Where("workspace_id = ? AND mas_id = ? AND trace_id = ? AND status IN ?", workspaceID, masID, traceID, []string{"pending", "ready", "running"}).
		Updates(map[string]interface{}{
			"last_span_time": gorm.Expr("GREATEST(COALESCE(last_span_time, ?), ?)", lastSpanTime, lastSpanTime),
			"status":         gorm.Expr("CASE WHEN status IN ? THEN ? ELSE status END", []string{"ready", "running"}, "pending"),
			"updated_at":     time.Now(),
		}).Error
}

// ClaimReadyOtelTraces atomically selects ready traces that have passed the inactivity delay
// and moves them to running so a task trigger can push their spans to CE without duplicate dispatch.
func (db *Database) ClaimReadyOtelTraces(workspaceID, masID string, limit int, inactivityThreshold time.Duration) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	var rows []model.OtelTraceIngestionState
	cutoff := time.Now().Add(-inactivityThreshold)

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("workspace_id = ? AND mas_id = ? AND status = ? AND last_span_time IS NOT NULL AND last_span_time < ?", workspaceID, masID, "ready", cutoff).
			Order("created_at ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}

		if len(rows) == 0 {
			return nil
		}

		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}

		return tx.Model(&model.OtelTraceIngestionState{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":     "running",
				"updated_at": time.Now(),
			}).Error
	})
	if err != nil {
		return nil, err
	}

	traceIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		traceIDs = append(traceIDs, r.TraceID)
	}
	return traceIDs, nil
}

// UpdateOtelTraceStatus transitions a trace's ingestion state.
func (db *Database) UpdateOtelTraceStatus(workspaceID, masID, traceID, newStatus string) error {
	query := db.DB.Model(&model.OtelTraceIngestionState{}).
		Where("workspace_id = ? AND mas_id = ? AND trace_id = ?", workspaceID, masID, traceID).
		Where("status = ?", "running")
	return query.Updates(map[string]interface{}{"status": newStatus, "updated_at": time.Now()}).Error
}

// MarkInactiveTracesReady transitions traces from "pending" to "ready" if no span batch has updated
// the trace for longer than the inactivity threshold. The always-polling extraction task will pick
// them up on its next scheduler tick without explicit wake-up.
// Returns count of traces marked ready.
func (db *Database) MarkInactiveTracesReady(inactivityThreshold time.Duration) (int, error) {
	now := time.Now()
	cutoff := now.Add(-inactivityThreshold)

	result := db.DB.Exec(`
		UPDATE otel_trace_ingestion_state
		SET status = 'ready', updated_at = ?
		WHERE status = 'pending'
		  AND last_span_time IS NOT NULL
		  AND updated_at < ?
	`, now, cutoff)
	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

// GetOtelSpansForTrace retrieves all spans for a given trace scoped by workspace and MAS.
func (db *Database) GetOtelSpansForTrace(workspaceID, masID, traceID string) ([]otelreceiver.OtelSpan, error) {
	var spans []otelreceiver.OtelSpan
	err := db.Where("workspace_id = ? AND mas_id = ? AND trace_id = ?", workspaceID, masID, traceID).
		Order("start_time ASC").Find(&spans).Error
	return spans, err
}
