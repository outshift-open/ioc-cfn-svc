package app

import (
	"encoding/json"
	"math"
	"net/http"
	"runtime"
	"strings"
	"time"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

// diagnosticsInfoHandler returns git build info
func (a *App) diagnosticsInfoHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	info := map[string]any{
		"git": map[string]any{
			"commit": map[string]any{
				"time": a.gitCommitTime,
				"id":   a.gitCommitSHA,
			},
			"branch": a.gitBranch,
		},
	}
	return eh.RespondWithJSON(w, http.StatusOK, info)
}

// diagnosticsHealthHandler returns standard health response.
// Checks DB connectivity and required tables — if the DB volume was lost and
// recreated, this will fail and the orchestrator can restart the app container
// to re-run MigrateUp.
func (a *App) diagnosticsHealthHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	if err := a.db.HealthCheck(); err != nil {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "DOWN",
			"error":  err.Error(),
		})
	}
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

// diagnosticsMetricsHandler returns process-level runtime metrics
func (a *App) diagnosticsMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	round2 := func(v float64) float64 {
		return math.Round(v*100) / 100
	}

	metrics := map[string]any{
		"uptime_seconds":       round2(time.Since(a.startTime).Seconds()),
		"goroutines":           runtime.NumGoroutine(),
		"memory_heap_alloc_mb": round2(float64(mem.HeapAlloc) / 1024 / 1024),
		"memory_sys_mb":        round2(float64(mem.Sys) / 1024 / 1024),
		"gc_runs":              mem.NumGC,
	}
	return eh.RespondWithJSON(w, http.StatusOK, metrics)
}

// diagnosticsLoggersHandler returns current log levels for ROOT and all packages
func (a *App) diagnosticsLoggersHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, logger.GetAllLevels())
}

type setLoggerRequest struct {
	ModuleName string `json:"module-name"`
	LogLevel   string `json:"log-level"`
}

// diagnosticsSetLoggersHandler sets the log level dynamically
func (a *App) diagnosticsSetLoggersHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	var req setLoggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.LogLevel == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "log-level is required"})
	}

	level := strings.ToLower(req.LogLevel)

	// Validate log level
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
		"dpanic": true, "panic": true, "fatal": true,
	}
	if !validLevels[level] {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid log level: " + req.LogLevel + ". Valid levels: debug, info, warn, error, dpanic, panic, fatal",
		})
	}

	moduleName := strings.TrimSpace(req.ModuleName)
	if moduleName == "" {
		moduleName = "ROOT"
	}

	// Validate module name - must be ROOT or a registered package
	if moduleName != "ROOT" && !logger.IsRegisteredPackage(moduleName) {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "unknown module: " + moduleName + ". Use GET /api/internal/diagnostics/loggers to see available modules",
		})
	}

	if err := logger.SetPackageLevel(moduleName, level); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	log.Infof("log level changed to %s (module: %s)", level, moduleName)
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}
