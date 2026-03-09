//go:build integration
// +build integration

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

func TestUpsertSharedMemoriesHandler_KnowledgeExtraction_Otel_Integration(t *testing.T) {
	// --- setup ---
	knowledgeMemClient, _ := iocmemoryprovider.NewClient("http://localhost:9003")
	cognitionAgentsClient := cognitionagentclient.New(
		"http://localhost:9004",
		30*time.Second,
	)

	app := &App{
		knowledgeMemSvcClient: knowledgeMemClient,
		cognitionAgentsClient: cognitionAgentsClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories",
		func(w http.ResponseWriter, r *http.Request) {
			app.upsertSharedMemoriesHandler(w, r)
		},
	)

	dataBytes, err := os.ReadFile("testdata/otel.json")
	if err != nil {
		t.Fatalf("failed to read test payload: %v", err)
	}

	if !json.Valid(dataBytes) {
		t.Fatal("otel.json is not valid JSON")
	}

	body, err := json.Marshal(map[string]any{
		"agent_id": "agent-1",
		"payload": map[string]any{
			"metadata": map[string]string{
				"format": "observe-sdk-otel",
			},
			"data": json.RawMessage(dataBytes),
		},
	})
	if err != nil {
		t.Fatalf("failed to build request body: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas_otel/shared-memories",
		bytes.NewReader(body),
	)

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf(
			"unexpected status code: got=%d want=%d\nresponse body:\n%s",
			w.Code,
			http.StatusCreated,
			w.Body.String(),
		)
	}
}

func TestUpsertSharedMemoriesHandler_KnowledgeExtraction_OpenClaw_Integration(t *testing.T) {
	// --- setup ---
	knowledgeMemClient, _ := iocmemoryprovider.NewClient("http://localhost:9003")
	cognitionAgentsClient := cognitionagentclient.New(
		"http://localhost:9004",
		30*time.Second,
	)

	app := &App{
		knowledgeMemSvcClient: knowledgeMemClient,
		cognitionAgentsClient: cognitionAgentsClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories",
		func(w http.ResponseWriter, r *http.Request) {
			app.upsertSharedMemoriesHandler(w, r)
		},
	)

	dataBytes, err := os.ReadFile("testdata/openclaw.json")
	if err != nil {
		t.Fatalf("failed to read test payload: %v", err)
	}

	if !json.Valid(dataBytes) {
		t.Fatal("openclaw.json is not valid JSON")
	}

	body, err := json.Marshal(map[string]any{
		"agent_id": "agent-1",
		"payload": map[string]any{
			"metadata": map[string]string{
				"format": "openclaw",
			},
			"data": json.RawMessage(dataBytes),
		},
	})
	if err != nil {
		t.Fatalf("failed to build request body: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas_openclaw/shared-memories",
		bytes.NewReader(body),
	)

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf(
			"unexpected status code: got=%d want=%d\nresponse body:\n%s",
			w.Code,
			http.StatusCreated,
			w.Body.String(),
		)
	}
}

func TestFetchSharedMemoriesHandler_EvidenceAndReasoning_Integration(t *testing.T) {
	// --- setup ---
	knowledgeMemClient, _ := iocmemoryprovider.NewClient("http://localhost:9003")
	cognitionAgentsClient := cognitionagentclient.New(
		"http://localhost:9004",
		30*time.Second,
	)

	app := &App{
		knowledgeMemSvcClient: knowledgeMemClient,
		cognitionAgentsClient: cognitionAgentsClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query",
		func(w http.ResponseWriter, r *http.Request) {
			app.fetchSharedMemoriesHandler(w, r)
		},
	)

	// Test for query from Otel trace ingestion
	body := `{
        "agent_id": "agent-2",
		"search_strategy": "semantic_graph_traversal",
		"intent": "what does the website_selector_agent do?"
    }`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas_otel/shared-memories/query",
		strings.NewReader(body),
	)

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf(
			"unexpected status code: got=%d want=%d\nresponse body:\n%s",
			w.Code,
			http.StatusOK,
			w.Body.String(),
		)
	}

	// Test for query from Open Claw ingestion
	body = `{
        "agent_id": "agent-2",
		"search_strategy": "semantic_graph_traversal",
		"intent": "Tell me something about Q2 budget planning"
    }`

	req = httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/ws1/multi-agentic-systems/mas_openclaw/shared-memories/query",
		strings.NewReader(body),
	)

	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf(
			"unexpected status code: got=%d want=%d\nresponse body:\n%s",
			w.Code,
			http.StatusOK,
			w.Body.String(),
		)
	}
}
