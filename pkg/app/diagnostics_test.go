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

func TestDiagnosticsHealthHandlerWithDependencies(t *testing.T) {
	healthyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	unhealthyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	t.Run("all dependencies UP", func(t *testing.T) {
		mgmt := httptest.NewServer(healthyHandler)
		defer mgmt.Close()
		mem := httptest.NewServer(healthyHandler)
		defer mem.Close()
		cog := httptest.NewServer(healthyHandler)
		defer cog.Close()

		t.Setenv("MGMT_URL", mgmt.URL)
		t.Setenv("KNOWLEDGE_MEMORY_SVC_URL", mem.URL)
		t.Setenv("COGNITION_ENGINE_SVC_URL", cog.URL)

		app := newDiagnosticsTestApp()
		req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/health?dependencies=true", nil)
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsHealthHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "UP", resp["status"])
		checks := resp["checks"].(map[string]any)
		assert.Equal(t, true, checks["management_plane"])
		assert.Equal(t, true, checks["knowledge_memory_svc"])
		assert.Equal(t, true, checks["cognition_engine"])
	})

	t.Run("critical dependency DOWN returns DOWN and 500", func(t *testing.T) {
		mgmt := httptest.NewServer(healthyHandler)
		defer mgmt.Close()
		mem := httptest.NewServer(unhealthyHandler)
		defer mem.Close()
		cog := httptest.NewServer(healthyHandler)
		defer cog.Close()

		t.Setenv("MGMT_URL", mgmt.URL)
		t.Setenv("KNOWLEDGE_MEMORY_SVC_URL", mem.URL)
		t.Setenv("COGNITION_ENGINE_SVC_URL", cog.URL)

		app := newDiagnosticsTestApp()
		req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/health?dependencies=true", nil)
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsHealthHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "DOWN", resp["status"])
		checks := resp["checks"].(map[string]any)
		assert.Equal(t, true, checks["management_plane"])
		assert.Equal(t, false, checks["knowledge_memory_svc"])
		assert.Equal(t, true, checks["cognition_engine"])
	})

	t.Run("non-critical dependency DOWN returns DEGRADED and 200", func(t *testing.T) {
		mgmt := httptest.NewServer(healthyHandler)
		defer mgmt.Close()
		mem := httptest.NewServer(healthyHandler)
		defer mem.Close()
		cog := httptest.NewServer(unhealthyHandler)
		defer cog.Close()

		t.Setenv("MGMT_URL", mgmt.URL)
		t.Setenv("KNOWLEDGE_MEMORY_SVC_URL", mem.URL)
		t.Setenv("COGNITION_ENGINE_SVC_URL", cog.URL)

		app := newDiagnosticsTestApp()
		req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/health?dependencies=true", nil)
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsHealthHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "DEGRADED", resp["status"])
		checks := resp["checks"].(map[string]any)
		assert.Equal(t, true, checks["management_plane"])
		assert.Equal(t, true, checks["knowledge_memory_svc"])
		assert.Equal(t, false, checks["cognition_engine"])
	})

	t.Run("unreachable dependency counts as DOWN", func(t *testing.T) {
		t.Setenv("MGMT_URL", "http://127.0.0.1:1")
		t.Setenv("KNOWLEDGE_MEMORY_SVC_URL", "http://127.0.0.1:1")

		app := newDiagnosticsTestApp()
		req := httptest.NewRequest(http.MethodGet, "/api/internal/diagnostics/health?dependencies=true", nil)
		rr := httptest.NewRecorder()

		code, err := app.diagnosticsHealthHandler(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "DOWN", resp["status"])
	})
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
