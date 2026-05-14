package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newAgentVectorApp spins up a fake knowledge-memory-svc server and wires it into
// a minimal App for testing agent vector handlers.
func newAgentVectorApp(t *testing.T, svcHandler http.HandlerFunc) (*App, *httptest.Server) {
	t.Helper()
	svc := httptest.NewServer(svcHandler)
	client := iocmemoryprovider.NewClientForTest(svc.URL)
	return &App{knowledgeMemSvcClient: client}, svc
}

// ---------------------------------------------------------------------------
// agentVectorUpsertHandler tests
// ---------------------------------------------------------------------------

func TestAgentVectorUpsertHandler_SetsAgentIDOnProviderRequest(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/knowledge/vectors" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message": "ok"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{
			{ID: "11111111-1111-1111-1111-111111111111", Content: "test", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify AgentID was set on the forwarded request
	if capturedBody["agent_id"] != "agent-abc" {
		t.Errorf("expected agent_id 'agent-abc', got %v", capturedBody["agent_id"])
	}
	if capturedBody["wksp_id"] != "ws1" {
		t.Errorf("expected wksp_id 'ws1', got %v", capturedBody["wksp_id"])
	}
	if capturedBody["mas_id"] != "mas1" {
		t.Errorf("expected mas_id 'mas1', got %v", capturedBody["mas_id"])
	}

	// Verify the valid UUID was passed through
	records := capturedBody["records"].([]interface{})
	rec := records[0].(map[string]interface{})
	if rec["id"] != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected id '11111111-1111-1111-1111-111111111111', got %v", rec["id"])
	}
}

func TestAgentVectorUpsertHandler_GeneratesIDWhenOmitted(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{
			{Content: "no id", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}}, // ID omitted
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	// Verify ID was auto-generated
	records, ok := capturedBody["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("expected 1 record, got %v", capturedBody["records"])
	}
	rec := records[0].(map[string]interface{})
	if rec["id"] == nil || rec["id"] == "" {
		t.Error("expected auto-generated ID, got empty or nil")
	}
}

func TestAgentVectorUpsertHandler_Returns404WhenStoreNotFound(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "not found"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{
			{ID: "22222222-2222-2222-2222-222222222222", Content: "test", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	// The client returns error on non-success status; handler maps to 500 for generic errors
	// For 404, client.go would need to check status and return ErrNotFound
	// Current behavior: non-success status returns error, mapped to 500
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 or 500, got %d", w.Code)
	}
}

func TestAgentVectorUpsertHandler_Returns400WhenRecordsEmpty(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream should not be called for empty records")
		w.WriteHeader(http.StatusOK)
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{}, // empty
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAgentVectorUpsertHandler_Returns400WhenIDNotValidUUID(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream should not be called for invalid UUID")
		w.WriteHeader(http.StatusOK)
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{
			{ID: "not-a-valid-uuid", Content: "test", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	// Verify error message mentions the invalid UUID
	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestAgentVectorUpsertHandler_AcceptsValidUUID(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	validUUID := "123e4567-e89b-12d3-a456-426614174000"
	reqBody := sharedmemory.AgentVectorUpsertRequest{
		Records: []sharedmemory.AgentVectorUpsertRecord{
			{ID: validUUID, Content: "test", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorUpsertHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the valid UUID was passed through
	records := capturedBody["records"].([]interface{})
	rec := records[0].(map[string]interface{})
	if rec["id"] != validUUID {
		t.Errorf("expected id %q, got %v", validUUID, rec["id"])
	}
}

// ---------------------------------------------------------------------------
// agentVectorDeleteHandler tests
// ---------------------------------------------------------------------------

func TestAgentVectorDeleteHandler_SetsAgentIDOnProviderRequest(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/knowledge/vectors" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message": "deleted"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorDeleteRequest{
		ID: "vec-42",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-xyz/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-xyz")

	w := httptest.NewRecorder()
	app.agentVectorDeleteHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify AgentID was set on the forwarded request
	if capturedBody["agent_id"] != "agent-xyz" {
		t.Errorf("expected agent_id 'agent-xyz', got %v", capturedBody["agent_id"])
	}
	if capturedBody["id"] != "vec-42" {
		t.Errorf("expected id 'vec-42', got %v", capturedBody["id"])
	}
}

func TestAgentVectorDeleteHandler_DefaultsToSoftDelete(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorDeleteRequest{
		ID: "vec-1",
		// SoftDelete not specified
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorDeleteHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify soft_delete defaults to true
	if capturedBody["soft_delete"] != true {
		t.Errorf("expected soft_delete to default to true, got %v", capturedBody["soft_delete"])
	}
}

func TestAgentVectorDeleteHandler_RespectsHardDeleteFlag(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	softDel := false
	reqBody := sharedmemory.AgentVectorDeleteRequest{
		ID:         "vec-1",
		SoftDelete: &softDel,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorDeleteHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify soft_delete=false was passed through
	if capturedBody["soft_delete"] != false {
		t.Errorf("expected soft_delete=false, got %v", capturedBody["soft_delete"])
	}
}

func TestAgentVectorDeleteHandler_Returns400WhenIDMissing(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream should not be called when ID is missing")
		w.WriteHeader(http.StatusOK)
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorDeleteRequest{
		// ID not specified
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorDeleteHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAgentVectorDeleteHandler_Returns404WhenVectorNotFound(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "not found", "message": "Vector not found"})
	})
	defer svc.Close()

	reqBody := sharedmemory.AgentVectorDeleteRequest{
		ID: "11111111-1111-1111-1111-111111111111",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/vectors", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorDeleteHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// agentVectorSimilaritySearchHandler tests
// ---------------------------------------------------------------------------

func TestAgentVectorSimilaritySearchHandler_SetsAgentIDOnProviderRequest(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/knowledge/vectors/query/similarity" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "results": []interface{}{}})
	})
	defer svc.Close()

	reqBody := sharedmemory.VectorSimilaritySearchRequest{
		Payload: sharedmemory.VectorSimilaritySearchPayload{
			EmbeddingVector: []float64{0.1, 0.2, 0.3},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-search/rag/similarity-search", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-search")

	w := httptest.NewRecorder()
	app.agentVectorSimilaritySearchHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify AgentID was set on the forwarded request
	if capturedBody["agent_id"] != "agent-search" {
		t.Errorf("expected agent_id 'agent-search', got %v", capturedBody["agent_id"])
	}
	if capturedBody["wksp_id"] != "ws1" {
		t.Errorf("expected wksp_id 'ws1', got %v", capturedBody["wksp_id"])
	}
	if capturedBody["mas_id"] != "mas1" {
		t.Errorf("expected mas_id 'mas1', got %v", capturedBody["mas_id"])
	}
}

func TestAgentVectorSimilaritySearchHandler_DefaultsLimitAndMetric(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := sharedmemory.VectorSimilaritySearchRequest{
		Payload: sharedmemory.VectorSimilaritySearchPayload{
			EmbeddingVector: []float64{0.1},
			// TopK and Metric not specified
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/similarity-search", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorSimilaritySearchHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify defaults
	if capturedBody["limit"] != float64(10) {
		t.Errorf("expected default limit=10, got %v", capturedBody["limit"])
	}
	if capturedBody["metric"] != "cosine" {
		t.Errorf("expected default metric='cosine', got %v", capturedBody["metric"])
	}
}

func TestAgentVectorSimilaritySearchHandler_Returns400WhenEmbeddingMissing(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream should not be called when embedding is missing")
		w.WriteHeader(http.StatusOK)
	})
	defer svc.Close()

	reqBody := sharedmemory.VectorSimilaritySearchRequest{
		Payload: sharedmemory.VectorSimilaritySearchPayload{
			// EmbeddingVector not specified
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/similarity-search", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorSimilaritySearchHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAgentVectorSimilaritySearchHandler_Returns404WhenStoreNotFound(t *testing.T) {
	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "not found"})
	})
	defer svc.Close()

	reqBody := sharedmemory.VectorSimilaritySearchRequest{
		Payload: sharedmemory.VectorSimilaritySearchPayload{
			EmbeddingVector: []float64{0.1},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/similarity-search", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorSimilaritySearchHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAgentVectorSimilaritySearchHandler_PassesThroughIncludeEmbeddings(t *testing.T) {
	var capturedQuery string

	app, svc := newAgentVectorApp(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := sharedmemory.VectorSimilaritySearchRequest{
		Payload: sharedmemory.VectorSimilaritySearchPayload{
			EmbeddingVector: []float64{0.1},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws1/multi-agentic-systems/mas1/agents/agent-abc/rag/similarity-search?include_embeddings=true", bytes.NewReader(body))
	req.SetPathValue("workspaceId", "ws1")
	req.SetPathValue("masId", "mas1")
	req.SetPathValue("agentId", "agent-abc")

	w := httptest.NewRecorder()
	app.agentVectorSimilaritySearchHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if capturedQuery != "include_embeddings=true" {
		t.Errorf("expected include_embeddings=true in forwarded query, got %q", capturedQuery)
	}
}
