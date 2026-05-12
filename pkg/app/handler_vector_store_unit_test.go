package app

import (
	"encoding/json"
	"testing"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// AgentVectorUpsertRequest DTO parsing
// ---------------------------------------------------------------------------

func TestAgentVectorUpsertRequest_DecodesRecords(t *testing.T) {
	raw := `{
		"request_id": "req-1",
		"records": [
			{
				"id": "vec-1",
				"content": "some text",
				"embedding": {"data": [0.1, 0.2, 0.3]},
				"metadata": {"doc_index": 0, "chunk_index": 1}
			}
		]
	}`

	var req sharedmemory.AgentVectorUpsertRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if req.RequestID == nil || *req.RequestID != "req-1" {
		t.Errorf("expected request_id 'req-1', got %v", req.RequestID)
	}
	if len(req.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(req.Records))
	}

	rec := req.Records[0]
	if rec.ID != "vec-1" {
		t.Errorf("expected id 'vec-1', got %q", rec.ID)
	}
	if rec.Content != "some text" {
		t.Errorf("expected content 'some text', got %q", rec.Content)
	}
	if len(rec.Embedding.Data) != 3 {
		t.Errorf("expected 3 embedding floats, got %d", len(rec.Embedding.Data))
	}
	if rec.Metadata["doc_index"] == nil {
		t.Errorf("expected doc_index in metadata")
	}
}

func TestAgentVectorUpsertRequest_OmittedIDIsEmptyString(t *testing.T) {
	raw := `{
		"records": [
			{
				"content": "no id provided",
				"embedding": {"data": [0.1]}
			}
		]
	}`

	var req sharedmemory.AgentVectorUpsertRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(req.Records) != 1 {
		t.Fatalf("expected 1 record")
	}
	if req.Records[0].ID != "" {
		t.Errorf("expected empty ID when omitted in JSON, got %q", req.Records[0].ID)
	}
}

func TestAgentVectorUpsertRequest_OmittedRequestIDIsNil(t *testing.T) {
	raw := `{"records": [{"content": "x", "embedding": {"data": [0.1]}}]}`

	var req sharedmemory.AgentVectorUpsertRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.RequestID != nil {
		t.Errorf("expected nil RequestID when omitted, got %v", req.RequestID)
	}
}

// ---------------------------------------------------------------------------
// AgentVectorDeleteRequest DTO parsing
// ---------------------------------------------------------------------------

func TestAgentVectorDeleteRequest_DecodesFields(t *testing.T) {
	softDel := false
	raw := `{"request_id": "req-del", "id": "vec-42", "soft_delete": false}`

	var req sharedmemory.AgentVectorDeleteRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if req.RequestID == nil || *req.RequestID != "req-del" {
		t.Errorf("expected request_id 'req-del', got %v", req.RequestID)
	}
	if req.ID != "vec-42" {
		t.Errorf("expected id 'vec-42', got %q", req.ID)
	}
	if req.SoftDelete == nil || *req.SoftDelete != softDel {
		t.Errorf("expected soft_delete=false, got %v", req.SoftDelete)
	}
}

func TestAgentVectorDeleteRequest_OmittedSoftDeleteIsNil(t *testing.T) {
	raw := `{"id": "vec-1"}`

	var req sharedmemory.AgentVectorDeleteRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.SoftDelete != nil {
		t.Errorf("expected nil SoftDelete when omitted, got %v", req.SoftDelete)
	}
}

// ---------------------------------------------------------------------------
// Record ID auto-generation logic (mirrors handler behaviour)
// ---------------------------------------------------------------------------

// buildUpsertRecords mirrors the record-building logic in agentVectorUpsertHandler
// so we can test the ID generation in isolation.
func buildUpsertRecords(reqRecords []sharedmemory.AgentVectorUpsertRecord) []iocmemoryprovider.KnowledgeVectorStoreRequestRecord {
	records := make([]iocmemoryprovider.KnowledgeVectorStoreRequestRecord, 0, len(reqRecords))
	for _, rec := range reqRecords {
		id := rec.ID
		if id == "" {
			id = uuid.New().String()
		}
		records = append(records, iocmemoryprovider.KnowledgeVectorStoreRequestRecord{
			ID:        id,
			Content:   rec.Content,
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{Data: rec.Embedding.Data},
			Metadata:  rec.Metadata,
		})
	}
	return records
}

func TestBuildUpsertRecords_PreservesExplicitID(t *testing.T) {
	records := buildUpsertRecords([]sharedmemory.AgentVectorUpsertRecord{
		{ID: "my-id", Content: "text", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
	})

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID != "my-id" {
		t.Errorf("expected ID 'my-id', got %q", records[0].ID)
	}
}

func TestBuildUpsertRecords_GeneratesIDWhenOmitted(t *testing.T) {
	records := buildUpsertRecords([]sharedmemory.AgentVectorUpsertRecord{
		{Content: "no id", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
	})

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID == "" {
		t.Error("expected non-empty auto-generated ID")
	}
	if _, err := uuid.Parse(records[0].ID); err != nil {
		t.Errorf("auto-generated ID is not a valid UUID: %q", records[0].ID)
	}
}

func TestBuildUpsertRecords_EachOmittedIDIsUnique(t *testing.T) {
	records := buildUpsertRecords([]sharedmemory.AgentVectorUpsertRecord{
		{Content: "first", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.1}}},
		{Content: "second", Embedding: sharedmemory.VectorEmbeddingPayload{Data: []float64{0.2}}},
	})

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].ID == records[1].ID {
		t.Errorf("auto-generated IDs should be unique, both got %q", records[0].ID)
	}
}

