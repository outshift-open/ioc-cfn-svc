package app

import (
	"time"

	"github.com/google/uuid"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/task"
)

// LongRunningBackgroundJob runs the DB-driven task scheduler loop.
// It polls every 30 seconds for due tasks, recovers timed-out executions, and dispatches work to CE.
func (a *App) LongRunningBackgroundJob() {
	log := getLogger()

	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Errorf("recovered from panic: [%s]", panicErr)
		}
	}()

	log.Info("task scheduler started")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			log.Infof("app stopped. ejecting from scheduler")
			return
		case <-ticker.C:
			a.runSchedulerTick()
		}
	}
}

// runSchedulerTick executes one iteration of the scheduler: first recovers any tasks
// whose callback deadline has expired, then finds and dispatches all due tasks.
func (a *App) runSchedulerTick() {
	log := getLogger()

	recovered, err := a.db.RecoverExpiredCallbacks()
	if err != nil {
		log.Warnf("error recovering expired callbacks: %s", err)
	} else if recovered > 0 {
		log.Infof("recovered %d expired task callbacks", recovered)
	}

	dueTasks, err := a.db.FindDueTasks()
	if err != nil {
		log.Warnf("error finding due tasks: %s", err)
		return
	}

	for i := range dueTasks {
		t := dueTasks[i]
		a.dispatchTask(t)
	}
}

// dispatchTask looks up the CE endpoint for a task, marks it as running with a 30-minute
// callback deadline, creates an execution history record, and sends the request to CE in a goroutine.
func (a *App) dispatchTask(t model.Task) {
	log := getLogger()

	endpointPath, ok := task.LookupEndpoint(t.Name)
	if !ok {
		log.Errorf("task %s has unknown task_name %q, skipping", t.ID, t.Name)
		_ = a.db.UpdateTaskStatus(t.ID, "failed", map[string]interface{}{
			"callback_deadline": nil,
		})
		return
	}

	now := time.Now()
	deadline := now.Add(time.Duration(a.Cfg.TaskCallbackDeadlineMinutes) * time.Minute)

	err := a.db.UpdateTaskStatus(t.ID, "running", map[string]interface{}{
		"callback_deadline": deadline,
	})
	if err != nil {
		log.Errorf("failed to mark task %s as running: %s", t.ID, err)
		return
	}

	hist := &model.TaskExecutionHistory{
		ID:          uuid.New().String(),
		TaskID:      t.ID,
		TaskName:    t.Name,
		WorkspaceID: &t.WorkspaceID,
		MasID:       &t.MASID,
		Status:      "running",
		StartedAt:   now,
	}
	if err := a.db.InsertTaskExecutionHistory(hist); err != nil {
		log.Errorf("failed to insert execution history for task %s: %s", t.ID, err)
		return
	}

	go a.sendTaskExecution(t, endpointPath, hist.ID)
}

// sendTaskExecution sends the task execution request to CE and handles dispatch failures
// by marking both the task and execution history as failed.
func (a *App) sendTaskExecution(t model.Task, endpointPath string, historyID string) {
	log := getLogger()

	callbackURL := a.Cfg.ExternalServiceURL + "/api/internal/tasks/callback"

	req := cognitionagentclient.TaskExecutionRequest{
		WorkspaceID: t.WorkspaceID,
		MASID:       t.MASID,
		TaskName:    t.Name,
		CallbackURL: callbackURL,
	}

	_, err := a.cognitionAgentsClient.SendTaskExecution(endpointPath, req)
	if err != nil {
		log.Errorf("failed to dispatch task %s to CE | workspace=%s mas=%s task_name=%s: %s", t.ID, t.WorkspaceID, t.MASID, t.Name, err)
		now := time.Now()
		errStr := err.Error()

		nextRun, cronErr := task.NextRunTime(t.Schedule, now)
		if cronErr != nil {
			nextRun = now.Add(time.Hour)
		}

		_ = a.db.UpdateTaskStatus(t.ID, "failed", map[string]interface{}{
			"callback_deadline": nil,
			"last_run_time":     now,
			"last_status":       "failed",
			"next_run_time":     nextRun,
		})
		_ = a.db.UpdateTaskExecutionHistory(historyID, map[string]interface{}{
			"status":      "failed",
			"finished_at": now,
			"error":       errStr,
		})
	}
}

// syncTasksFromConfig reconciles the tasks table with the latest config from the management plane.
// New tasks are created with next_run_time=now for immediate first execution; changed schedules
// recompute next_run_time; unknown task names are logged and skipped.
func (a *App) syncTasksFromConfig(cfg *CfnConfigPayload) {
	log := getLogger()

	// Skip sync if database is not initialized (e.g., in tests)
	if a.db == nil {
		log.Debug("skipping task sync: database not initialized")
		return
	}

	log.Info("syncing tasks from config")

	seenKeys := make(map[string]bool)

	for _, ws := range cfg.Workspaces {
		for _, mas := range ws.MultiAgenticSystems {
			if mas.TaskSchedule == nil {
				continue
			}
			ts := mas.TaskSchedule
			if !task.IsRegistered(ts.TaskName) {
				log.Warnf("workspace %s MAS %s: unknown task_name %q, skipping", ws.ID, mas.ID, ts.TaskName)
				continue
			}

			seenKeys[ws.ID+"|"+mas.ID] = true

			existing, err := a.db.FindTaskByKey(ws.ID, mas.ID, ts.TaskName)
			if err != nil {
				log.Errorf("error looking up task for ws=%s mas=%s name=%s: %s", ws.ID, mas.ID, ts.TaskName, err)
				continue
			}

			now := time.Now()
			if existing == nil {
				newTask := &model.Task{
					ID:          uuid.New().String(),
					WorkspaceID: ws.ID,
					MASID:       mas.ID,
					Name:        ts.TaskName,
					Schedule:    ts.Schedule,
					Enabled:     ts.Enabled,
					Status:      "scheduled",
					NextRunTime: now,
				}
				if err := a.db.UpsertTask(newTask); err != nil {
					log.Errorf("failed to create task for ws=%s mas=%s: %s", ws.ID, mas.ID, err)
				} else {
					log.Infof("task added | ws=%s mas=%s task_name=%s schedule=%s", ws.ID, mas.ID, ts.TaskName, ts.Schedule)
				}
			} else {
				scheduleChanged := existing.Schedule != ts.Schedule
				enabledChanged := existing.Enabled != ts.Enabled

				if scheduleChanged || enabledChanged {
					existing.Schedule = ts.Schedule
					existing.Enabled = ts.Enabled

					if scheduleChanged {
						nextRun, err := task.NextRunTime(ts.Schedule, now)
						if err != nil {
							log.Errorf("invalid cron expression %q for ws=%s mas=%s: %s", ts.Schedule, ws.ID, mas.ID, err)
							continue
						}
						existing.NextRunTime = nextRun
					}

					if err := a.db.UpsertTask(existing); err != nil {
						log.Errorf("failed to update task for ws=%s mas=%s: %s", ws.ID, mas.ID, err)
					}
				}
			}
		}
	}

	// Disable tasks whose MAS no longer exists in config (e.g. MAS was deleted).
	// Soft-disables by setting enabled=false — preserves execution history for audit.
	disabled, err := a.db.DisableTasksNotInSet(seenKeys)
	if err != nil {
		log.Warnf("error disabling orphaned tasks: %s", err)
	}
	for _, dt := range disabled {
		log.Infof("task removed | ws=%s mas=%s task_name=%s", dt.WorkspaceID, dt.MASID, dt.Name)
	}
}
