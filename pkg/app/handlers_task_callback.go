package app

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/task"
)

// taskCallbackRequest is the payload CE sends when a task execution completes or fails.
type taskCallbackRequest struct {
	WorkspaceID string `json:"workspace_id"`
	MASID       string `json:"mas_id"`
	TaskName    string `json:"task_name"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	Result      string `json:"result,omitempty"`
}

// handleTaskCallback processes CE completion callbacks for async task executions.
// On success it resets the task to scheduled with a new next_run_time; on failure it marks the task as failed.
// Late callbacks for already-finalized tasks are ignored.
func (a *App) handleTaskCallback(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	var req taskCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	if req.WorkspaceID == "" || req.MASID == "" || req.TaskName == "" || req.Status == "" {
		http.Error(w, "workspace_id, mas_id, task_name, and status are required", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	t, err := a.db.FindTaskByKey(req.WorkspaceID, req.MASID, req.TaskName)
	if err != nil {
		log.Errorf("callback: error finding task ws=%s mas=%s name=%s: %s", req.WorkspaceID, req.MASID, req.TaskName, err)
		return http.StatusInternalServerError, err
	}
	if t == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return http.StatusNotFound, nil
	}

	if t.Status != "running" {
		log.Warnf("callback: task %s is not in running state (status=%s), ignoring late callback", t.ID, t.Status)
		w.WriteHeader(http.StatusOK)
		return http.StatusOK, nil
	}

	now := time.Now()

	historyFields := map[string]interface{}{
		"status":      req.Status,
		"finished_at": now,
	}
	if req.Error != "" {
		historyFields["error"] = req.Error
	}
	if req.Result != "" {
		historyFields["result"] = req.Result
	}

	if err := a.db.UpdateLatestExecutionHistoryByTaskID(t.ID, historyFields); err != nil {
		log.Errorf("callback: failed to update execution history for task %s: %s", t.ID, err)
	}

	taskFields := map[string]interface{}{
		"callback_deadline": nil,
		"last_run_time":     now,
		"last_status":       req.Status,
	}

	if req.Status == "success" {
		nextRun, err := task.NextRunTime(t.Schedule, now)
		if err != nil {
			log.Errorf("callback: failed to compute next run time for task %s: %s", t.ID, err)
			_ = a.db.UpdateTaskStatus(t.ID, "failed", taskFields)
			w.WriteHeader(http.StatusOK)
			return http.StatusOK, nil
		}
		taskFields["next_run_time"] = nextRun
		_ = a.db.UpdateTaskStatus(t.ID, "scheduled", taskFields)
	} else {
		_ = a.db.UpdateTaskStatus(t.ID, "failed", taskFields)
	}

	w.WriteHeader(http.StatusOK)
	return http.StatusOK, nil
}
