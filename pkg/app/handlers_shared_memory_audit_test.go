package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

func newMockCognitionServer(extractionHandler, reasoningHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	if extractionHandler != nil {
		mux.HandleFunc("/api/knowledge-mgmt/extraction", extractionHandler)
	}
	if reasoningHandler != nil {
		mux.HandleFunc("/api/knowledge-mgmt/reasoning/evidence", reasoningHandler)
	}
	return httptest.NewServer(mux)
}

func newMockKnowledgeMemoryServer(upsertHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/diagnostics/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	if upsertHandler != nil {
		mux.HandleFunc("/api/knowledge/graphs", upsertHandler)
	}
	return httptest.NewServer(mux)
}

func noRetryCognitionClient(baseURL string) *cognitionagentclient.Client {
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 5 * time.Second
	cfg.MaxRetries = 0
	return cognitionagentclient.NewWithHTTPClient(baseURL, httpclient.NewWithConfig(cfg))
}

func newSharedMemoryTestApp(
	cognitionServer *httptest.Server,
	knowledgeMemServer *httptest.Server,
) *App {
	db := client.NewMockDatabase()
	cognitionClient := cognitionagentclient.New(cognitionServer.URL, 5*time.Second)

	knowledgeMemClient, err := iocmemoryprovider.NewClient(knowledgeMemServer.URL)
	if err != nil {
		panic("newSharedMemoryTestApp: knowledge mem client creation failed: " + err.Error())
	}

	return &App{
		db:                    db,
		cognitionEngineClient: cognitionClient,
		knowledgeMemSvcClient: knowledgeMemClient,
	}
}

func newSharedMemoryTestAppNoRetry(
	cognitionServer *httptest.Server,
	knowledgeMemServer *httptest.Server,
) *App {
	db := client.NewMockDatabase()

	knowledgeMemClient, err := iocmemoryprovider.NewClient(knowledgeMemServer.URL)
	if err != nil {
		panic("newSharedMemoryTestAppNoRetry: knowledge mem client creation failed: " + err.Error())
	}

	return &App{
		db:                    db,
		cognitionEngineClient: noRetryCognitionClient(cognitionServer.URL),
		knowledgeMemSvcClient: knowledgeMemClient,
	}
}

func listAllAudits(t *testing.T, db client.Database) []audit.Audit {
	t.Helper()
	resp, err := db.ListAuditEvents("", "", 0, 1000)
	require.NoError(t, err)
	return resp.Data
}

// ---------------------------------------------------------------------------
// createOrUpdateSharedMemoriesHandler — audit tests
// ---------------------------------------------------------------------------

func TestCreateOrUpdateSharedMemories_Audit_Success(t *testing.T) {
	cogServer := newMockCognitionServer(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cognitionagentclient.KnowledgeCognitionResponse{
				ResponseID: "resp-1",
				Concepts: []cognitionagentclient.Concept{
					{ID: "c1", Name: "TestConcept"},
				},
			})
		},
		nil,
	)
	defer cogServer.Close()

	reqID := "req-1"
	kmServer := newMockKnowledgeMemoryServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(iocmemoryprovider.KnowledgeGraphStoreResponse{
			RequestID: &reqID,
			Status:    iocmemoryprovider.ResponseStatusSuccess,
		})
	})
	defer kmServer.Close()

	app := newSharedMemoryTestApp(cogServer, kmServer)

	body, _ := json.Marshal(map[string]any{
		"header":  map[string]string{"agent_id": "agent-1"},
		"payload": map[string]any{"metadata": map[string]string{"format": "observe-sdk-otel"}, "data": []any{}},
	})

	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories",
		bytes.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, err := app.createOrUpdateSharedMemoriesHandler(rr, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeKnowledgeIngestion, ev.AuditType)
	assert.Equal(t, audit.ResourceTypeMAS, ev.ResourceType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)
	assert.NotNil(t, ev.OperationID)
	assert.Nil(t, ev.AuditExtraInformation)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "SUCCESS", info["status"])
	assert.Empty(t, info["error"])
}

