// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/outshift-open/ioc-cfn-svc/pkg/tools/logger"
)

// diagnosticsInfoHandler returns service and git build info
func (a *App) diagnosticsInfoHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	info := map[string]any{
		"service":     getEnvOrDefault("SERVICE_NAME", "ioc-cfn-svc"),
		"version":     a.buildVersion,
		"go_version":  runtime.Version(),
		"platform":    runtime.GOOS + "/" + runtime.GOARCH,
		"environment": getEnvOrDefault("ENV", "development"),
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
// When ?dependencies=true is passed, it probes downstream services grouped by type,
// populated from ParsedConfig. The basic check (no query param) is used by Docker health checks.
func (a *App) diagnosticsHealthHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	if r.URL.Query().Get("dependencies") != "true" {
		// Basic health check - verify both HTTP and MCP servers are running
		mcpPort := fmt.Sprintf("%d", a.Cfg.McpPort)
		mcpHealthy := a.probeTCPPort("localhost:" + mcpPort)

		if !mcpHealthy {
			return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "DOWN",
				"message": "MCP server not responding on port " + mcpPort,
			})
		}

		return eh.RespondWithJSON(w, http.StatusOK, map[string]any{
			"status": "UP",
			"servers": map[string]bool{
				"http": true,
				"mcp":  mcpHealthy,
			},
		})
	}

	// probe checks reachability via TCP dial — works for any service regardless
	// of what health endpoint it exposes (including external providers like Mem0).
	probe := func(rawURL string) bool {
		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			rawURL = "http://" + rawURL
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return false
		}
		host := u.Host
		if u.Port() == "" {
			if u.Scheme == "https" {
				host += ":443"
			} else {
				host += ":80"
			}
		}
		conn, err := net.DialTimeout("tcp", host, 3*time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	cfnConfigMutex.RLock()
	cfg := ParsedConfig
	cfnConfigMutex.RUnlock()

	status := "UP"

	mgmtHealthy := probe(getEnvOrDefault("MGMT_URL", "http://localhost:9000"))
	if !mgmtHealthy {
		status = "DOWN"
	}

	memoryProviders := map[string]bool{}
	if cfg != nil {
		for _, p := range cfg.MemoryProviders {
			if p.Name == "" {
				continue
			}
			var providerURL string
			if p.Config != nil {
				providerURL = p.Config.URL
			}
			healthy := providerURL != "" && probe(providerURL)
			memoryProviders[p.Name] = healthy
			if !healthy && status != "DOWN" {
				status = "DOWN"
			}
		}
	}

	cognitionEngines := map[string]bool{}
	if cfg != nil {
		// CEs are now at the top level, not per workspace
		for _, engine := range cfg.CognitionEngines {
			if engine.Name == "" {
				continue
			}
			engineURL := engine.URL
			healthy := engineURL != "" && probe(engineURL)
			cognitionEngines[engine.Name] = healthy
			if !healthy && status == "UP" {
				status = "DEGRADED"
			}
		}
	}

	httpStatus := http.StatusOK
	if status == "DOWN" {
		httpStatus = http.StatusInternalServerError
	}

	// Add MCP server check to detailed health response
	mcpPort := fmt.Sprintf("%d", a.Cfg.McpPort)
	mcpHealthy := a.probeTCPPort("localhost:" + mcpPort)
	if !mcpHealthy && status != "DOWN" {
		status = "DEGRADED"
	}

	return eh.RespondWithJSON(w, httpStatus, map[string]any{
		"status": status,
		"checks": map[string]any{
			"management_plane":  mgmtHealthy,
			"memory_providers":  memoryProviders,
			"cognition_engines": cognitionEngines,
			"mcp_server":        mcpHealthy,
		},
	})
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

// probeTCPPort checks if a TCP port is reachable
func (a *App) probeTCPPort(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