func TestBuildUpsertRecords_MapsEmbeddingAndMetadata(t *testing.T) {
	meta := map[string]interface{}{"doc_index": float64(3)}
	embedding := []float64{1.0, 2.0, 3.0}

	records := buildUpsertRecords([]sharedmemory.AgentVectorUpsertRecord{
		{
			ID:        "rec-1",
			Content:   "doc text",
			Embedding: sharedmemory.VectorEmbeddingPayload{Data: embedding},
			Metadata:  meta,
		},
	})

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Content != "doc text" {
		t.Errorf("expected content 'doc text', got %q", r.Content)
	}
	if r.Embedding == nil || len(r.Embedding.Data) != 3 {
		t.Errorf("expected embedding with 3 floats")
	}
	for i, v := range embedding {
		if r.Embedding.Data[i] != v {
			t.Errorf("embedding[%d]: want %f got %f", i, v, r.Embedding.Data[i])
		}
	}
	if r.Metadata["doc_index"] != float64(3) {
		t.Errorf("expected doc_index 3, got %v", r.Metadata["doc_index"])
	}
}

// ---------------------------------------------------------------------------
// KnowledgeVectorStoreRequest AgentID propagation
// ---------------------------------------------------------------------------

func TestKnowledgeVectorStoreRequest_AgentIDPropagatesToRequest(t *testing.T) {
	records := []iocmemoryprovider.KnowledgeVectorStoreRequestRecord{
		{ID: "rec-1", Content: "text", Embedding: &iocmemoryprovider.VectorEmbeddingConfig{Data: []float64{0.1}}},
	}
	agentID := "agent-xyz"

	req := iocmemoryprovider.NewKnowledgeVectorStoreRequest("wksp-1", "mas-1", records)
	req.AgentID = &agentID

	if req.AgentID == nil || *req.AgentID != agentID {
		t.Errorf("expected AgentID %q, got %v", agentID, req.AgentID)
	}
	if req.WkspID != "wksp-1" {
		t.Errorf("expected WkspID 'wksp-1', got %q", req.WkspID)
	}
	if req.MasID != "mas-1" {
		t.Errorf("expected MasID 'mas-1', got %q", req.MasID)
	}
}

func TestKnowledgeVectorStoreRequest_AgentIDIsNilByDefault(t *testing.T) {
	records := []iocmemoryprovider.KnowledgeVectorStoreRequestRecord{}
	req := iocmemoryprovider.NewKnowledgeVectorStoreRequest("wksp-1", "mas-1", records)

	if req.AgentID != nil {
		t.Errorf("expected nil AgentID by default, got %v", req.AgentID)
	}
}

func TestKnowledgeVectorDeleteRequest_AgentIDPropagatesToRequest(t *testing.T) {
	agentID := "agent-xyz"

	req := iocmemoryprovider.NewKnowledgeVectorDeleteRequest("wksp-1", "mas-1", "vec-1", true)
	req.AgentID = &agentID

	if req.AgentID == nil || *req.AgentID != agentID {
		t.Errorf("expected AgentID %q, got %v", agentID, req.AgentID)
	}
	if req.ID != "vec-1" {
		t.Errorf("expected ID 'vec-1', got %q", req.ID)
	}
	if !req.SoftDelete {
		t.Error("expected SoftDelete=true")
	}
}

func TestKnowledgeVectorSimilaritySearchRequest_AgentIDPropagatesToRequest(t *testing.T) {
	agentID := "agent-xyz"
	req := &iocmemoryprovider.KnowledgeVectorSimilaritySearchRequest{
		RequestID: "req-1",
		WkspID:    "wksp-1",
		MasID:     "mas-1",
		AgentID:   &agentID,
		Embedding: []float64{0.1, 0.2},
		Limit:     5,
		Metric:    "cosine",
	}

	if req.AgentID == nil || *req.AgentID != agentID {
		t.Errorf("expected AgentID %q, got %v", agentID, req.AgentID)
	}
}

func TestKnowledgeVectorSimilaritySearchRequest_AgentIDOmittedInJSON(t *testing.T) {
	req := &iocmemoryprovider.KnowledgeVectorSimilaritySearchRequest{
		RequestID: "req-1",
		WkspID:    "wksp-1",
		MasID:     "mas-1",
		Embedding: []float64{0.1},
		Limit:     5,
		Metric:    "cosine",
		// AgentID intentionally nil
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := m["agent_id"]; ok {
		t.Error("agent_id should be omitted from JSON when nil")
	}
}