func TestCreateOrUpdateSharedMemories_Audit_ExtractionFailure(t *testing.T) {
	cogServer := newMockCognitionServer(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"extraction boom"}`))
		},
		nil,
	)
	defer cogServer.Close()

	kmServer := newMockKnowledgeMemoryServer(nil)
	defer kmServer.Close()

	app := newSharedMemoryTestAppNoRetry(cogServer, kmServer)

	body, _ := json.Marshal(map[string]any{
		"payload": map[string]any{"metadata": map[string]string{"format": "observe-sdk-otel"}, "data": []any{}},
	})
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories",
		bytes.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, _ := app.createOrUpdateSharedMemoriesHandler(rr, req)

	assert.Equal(t, http.StatusInternalServerError, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event on extraction failure")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeKnowledgeIngestion, ev.AuditType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)
	assert.NotNil(t, ev.AuditExtraInformation)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "FAILED", info["status"])
	assert.NotEmpty(t, info["error"])
}

func TestCreateOrUpdateSharedMemories_Audit_UpsertFailure(t *testing.T) {
	cogServer := newMockCognitionServer(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cognitionagentclient.KnowledgeCognitionResponse{
				ResponseID: "resp-1",
			})
		},
		nil,
	)
	defer cogServer.Close()

	// Return HTTP 200 with a failure status to avoid triggering HTTP-level retries.
	kmServer := newMockKnowledgeMemoryServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "failure",
			"message": "upsert boom",
		})
	})
	defer kmServer.Close()

	app := newSharedMemoryTestApp(cogServer, kmServer)

	body, _ := json.Marshal(map[string]any{
		"payload": map[string]any{"metadata": map[string]string{"format": "observe-sdk-otel"}, "data": []any{}},
	})
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories",
		bytes.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, _ := app.createOrUpdateSharedMemoriesHandler(rr, req)

	assert.Equal(t, http.StatusInternalServerError, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event on upsert failure")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeKnowledgeIngestion, ev.AuditType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "FAILED", info["status"])
	assert.NotEmpty(t, info["error"])
}

// ---------------------------------------------------------------------------
// fetchSharedMemoriesHandler — audit tests
// ---------------------------------------------------------------------------

func TestFetchSharedMemories_Audit_Success(t *testing.T) {
	cogServer := newMockCognitionServer(
		nil,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cognitionagentclient.ReasonerCognitionResponse{
				ResponseID: "resp-1",
				Records: []cognitionagentclient.ReasonerRecord{
					{Content: cognitionagentclient.ReasonerContent{
						Evidence: cognitionagentclient.ReasonerEvidence{
							Status:        "success",
							FinalResponse: "The agent handles web selection tasks.",
						},
					}},
				},
			})
		},
	)
	defer cogServer.Close()

	kmServer := newMockKnowledgeMemoryServer(nil)
	defer kmServer.Close()

	app := newSharedMemoryTestApp(cogServer, kmServer)

	body := `{
		"header": {"agent_id": "agent-1"},
		"search_strategy": "semantic_graph_traversal",
		"intent": "what does the agent do?"
	}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories/query",
		strings.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, err := app.fetchSharedMemoriesHandler(rr, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeSharedMemoryOperation, ev.AuditType)
	assert.Equal(t, audit.ResourceTypeMAS, ev.ResourceType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)
	assert.NotNil(t, ev.OperationID)
	assert.Nil(t, ev.AuditExtraInformation)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "SUCCESS", info["status"])
	assert.Empty(t, info["error"])
}

func TestFetchSharedMemories_Audit_ReasoningFailure(t *testing.T) {
	cogServer := newMockCognitionServer(
		nil,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"reasoning boom"}`))
		},
	)
	defer cogServer.Close()

	kmServer := newMockKnowledgeMemoryServer(nil)
	defer kmServer.Close()

	app := newSharedMemoryTestAppNoRetry(cogServer, kmServer)

	body := `{
		"header": {"agent_id": "agent-1"},
		"intent": "tell me something"
	}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories/query",
		strings.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, _ := app.fetchSharedMemoriesHandler(rr, req)

	assert.Equal(t, http.StatusInternalServerError, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event on reasoning failure")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeSharedMemoryOperation, ev.AuditType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "FAILED", info["status"])
	assert.NotEmpty(t, info["error"])
}

func TestFetchSharedMemories_Audit_InsufficientEvidence(t *testing.T) {
	cogServer := newMockCognitionServer(
		nil,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cognitionagentclient.ReasonerCognitionResponse{
				ResponseID: "resp-1",
				Records: []cognitionagentclient.ReasonerRecord{
					{Content: cognitionagentclient.ReasonerContent{
						Evidence: cognitionagentclient.ReasonerEvidence{
							FinalResponse: "The evidence does not support an answer to this question.",
						},
					}},
				},
			})
		},
	)
	defer cogServer.Close()

	kmServer := newMockKnowledgeMemoryServer(nil)
	defer kmServer.Close()

	app := newSharedMemoryTestApp(cogServer, kmServer)

	body := `{
		"header": {"agent_id": "agent-1"},
		"intent": "some unknown topic"
	}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas1/shared-memories/query",
		strings.NewReader(body),
	)
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")

	rr := httptest.NewRecorder()
	code, _ := app.fetchSharedMemoriesHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, code)

	events := listAllAudits(t, app.db)
	require.Len(t, events, 1, "expected exactly one audit event on insufficient evidence")

	ev := events[0]
	assert.Equal(t, audit.AuditTypeSharedMemoryOperation, ev.AuditType)
	assert.Equal(t, "mas1", ev.ResourceIdentifier)

	var info map[string]string
	require.NoError(t, json.Unmarshal(ev.AuditInformation, &info))
	assert.Equal(t, "FAILED", info["status"])
	assert.NotEmpty(t, info["error"])
}