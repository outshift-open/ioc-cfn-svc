package app

import (
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/task"
)

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

func (a *App) dispatchTask(t model.Task) {
	log := getLogger()

	endpointPath, ok := task.LookupEndpoint(t.Name)
	if !ok {
		log.Errorf("task %d has unknown task_name %q, skipping", t.ID, t.Name)
		_ = a.db.UpdateTaskStatus(t.ID, "failed", map[string]interface{}{
			"callback_deadline": nil,
		})
		return
	}

	now := time.Now()
	deadline := now.Add(30 * time.Minute)

	err := a.db.UpdateTaskStatus(t.ID, "running", map[string]interface{}{
		"callback_deadline": deadline,
	})
	if err != nil {
		log.Errorf("failed to mark task %d as running: %s", t.ID, err)
		return
	}

	hist := &model.TaskExecutionHistory{
		TaskID:    t.ID,
		Status:    "running",
		StartedAt: now,
	}
	if err := a.db.InsertTaskExecutionHistory(hist); err != nil {
		log.Errorf("failed to insert execution history for task %d: %s", t.ID, err)
		return
	}

	go a.sendTaskExecution(t, endpointPath, hist.ID)
}

func (a *App) sendTaskExecution(t model.Task, endpointPath string, historyID uint) {
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
		log.Errorf("failed to dispatch task %d to CE: %s", t.ID, err)
		now := time.Now()
		errStr := err.Error()
		_ = a.db.UpdateTaskStatus(t.ID, "failed", map[string]interface{}{
			"callback_deadline": nil,
		})
		_ = a.db.UpdateTaskExecutionHistory(historyID, map[string]interface{}{
			"status":      "failed",
			"finished_at": now,
			"error":       errStr,
		})
	}
}

func (a *App) syncTasksFromConfig(cfg *CfnConfigPayload) {
	log := getLogger()

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

			existing, err := a.db.FindTaskByKey(ws.ID, mas.ID, ts.TaskName)
			if err != nil {
				log.Errorf("error looking up task for ws=%s mas=%s name=%s: %s", ws.ID, mas.ID, ts.TaskName, err)
				continue
			}

			now := time.Now()
			if existing == nil {
				nextRun, err := task.NextRunTime(ts.Schedule, now)
				if err != nil {
					log.Errorf("invalid cron expression %q for ws=%s mas=%s: %s", ts.Schedule, ws.ID, mas.ID, err)
					continue
				}
				newTask := &model.Task{
					WorkspaceID: ws.ID,
					MASID:       mas.ID,
					Name:        ts.TaskName,
					Schedule:    ts.Schedule,
					Enabled:     ts.Enabled,
					Status:      "scheduled",
					NextRunTime: &nextRun,
				}
				if err := a.db.UpsertTask(newTask); err != nil {
					log.Errorf("failed to create task for ws=%s mas=%s: %s", ws.ID, mas.ID, err)
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
						existing.NextRunTime = &nextRun
					}

					if err := a.db.UpsertTask(existing); err != nil {
						log.Errorf("failed to update task for ws=%s mas=%s: %s", ws.ID, mas.ID, err)
					}
				}
			}
		}
	}
}
