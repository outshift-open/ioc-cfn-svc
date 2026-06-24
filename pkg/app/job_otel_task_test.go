// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/outshift-open/ioc-cfn-svc/pkg/client"
	"github.com/outshift-open/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/outshift-open/ioc-cfn-svc/pkg/config"
	"github.com/outshift-open/ioc-cfn-svc/pkg/model"
	"github.com/outshift-open/ioc-cfn-svc/pkg/otelreceiver"
	iocmemoryprovider "github.com/outshift-open/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/outshift-open/ioc-cfn-svc/pkg/task"
)

type otelTaskExecutionDB struct {
	*client.MockDatabase

	claimedTraceIDs []string
	spansByTraceID  map[string][]otelreceiver.OtelSpan

	traceStatusUpdates map[string]string
	taskStatus         string
	taskFields         map[string]interface{}
	historyFields      map[string]interface{}
}

type otelTaskSyncDB struct {
	*client.MockDatabase

	existingTask *model.Task
	upsertedTask *model.Task
}

func (db *otelTaskExecutionDB) ClaimReadyOtelTraces(_, _ string, _ int, _ time.Duration) ([]string, error) {
	return db.claimedTraceIDs, nil
}

func (db *otelTaskExecutionDB) GetOtelSpansForTrace(_, _, traceID string) ([]otelreceiver.OtelSpan, error) {
	return db.spansByTraceID[traceID], nil
}

func (db *otelTaskExecutionDB) UpdateOtelTraceStatus(_, _, traceID, status string) error {
	if db.traceStatusUpdates == nil {
		db.traceStatusUpdates = make(map[string]string)
	}
	db.traceStatusUpdates[traceID] = status
	return nil
}

func (db *otelTaskExecutionDB) UpdateTaskStatus(_ string, status string, fields map[string]interface{}) error {
	db.taskStatus = status
	db.taskFields = fields
	return nil
}

func (db *otelTaskExecutionDB) UpdateTaskExecutionHistory(_ string, fields map[string]interface{}) error {
	db.historyFields = fields
	return nil
}

func (db *otelTaskSyncDB) FindTaskByKey(_, _, _ string) (*model.Task, error) {
	return db.existingTask, nil
}

func (db *otelTaskSyncDB) UpsertTask(t *model.Task) error {
	copyTask := *t
	db.upsertedTask = &copyTask
	return nil
}

func (db *otelTaskSyncDB) DeleteTasksNotInSet(_ map[string]bool) ([]model.Task, error) {
	return nil, nil
}

