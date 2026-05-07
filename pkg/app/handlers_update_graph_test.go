package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

// newUpdateGraphApp spins up a fake knowledge-memory-svc server whose POST
// /api/knowledge/graphs handler is provided by the caller, and wires it into
// a minimal App ready to call updateGraphHandler.
//
// Uses NewClientForTest (no retries, no health check) so tests are fast even
// when the fake server returns 5xx.
func newUpdateGraphApp(t *testing.T, svcHandler http.HandlerFunc) (*App, *httptest.Server) {
	t.Helper()

	svc := httptest.NewServer(svcHandler)
	client := iocmemoryprovider.NewClientForTest(svc.URL)
	return &App{knowledgeMemSvcClient: client}, svc
}

// doUpdateGraph sends a PUT to updateGraphHandler via httptest.
func doUpdateGraph(t *testing.T, app *App, workspaceID, masID string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/internal/workspaces/"+workspaceID+"/multi-agentic-systems/"+masID+"/graph/update", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", workspaceID)
	req.SetPathValue("masId", masID)

	rr := httptest.NewRecorder()
	app.updateGraphHandler(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Success path
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_Success(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/knowledge/graphs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		// Decode and verify forwarded payload.
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode forwarded body: %v", err)
		}
		if payload["mas_id"] != "mas-1" {
			t.Errorf("expected mas_id 'mas-1', got %v", payload["mas_id"])
		}
		if payload["wksp_id"] != "ws-1" {
			t.Errorf("expected wksp_id 'ws-1', got %v", payload["wksp_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
		})
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"header": map[string]string{
			"agent_id": "agent-42",
		},
		"request_id": "req-abc",
		// Only the new distilled concept is in this batch.
		// The relation also references "anchor_concept_id" which already exists
		// in the graph — this must NOT be rejected by client-side validation.
		"concepts": []map[string]interface{}{
			{
				"id":          "15118c8b99e5813a2239279f0d7fb7c6",
				"name":        "CoDiN",
				"description": "The distilled information",
				"type":        "CoDiN",
				"attributes": map[string]interface{}{
					"embedding": []float64{0.042, 0.018, -0.079},
				},
			},
		},
		"relations": []map[string]interface{}{
			{
				"id": "91b581b91afb041bdcca33d74ab687c2",
				// anchor_concept_id is an existing graph node, not in concepts above.
				"node_ids":     []string{"4e706aec50174e58f15a52a53e6ca4f5", "15118c8b99e5813a2239279f0d7fb7c6"},
				"relationship": "CoDi",
				"attributes": map[string]interface{}{
					"source_name":    "anchor concept",
					"target_name":    "CoDiN",
					"distill_status": "CoDi",
				},
			},
		},
		"descriptor": "Cognition Distillation",
		"metadata": map[string]interface{}{
			"added_distilled_nodes":     1,
			"added_distilled_relations": 1,
		},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp updateGraphResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ResponseID != "req-abc" {
		t.Errorf("expected response_id 'req-abc', got %q", resp.ResponseID)
	}
	if resp.ConceptsUpdated != 1 {
		t.Errorf("expected concepts_updated 1, got %d", resp.ConceptsUpdated)
	}
	if resp.RelationsUpdated != 1 {
		t.Errorf("expected relations_updated 1, got %d", resp.RelationsUpdated)
	}
	if resp.Descriptor != "Cognition Distillation" {
		t.Errorf("expected descriptor 'Cognition Distillation', got %q", resp.Descriptor)
	}
	if resp.Error != nil {
		t.Errorf("expected nil error, got %v", resp.Error)
	}
	if resp.UpdatedAt == 0 {
		t.Error("expected non-zero updated_at")
	}
	if resp.Header.AgentID != "agent-42" {
		t.Errorf("expected agent_id 'agent-42', got %q", resp.Header.AgentID)
	}
}

// ---------------------------------------------------------------------------
// request_id is auto-generated when absent
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_AutoGeneratesRequestID(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"concepts":  []interface{}{},
		"relations": []interface{}{},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp updateGraphResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ResponseID == "" {
		t.Error("expected auto-generated response_id, got empty string")
	}
}

// ---------------------------------------------------------------------------
// Relation field mapping: "relationship" → "relation"
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_RelationshipFieldMappedToRelation(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"concepts": []map[string]interface{}{
			{"id": "c1", "name": "ConceptA"},
			{"id": "c2", "name": "ConceptB"},
		},
		"relations": []map[string]interface{}{
			{
				"id":           "r1",
				"node_ids":     []string{"c1", "c2"},
				"relationship": "CONNECTS_TO",
			},
		},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	// Inspect what was forwarded to knowledge-memory-svc.
	records, ok := capturedBody["records"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected records in forwarded body, got %v", capturedBody)
	}
	relations, _ := records["relations"].([]interface{})
	if len(relations) != 1 {
		t.Fatalf("expected 1 relation forwarded, got %d", len(relations))
	}
	rel := relations[0].(map[string]interface{})
	if rel["relation"] != "CONNECTS_TO" {
		t.Errorf("expected forwarded field 'relation'='CONNECTS_TO', got %v", rel["relation"])
	}
	if _, hasRelationship := rel["relationship"]; hasRelationship {
		t.Error("forwarded body should not contain 'relationship' key")
	}
}

// ---------------------------------------------------------------------------
// 404 from downstream → handler returns 404
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_404FromDownstream(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"not found"}`))
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"concepts":  []interface{}{},
		"relations": []interface{}{},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}

	var errResp map[string]string
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// ---------------------------------------------------------------------------
// 500 from downstream → handler returns 500
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_500FromDownstream(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"db failure"}`))
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"concepts":  []interface{}{},
		"relations": []interface{}{},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Invalid JSON body → 400
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_InvalidJSON(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, _ *http.Request) {
		// Should never be reached.
		t.Error("downstream should not be called for invalid JSON")
		w.WriteHeader(http.StatusOK)
	})
	defer svc.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/internal/workspaces/ws-1/multi-agentic-systems/mas-1/graph/update", bytes.NewBufferString("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", "ws-1")
	req.SetPathValue("masId", "mas-1")

	rr := httptest.NewRecorder()
	app.updateGraphHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Path params override header workspace/mas values
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_PathParamsTakePrecedence(t *testing.T) {
	var capturedBody map[string]interface{}

	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"header": map[string]string{
			// These values differ from the path params — path params win.
			"workspace_id": "wrong-ws",
			"mas_id":       "wrong-mas",
		},
		"concepts":  []interface{}{},
		"relations": []interface{}{},
	}

	rr := doUpdateGraph(t, app, "ws-correct", "mas-correct", reqBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if capturedBody["mas_id"] != "mas-correct" {
		t.Errorf("expected mas_id 'mas-correct' forwarded, got %v", capturedBody["mas_id"])
	}
	if capturedBody["wksp_id"] != "ws-correct" {
		t.Errorf("expected wksp_id 'ws-correct' forwarded, got %v", capturedBody["wksp_id"])
	}
}

