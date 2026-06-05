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
	CEID        string `json:"ce_id"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	Result      string `json:"result,omitempty"`
}

// handleTaskCallback processes CE completion callbacks for async task executions.
// On success it resets the task to scheduled (recomputing next_run_time from cron if schedule is set);
// on failure it marks the task as failed. Late callbacks for already-finalized tasks are ignored.
// For externally scheduled tasks (schedule=nil), next_run_time is left unchanged on success —
// developers can override it via their own function or API for their use case.
func (a *App) handleTaskCallback(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	var req taskCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	if req.WorkspaceID == "" || req.MASID == "" || req.CEID == "" || req.Status == "" {
		http.Error(w, "workspace_id, mas_id, ce_id, and status are required", http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}

	t, err := a.db.FindTaskByKey(req.WorkspaceID, req.MASID, req.CEID)
	if err != nil {
		log.Errorf("callback: error finding task ws=%s mas=%s ce=%s: %s", req.WorkspaceID, req.MASID, req.CEID, err)
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
		if t.Schedule != nil {
			nextRun, err := task.NextRunTime(*t.Schedule, now)
			if err != nil {
				log.Errorf("callback: failed to compute next run time for task %s: %s", t.ID, err)
				_ = a.db.UpdateTaskStatus(t.ID, "failed", taskFields)
				w.WriteHeader(http.StatusOK)
				return http.StatusOK, nil
			}
			taskFields["next_run_time"] = nextRun
		}
		_ = a.db.UpdateTaskStatus(t.ID, "scheduled", taskFields)
	} else {
		_ = a.db.UpdateTaskStatus(t.ID, "failed", taskFields)
	}

	w.WriteHeader(http.StatusOK)
	return http.StatusOK, nil
}
