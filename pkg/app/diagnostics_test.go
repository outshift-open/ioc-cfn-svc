package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