// ---------------------------------------------------------------------------
// Metadata and descriptor are echoed back in response
// ---------------------------------------------------------------------------

func TestUpdateGraphHandler_MetadataEchoed(t *testing.T) {
	app, svc := newUpdateGraphApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
	})
	defer svc.Close()

	reqBody := map[string]interface{}{
		"concepts":   []interface{}{},
		"relations":  []interface{}{},
		"descriptor": "batch-42",
		"metadata": map[string]interface{}{
			"concepts_updated": float64(7),
			"CoDiN_generated":  float64(3),
		},
	}

	rr := doUpdateGraph(t, app, "ws-1", "mas-1", reqBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp updateGraphResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Descriptor != "batch-42" {
		t.Errorf("expected descriptor 'batch-42', got %q", resp.Descriptor)
	}
	if resp.Metadata["concepts_updated"] != float64(7) {
		t.Errorf("expected metadata.concepts_updated=7, got %v", resp.Metadata["concepts_updated"])
	}
}

// ---------------------------------------------------------------------------
// extractEmbeddingFromAttributes unit tests
// ---------------------------------------------------------------------------

func TestExtractEmbeddingFromAttributes_NilAttrs(t *testing.T) {
	attrs, emb := extractEmbeddingFromAttributes(nil)
	if attrs != nil {
		t.Errorf("expected nil attrs, got %v", attrs)
	}
	if emb != nil {
		t.Errorf("expected nil embedding, got %v", emb)
	}
}

func TestExtractEmbeddingFromAttributes_NoEmbeddingKey(t *testing.T) {
	input := map[string]interface{}{"category": "Technology", "founded_year": 1956}
	attrs, emb := extractEmbeddingFromAttributes(input)
	if emb != nil {
		t.Errorf("expected nil embedding, got %v", emb)
	}
	if attrs["category"] != "Technology" || attrs["founded_year"] != 1956 {
		t.Errorf("attributes should be unchanged, got %v", attrs)
	}
}

func TestExtractEmbeddingFromAttributes_Float64Slice(t *testing.T) {
	input := map[string]interface{}{
		"category":  "Technology",
		"embedding": []float64{0.1, 0.2, 0.3},
	}
	attrs, emb := extractEmbeddingFromAttributes(input)
	if emb == nil {
		t.Fatal("expected non-nil embedding")
	}
	if len(emb.Data) != 3 || emb.Data[0] != 0.1 {
		t.Errorf("unexpected embedding data: %v", emb.Data)
	}
	if _, ok := attrs["embedding"]; ok {
		t.Error("embedding key should be removed from attributes")
	}
	if attrs["category"] != "Technology" {
		t.Errorf("other attributes should be preserved, got %v", attrs)
	}
}

func TestExtractEmbeddingFromAttributes_InterfaceSlice(t *testing.T) {
	// Simulates JSON-unmarshaled []interface{} from map[string]interface{}
	input := map[string]interface{}{
		"embedding": []interface{}{float64(0.042), float64(0.018), float64(-0.079)},
	}
	attrs, emb := extractEmbeddingFromAttributes(input)
	if emb == nil {
		t.Fatal("expected non-nil embedding")
	}
	if len(emb.Data) != 3 || emb.Data[2] != -0.079 {
		t.Errorf("unexpected embedding data: %v", emb.Data)
	}
	if _, ok := attrs["embedding"]; ok {
		t.Error("embedding key should be removed from attributes")
	}
}

func TestExtractEmbeddingFromAttributes_EmptyVector(t *testing.T) {
	input := map[string]interface{}{
		"embedding": []float64{},
	}
	attrs, emb := extractEmbeddingFromAttributes(input)
	if emb != nil {
		t.Errorf("expected nil embedding for empty vector, got %v", emb)
	}
	// original attrs returned unchanged when vector is empty
	if _, ok := attrs["embedding"]; !ok {
		t.Error("embedding key should remain when vector is empty")
	}
}

func TestExtractEmbeddingFromAttributes_WrongType(t *testing.T) {
	input := map[string]interface{}{
		"embedding": "not-a-vector",
	}
	attrs, emb := extractEmbeddingFromAttributes(input)
	if emb != nil {
		t.Errorf("expected nil embedding for wrong type, got %v", emb)
	}
	// original attrs returned unchanged for unrecognised type
	if attrs["embedding"] != "not-a-vector" {
		t.Errorf("embedding key should remain for unrecognised type, got %v", attrs)
	}
}
