package app

import (
	"encoding/json"
	"net/http"
	"strings"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

// diagnosticsHealthHandler returns TKF standard health response
func (a *App) diagnosticsHealthHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

// diagnosticsLoggersHandler returns current log level
func (a *App) diagnosticsLoggersHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{"log-level": logger.GetLevel()})
}

type setLoggerRequest struct {
	ModuleName string `json:"module-name"`
	LogLevel   string `json:"log-level"`
}

// diagnosticsSetLoggersHandler sets the log level dynamically
func (a *App) diagnosticsSetLoggersHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var req setLoggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.LogLevel == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "log-level is required"})
	}

	level := strings.ToLower(req.LogLevel)
	if err := logger.SetLevel(level); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid log level: " + req.LogLevel})
	}

	log.Infof("log level changed to %s (module: %s)", level, req.ModuleName)
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}
