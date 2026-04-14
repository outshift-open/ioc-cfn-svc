package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
)

func newDiagnosticsTestApp() *App {
	return &App{
		buildVersion:  "1.0.0",
		gitCommitSHA:  "abc1234",
		gitCommitTime: "2025-01-01T00:00:00-08:00",
		gitBranch:     "main",
		startTime:     time.Now(),
		db:            client.NewMockDatabase(),
	}
}

func TestDiagnosticsInfoHandler(t *testing.T) {
	app := newDiagnosticsTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/info", nil)
	rr := httptest.NewRecorder()

	code, err := app.diagnosticsInfoHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	git := resp["git"].(map[string]any)
	commit := git["commit"].(map[string]any)
	assert.Equal(t, "abc1234", commit["id"])
	assert.Equal(t, "2025-01-01T00:00:00-08:00", commit["time"])
	assert.Equal(t, "main", git["branch"])
}

func TestDiagnosticsHealthHandler(t *testing.T) {
	app := newDiagnosticsTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/health", nil)
	rr := httptest.NewRecorder()

	code, err := app.diagnosticsHealthHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "UP", resp["status"])
}

func TestDiagnosticsMetricsHandler(t *testing.T) {
	app := newDiagnosticsTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/metrics", nil)
	rr := httptest.NewRecorder()

	code, err := app.diagnosticsMetricsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	assert.Contains(t, resp, "uptime_seconds")
	assert.Contains(t, resp, "goroutines")
	assert.Contains(t, resp, "memory_heap_alloc_mb")
	assert.Contains(t, resp, "memory_sys_mb")
	assert.Contains(t, resp, "gc_runs")

	assert.GreaterOrEqual(t, resp["uptime_seconds"].(float64), 0.0)
	assert.GreaterOrEqual(t, resp["goroutines"].(float64), 1.0)
}

func TestDiagnosticsLoggersHandler(t *testing.T) {
	app := newDiagnosticsTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/loggers", nil)
	rr := httptest.NewRecorder()

	code, err := app.diagnosticsLoggersHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp, "ROOT")
}

func TestDiagnosticsSetLoggersHandler(t *testing.T) {
	app := newDiagnosticsTestApp()

	t.Run("returns 204 on valid level for ROOT", func(t *testing.T) {
		body := `{"module-name": "ROOT", "log-level": "debug"}`
		req := httptest.NewRequest(http.MethodPut, "/api/internal/diagnostics/loggers", strings.NewReader(body))
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsSetLoggersHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, code)
	})

	t.Run("returns 400 on invalid log level", func(t *testing.T) {
		body := `{"module-name": "ROOT", "log-level": "nonsense"}`
		req := httptest.NewRequest(http.MethodPut, "/api/internal/diagnostics/loggers", strings.NewReader(body))
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsSetLoggersHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("returns 400 on missing log level", func(t *testing.T) {
		body := `{"module-name": "ROOT"}`
		req := httptest.NewRequest(http.MethodPut, "/api/internal/diagnostics/loggers", strings.NewReader(body))
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsSetLoggersHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("returns 400 on unknown module", func(t *testing.T) {
		body := `{"module-name": "nonexistent-pkg", "log-level": "debug"}`
		req := httptest.NewRequest(http.MethodPut, "/api/internal/diagnostics/loggers", strings.NewReader(body))
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsSetLoggersHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, code)
	})
}
