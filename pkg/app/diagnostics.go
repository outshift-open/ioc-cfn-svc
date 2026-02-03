package app

import (
	"net/http"

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
