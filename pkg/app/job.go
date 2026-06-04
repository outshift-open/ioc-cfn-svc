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

// dispatchTask looks up the CE definition, derives the endpoint from its kind/subkind,
// marks the task as running, creates an execution history record, and dispatches to CE.
func (a *App) dispatchTask(t model.Task) {
	log := getLogger()

	cfnConfigMutex.RLock()
	ceConfig := ParsedConfig.FindCE(t.CEID)
	cfnConfigMutex.RUnlock()

	if ceConfig == nil {
		log.Errorf("task %s: CE %s not found in config, skipping", t.ID, t.CEID)
		_ = a.db.UpdateTaskStatus(t.ID, "failed", map[string]interface{}{
			"callback_deadline": nil,
		})
		return
	}

	endpointPath := task.GetEndpointForCE(ceConfig.Kind, ceConfig.Subkind)
	if endpointPath == "" {
		log.Errorf("task %s: no endpoint mapping for CE kind=%s subkind=%s, skipping", t.ID, ceConfig.Kind, ceConfig.Subkind)
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
		CeID:        &t.CEID,
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
		CEID:        t.CEID,
		CallbackURL: callbackURL,
	}

	_, err := a.cognitionAgentsClient.SendTaskExecution(endpointPath, req)
	if err != nil {
		log.Errorf("failed to dispatch task %s to CE %s | workspace=%s mas=%s: %s", t.ID, t.CEID, t.WorkspaceID, t.MASID, err)
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
			// Sync CE-scoped tasks (extract schedule from CE's MASConfig)
			for _, ce := range mas.CognitionEngines {
				if ce.MASConfig == nil {
					continue
				}
				// Extract schedule from MASConfig["schedule"]
				// Expected format: "0 */12 * * *" (cron expression string)
				scheduleVal, hasSchedule := ce.MASConfig["schedule"]
				if !hasSchedule {
					// No schedule configured for this CE in this MAS - skip
					continue
				}

				cronExpr, ok := scheduleVal.(string)
				if !ok || cronExpr == "" {
					log.Warnf("workspace %s MAS %s CE %s: MASConfig.schedule is not a string or is empty, skipping", ws.ID, mas.ID, ce.ID)
					continue
				}

				// Look up the CE definition to get its name and verify kind/subkind has an endpoint
				ceConfig := cfg.FindCE(ce.ID)
				if ceConfig == nil {
					log.Warnf("workspace %s MAS %s: CE %s not found in top-level cognition_engines, skipping", ws.ID, mas.ID, ce.ID)
					continue
				}

				// Verify that this CE type has an endpoint mapping
				endpointPath := task.GetEndpointForCE(ceConfig.Kind, ceConfig.Subkind)
				if endpointPath == "" {
					log.Warnf("workspace %s MAS %s CE %s: no endpoint mapping for kind=%s subkind=%s, skipping", ws.ID, mas.ID, ce.ID, ceConfig.Kind, ceConfig.Subkind)
					continue
				}

				// Use CE name as task name for better logging
				a.syncSingleTask(ws.ID, mas.ID, ce.ID, ceConfig.Name, cronExpr, seenKeys)
			}
		}
	}

	// Delete tasks whose CE schedule was removed from config.
	// Execution history is preserved in task_execution_history for audit.
	deleted, err := a.db.DeleteTasksNotInSet(seenKeys)
	if err != nil {
		log.Warnf("error deleting orphaned tasks: %s", err)
	}
	for _, dt := range deleted {
		log.Infof("task deleted | ws=%s mas=%s ce=%s name=%s", dt.WorkspaceID, dt.MASID, dt.CEID, dt.Name)
	}
}

// syncSingleTask handles the upsert logic for a single CE task.
func (a *App) syncSingleTask(workspaceID, masID, ceID, ceName, cronExpr string, seenKeys map[string]bool) {
	log := getLogger()

	seenKeys[workspaceID+"|"+masID+"|"+ceID] = true

	existing, err := a.db.FindTaskByKey(workspaceID, masID, ceID)
	if err != nil {
		log.Errorf("error looking up task for ws=%s mas=%s ce=%s: %s", workspaceID, masID, ceID, err)
		return
	}

	now := time.Now()

	if existing == nil {
		newTask := &model.Task{
			ID:          uuid.New().String(),
			WorkspaceID: workspaceID,
			MASID:       masID,
			CEID:        ceID,
			Name:        ceName,
			Schedule:    cronExpr,
			Status:      "scheduled",
			NextRunTime: now,
		}
		if err := a.db.UpsertTask(newTask); err != nil {
			log.Errorf("failed to create task for ws=%s mas=%s ce=%s: %s", workspaceID, masID, ceID, err)
		} else {
			log.Infof("task added | ws=%s mas=%s ce=%s name=%s schedule=%s", workspaceID, masID, ceID, ceName, cronExpr)
		}
	} else {
		// Update task if name or schedule changed
		nameChanged := existing.Name != ceName
		scheduleChanged := existing.Schedule != cronExpr

		if nameChanged || scheduleChanged {
			existing.Name = ceName
			existing.Schedule = cronExpr

			if scheduleChanged {
				nextRun, err := task.NextRunTime(cronExpr, now)
				if err != nil {
					log.Errorf("invalid cron expression %q for ws=%s mas=%s ce=%s: %s", cronExpr, workspaceID, masID, ceID, err)
					return
				}
				existing.NextRunTime = nextRun
			}

			if err := a.db.UpsertTask(existing); err != nil {
				log.Errorf("failed to update task for ws=%s mas=%s ce=%s: %s", workspaceID, masID, ceID, err)
			} else if scheduleChanged {
				log.Infof("task updated | ws=%s mas=%s ce=%s schedule changed to %s", workspaceID, masID, ceID, cronExpr)
			}
		}
	}
}