func TestSendOtelTaskExecutionUsesExtractionAndPersistsResponse(t *testing.T) {
	workspaceID := uuid.NewString()
	masID := uuid.NewString()
	taskID := uuid.NewString()
	historyID := uuid.NewString()
	startTime := time.Date(2026, 6, 3, 19, 0, 0, 0, time.UTC)

	db := &otelTaskExecutionDB{
		MockDatabase:    client.NewMockDatabase(),
		claimedTraceIDs: []string{"trace-1"},
		spansByTraceID: map[string][]otelreceiver.OtelSpan{
			"trace-1": {
				{
					StartTime:     startTime,
					TraceID:       "trace-1",
					SpanID:        "span-1",
					WorkspaceID:   uuid.MustParse(workspaceID),
					MasID:         uuid.MustParse(masID),
					Name:          "agent.reply",
					ServiceName:   "openclaw",
					DurationNano:  123,
					StatusCode:    1,
					StatusMessage: "ok",
					Attributes:    datatypes.JSON([]byte(`{"openclaw.message.channel":"final"}`)),
					Events:        datatypes.JSON([]byte(`[]`)),
					Links:         datatypes.JSON([]byte(`[]`)),
					Resource:      datatypes.JSON([]byte(`{"service.name":"openclaw"}`)),
				},
			},
		},
	}

	var extractionReq map[string]interface{}
	ceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/knowledge-mgmt/extraction", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&extractionReq))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"header": map[string]string{
				"workspace_id": workspaceID,
				"mas_id":       masID,
			},
			"response_id": "ce-response-1",
			"concepts": []map[string]interface{}{
				{
					"id":          "concept-1",
					"name":        "Agent Reply",
					"description": "agent reply span",
					"type":        "concept",
					"attributes": map[string]interface{}{
						"concept_type": "span",
					},
				},
			},
			"relations": []map[string]interface{}{},
			"metadata": map[string]interface{}{
				"records_processed":   1,
				"concepts_extracted":  1,
				"relations_extracted": 0,
			},
		}))
	}))
	defer ceServer.Close()

	var graphReq map[string]interface{}
	kmsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/internal/diagnostics/health" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/knowledge/graphs/query" {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(iocmemoryprovider.KnowledgeGraphQueryResponse{
				Status: iocmemoryprovider.ResponseStatusSuccess,
			}))
			return
		}

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/knowledge/graphs", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&graphReq))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(iocmemoryprovider.KnowledgeGraphStoreResponse{
			RequestID: &historyID,
			Status:    iocmemoryprovider.ResponseStatusSuccess,
		}))
	}))
	defer kmsServer.Close()

	knowledgeMemClient, err := iocmemoryprovider.NewClient(kmsServer.URL)
	require.NoError(t, err)

	app := &App{
		db:                    db,
		Cfg:                   config.Config{},
		knowledgeMemSvcClient: knowledgeMemClient,
		cognitionAgentsClient: cognitionagentclient.New(ceServer.URL, 5*time.Second),
	}

	schedule := "* * * * *"
	app.sendTaskExecution(model.Task{
		ID:          taskID,
		Name:        "Knowledge Extraction CE",
		Schedule:    &schedule,
		WorkspaceID: workspaceID,
		MASID:       masID,
	}, task.GetEndpointForCE(t.Name()), historyID)

	require.NotNil(t, extractionReq)
	assert.Equal(t, historyID, extractionReq["request_id"])
	header := extractionReq["header"].(map[string]interface{})
	assert.Equal(t, workspaceID, header["workspace_id"])
	assert.Equal(t, masID, header["mas_id"])
	payload := extractionReq["payload"].(map[string]interface{})
	metadata := payload["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-trace", metadata["format"])
	data := payload["data"].([]interface{})
	require.Len(t, data, 1)
	traceGroup := data[0].(map[string]interface{})
	assert.Equal(t, "trace-1", traceGroup["trace_id"])
	assert.Equal(t, float64(1), traceGroup["span_count"])
	require.Len(t, traceGroup["spans"].([]interface{}), 1)

	require.NotNil(t, graphReq)
	assert.Equal(t, historyID, graphReq["request_id"])
	assert.Equal(t, workspaceID, graphReq["wksp_id"])
	assert.Equal(t, masID, graphReq["mas_id"])
	assert.Equal(t, false, graphReq["incremental_update"])
	assert.Equal(t, true, graphReq["force_replace"])
	records := graphReq["records"].(map[string]interface{})
	concepts := records["concepts"].([]interface{})
	require.Len(t, concepts, 1)
	assert.Equal(t, "concept-1", concepts[0].(map[string]interface{})["id"])

	assert.Equal(t, "completed", db.traceStatusUpdates["trace-1"])
	assert.Equal(t, "scheduled", db.taskStatus)
	require.NotNil(t, db.historyFields)
	assert.Equal(t, "success", db.historyFields["status"])
	result := db.historyFields["result"].(string)
	assert.Contains(t, result, historyID)
	assert.Contains(t, result, "graph_status")
}

func TestCompleteTaskExecutionKeepsExternalTaskRunnable(t *testing.T) {
	db := &otelTaskExecutionDB{MockDatabase: client.NewMockDatabase()}
	app := &App{db: db}

	before := time.Now()
	app.completeTaskExecution(
		model.Task{
			ID:          uuid.NewString(),
			Name:        "Knowledge Extraction CE",
			WorkspaceID: uuid.NewString(),
			MASID:       uuid.NewString(),
		},
		uuid.NewString(),
		"success",
		map[string]string{"status": "ok"},
		nil,
	)

	assert.Equal(t, "scheduled", db.taskStatus)
	require.NotNil(t, db.taskFields)
	nextRun, ok := db.taskFields["next_run_time"].(time.Time)
	require.True(t, ok)
	// Externally-triggered tasks should stay runnable (next_run_time ~ now), not parked.
	assert.False(t, nextRun.After(time.Now().Add(time.Minute)), "next_run_time should not be far in the future")
	assert.False(t, nextRun.Before(before.Add(-time.Second)), "next_run_time should not be in the past")
	assert.Equal(t, "success", db.historyFields["status"])
}
