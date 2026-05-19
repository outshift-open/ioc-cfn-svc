package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestMetricsHandler_Success(t *testing.T) {
	app := newTestApp()

	workspaceID := uuid.New()
	masID := uuid.New()
	sessionID := uuid.New()

	payload := IngestMetricsRequest{
		WorkspaceID:   workspaceID.String(),
		MASID:         masID.String(),
		AgentID:       "agent-1",
		SessionID:     sessionID.String(),
		OperationType: "semantic_negotiation",
		Model:         "gpt-4",
		Metrics: map[string]float64{
			"token_input":  1000,
			"token_output": 500,
			"token_total":  1500,
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/internal/metrics/ingest", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	code, err := app.ingestMetricsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "accepted", resp["status"])

	// Give async storage time to complete
	time.Sleep(100 * time.Millisecond)
}

func TestIngestMetricsHandler_MissingFields(t *testing.T) {
	app := newTestApp()

	tests := []struct {
		name        string
		payload     IngestMetricsRequest
		expectedErr string
	}{
		{
			name: "missing workspace_id",
			payload: IngestMetricsRequest{
				MASID:     uuid.New().String(),
				AgentID:   "agent-1",
				SessionID: uuid.New().String(),
				Metrics:   map[string]float64{"token_total": 100},
			},
			expectedErr: "workspace_id, mas_id, agent_id, and session_id are required",
		},
		{
			name: "missing mas_id",
			payload: IngestMetricsRequest{
				WorkspaceID: uuid.New().String(),
				AgentID:     "agent-1",
				SessionID:   uuid.New().String(),
				Metrics:     map[string]float64{"token_total": 100},
			},
			expectedErr: "workspace_id, mas_id, agent_id, and session_id are required",
		},
		{
			name: "missing agent_id",
			payload: IngestMetricsRequest{
				WorkspaceID: uuid.New().String(),
				MASID:       uuid.New().String(),
				SessionID:   uuid.New().String(),
				Metrics:     map[string]float64{"token_total": 100},
			},
			expectedErr: "workspace_id, mas_id, agent_id, and session_id are required",
		},
		{
			name: "empty metrics",
			payload: IngestMetricsRequest{
				WorkspaceID: uuid.New().String(),
				MASID:       uuid.New().String(),
				AgentID:     "agent-1",
				SessionID:   uuid.New().String(),
				Metrics:     map[string]float64{},
			},
			expectedErr: "metrics field must contain at least one metric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/internal/metrics/ingest", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			code, err := app.ingestMetricsHandler(rr, req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, code)

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
			assert.Contains(t, resp["error"], tt.expectedErr)
		})
	}
}

func TestIngestMetricsHandler_InvalidUUIDs(t *testing.T) {
	app := newTestApp()

	tests := []struct {
		name        string
		workspaceID string
		masID       string
		sessionID   string
		expectedErr string
	}{
		{
			name:        "invalid workspace_id",
			workspaceID: "not-a-uuid",
			masID:       uuid.New().String(),
			sessionID:   uuid.New().String(),
			expectedErr: "workspace_id must be a valid UUID",
		},
		{
			name:        "invalid mas_id",
			workspaceID: uuid.New().String(),
			masID:       "not-a-uuid",
			sessionID:   uuid.New().String(),
			expectedErr: "mas_id must be a valid UUID",
		},
		{
			name:        "invalid session_id",
			workspaceID: uuid.New().String(),
			masID:       uuid.New().String(),
			sessionID:   "not-a-uuid",
			expectedErr: "session_id must be a valid UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := IngestMetricsRequest{
				WorkspaceID: tt.workspaceID,
				MASID:       tt.masID,
				AgentID:     "agent-1",
				SessionID:   tt.sessionID,
				Metrics:     map[string]float64{"token_total": 100},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/internal/metrics/ingest", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			code, err := app.ingestMetricsHandler(rr, req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, code)

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
			assert.Contains(t, resp["error"], tt.expectedErr)
		})
	}
}

// Note: Full integration tests with real database seeding require a real database connection.
// These tests focus on validation and error handling.

func TestGetWorkspaceTokenMetricsHandler_InvalidWorkspaceID(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/workspaces/not-a-uuid/metrics/tokens?start_time=2024-01-01T00:00:00Z&end_time=2024-01-02T00:00:00Z",
		nil,
	)
	req.SetPathValue("workspaceId", "not-a-uuid")
	rr := httptest.NewRecorder()

	code, err := app.getWorkspaceTokenMetricsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "workspace_id must be a valid UUID")
}

func TestGetWorkspaceTokenMetricsHandler_MissingTimeParams(t *testing.T) {
	app := newTestApp()
	workspaceID := uuid.New()

	tests := []struct {
		name        string
		startTime   string
		endTime     string
		expectedErr string
	}{
		{
			name:        "missing start_time",
			startTime:   "",
			endTime:     "2024-01-02T00:00:00Z",
			expectedErr: "start_time and end_time are required",
		},
		{
			name:        "missing end_time",
			startTime:   "2024-01-01T00:00:00Z",
			endTime:     "",
			expectedErr: "start_time and end_time are required",
		},
		{
			name:        "invalid start_time format",
			startTime:   "2024-01-01",
			endTime:     "2024-01-02T00:00:00Z",
			expectedErr: "start_time must be in ISO 8601 format",
		},
		{
			name:        "invalid end_time format",
			startTime:   "2024-01-01T00:00:00Z",
			endTime:     "not-a-date",
			expectedErr: "end_time must be in ISO 8601 format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/api/workspaces/%s/metrics/tokens", workspaceID.String())
			if tt.startTime != "" || tt.endTime != "" {
				url += "?"
				if tt.startTime != "" {
					url += "start_time=" + tt.startTime
				}
				if tt.endTime != "" {
					if tt.startTime != "" {
						url += "&"
					}
					url += "end_time=" + tt.endTime
				}
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("workspaceId", workspaceID.String())
			rr := httptest.NewRecorder()

			code, err := app.getWorkspaceTokenMetricsHandler(rr, req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, code)

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
			assert.Contains(t, resp["error"], tt.expectedErr)
		})
	}
}

// Note: Integration tests with multiple agents require a real database connection.
// See integration test suite for full end-to-end testing.

func TestGetMASTokenMetricsHandler_InvalidMASID(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/mas/not-a-uuid/metrics/tokens?start_time=2024-01-01T00:00:00Z&end_time=2024-01-02T00:00:00Z",
		nil,
	)
	req.SetPathValue("masId", "not-a-uuid")
	rr := httptest.NewRecorder()

	code, err := app.getMASTokenMetricsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "mas_id must be a valid UUID")
}
