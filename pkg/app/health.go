package app

import (
	"fmt"
	"net/http"
	"time"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) (
	int, error) {

	status := http.StatusOK
	resp := map[string]interface{}{
		"ready":         fmt.Sprintf("%t", a.readyForRequests.Load()),
		"build_version": a.buildVersion,
		"tag_version":   a.Cfg.TagVersion,
		"host_id":       a.Cfg.HostID,
		"chart_version": a.Cfg.HelmChartVersion,

		// fields required by service-health-monitor service
		"service_name":  a.Cfg.ServiceName,
		"service_state": "Up",
		"last_updated":  time.Now().UTC().Format(time.RFC3339),
	}

	//nolint:goconst // allow "true" string
	allCheck := r.URL.Query().Get("full_diagnostic") == "true"
	dbCheck := allCheck || r.URL.Query().Get("db_check") == "true"

	if dbCheck {
		err := a.db.Ping()
		resp["db_healthy"] = err == nil
		if err != nil {
			resp["db_err"] = fmt.Sprintf("%s", err)
			status = http.StatusServiceUnavailable // critical failure
		}
	}

	if status != http.StatusOK {
		resp["service_state"] = "Down"
	}

	resp["status"] = http.StatusText(status)
	return eh.RespondWithJSON(w, status, resp)
}

func (a *App) readyHandler(w http.ResponseWriter, r *http.Request) (
	int, error) {

	isReady := a.readyForRequests.Load()
	status := http.StatusTooEarly
	if isReady {
		status = http.StatusOK
	}
	return eh.RespondWithJSON(w, status, map[string]bool{"ready": isReady})
}
