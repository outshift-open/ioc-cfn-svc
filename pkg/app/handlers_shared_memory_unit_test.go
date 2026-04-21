package app

import (
	"testing"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// transformRagChunksToVectorRecords
// ---------------------------------------------------------------------------

func TestTransformRagChunksToVectorRecords_Empty(t *testing.T) {
	result := transformRagChunksToVectorRecords("ws1", "mas1", nil)
	if result != nil {
		t.Errorf("expected nil for empty chunks, got %v", result)
	}

	result = transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{})
	if result != nil {
		t.Errorf("expected nil for empty chunks slice, got %v", result)
	}
}

func TestTransformRagChunksToVectorRecords_SkipsChunksWithNoEmbedding(t *testing.T) {
	chunks := []cognitionagentclient.RagChunk{
		{
			Text:      "some text",
			Metadata:  cognitionagentclient.RagChunkMetadata{Domain: "test", DocIndex: 0, ChunkIndex: 0},
			Embedding: nil, // no embedding
		},
		{
			Text:      "other text",
			Metadata:  cognitionagentclient.RagChunkMetadata{Domain: "test", DocIndex: 0, ChunkIndex: 1},
			Embedding: [][]float64{}, // empty outer slice
		},
		{
			Text:      "third text",
			Metadata:  cognitionagentclient.RagChunkMetadata{Domain: "test", DocIndex: 0, ChunkIndex: 2},
			Embedding: [][]float64{{}}, // empty inner slice
		},
	}

	result := transformRagChunksToVectorRecords("ws1", "mas1", chunks)
	if len(result) != 0 {
		t.Errorf("expected 0 records for chunks without valid embeddings, got %d", len(result))
	}
}

func TestTransformRagChunksToVectorRecords_DeterministicIDs(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	chunks := []cognitionagentclient.RagChunk{
		{
			Text: "hello world",
			Metadata: cognitionagentclient.RagChunkMetadata{
				Domain:     "test-domain",
				Timestamp:  "2026-01-01T00:00:00Z",
				DocIndex:   1,
				ChunkIndex: 0,
			},
			Embedding: [][]float64{embedding},
		},
	}

	result1 := transformRagChunksToVectorRecords("ws1", "mas1", chunks)
	result2 := transformRagChunksToVectorRecords("ws1", "mas1", chunks)

	if len(result1) != 1 || len(result2) != 1 {
		t.Fatalf("expected 1 record each, got %d and %d", len(result1), len(result2))
	}

	if result1[0].ID != result2[0].ID {
		t.Errorf("IDs should be deterministic: got %s and %s", result1[0].ID, result2[0].ID)
	}

	if _, err := uuid.Parse(result1[0].ID); err != nil {
		t.Errorf("ID is not a valid UUID: %s", result1[0].ID)
	}
}

func TestTransformRagChunksToVectorRecords_DifferentInputsDifferentIDs(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	makeChunk := func(domain string, docIdx, chunkIdx int) cognitionagentclient.RagChunk {
		return cognitionagentclient.RagChunk{
			Text: "same text",
			Metadata: cognitionagentclient.RagChunkMetadata{
				Domain:     domain,
				DocIndex:   docIdx,
				ChunkIndex: chunkIdx,
			},
			Embedding: [][]float64{embedding},
		}
	}

	tests := []struct {
		name   string
		a, b   cognitionagentclient.RagChunk
		sameID bool
	}{
		{
			name:   "different doc_index → different ID",
			a:      makeChunk("domain", 1, 0),
			b:      makeChunk("domain", 2, 0),
			sameID: false,
		},
		{
			name:   "different chunk_index → different ID",
			a:      makeChunk("domain", 1, 0),
			b:      makeChunk("domain", 1, 1),
			sameID: false,
		},
		{
			name:   "different domain → different ID",
			a:      makeChunk("domain-a", 1, 0),
			b:      makeChunk("domain-b", 1, 0),
			sameID: false,
		},
		{
			name:   "same inputs → same ID",
			a:      makeChunk("domain", 1, 0),
			b:      makeChunk("domain", 1, 0),
			sameID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ra := transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{tt.a})
			rb := transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{tt.b})

			if len(ra) != 1 || len(rb) != 1 {
				t.Fatalf("expected 1 record each")
			}

			if tt.sameID && ra[0].ID != rb[0].ID {
				t.Errorf("expected same ID, got %s vs %s", ra[0].ID, rb[0].ID)
			}
			if !tt.sameID && ra[0].ID == rb[0].ID {
				t.Errorf("expected different IDs, both got %s", ra[0].ID)
			}
		})
	}
}

func TestTransformRagChunksToVectorRecords_DifferentWorkspaceOrMasDifferentID(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	chunk := cognitionagentclient.RagChunk{
		Text: "text",
		Metadata: cognitionagentclient.RagChunkMetadata{
			Domain: "domain", DocIndex: 0, ChunkIndex: 0,
		},
		Embedding: [][]float64{embedding},
	}

	rWs1 := transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{chunk})
	rWs2 := transformRagChunksToVectorRecords("ws2", "mas1", []cognitionagentclient.RagChunk{chunk})
	rMas2 := transformRagChunksToVectorRecords("ws1", "mas2", []cognitionagentclient.RagChunk{chunk})

	if rWs1[0].ID == rWs2[0].ID {
		t.Errorf("different workspaces should produce different IDs")
	}
	if rWs1[0].ID == rMas2[0].ID {
		t.Errorf("different MAS IDs should produce different IDs")
	}
}

func TestTransformRagChunksToVectorRecords_MetadataAndContent(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	chunk := cognitionagentclient.RagChunk{
		Text: "chunk text",
		Metadata: cognitionagentclient.RagChunkMetadata{
			Domain:     "my-domain",
			Timestamp:  "2026-01-15T10:00:00Z",
			DocIndex:   3,
			ChunkIndex: 7,
		},
		Embedding: [][]float64{embedding},
	}

	result := transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{chunk})
	if len(result) != 1 {
		t.Fatalf("expected 1 record, got %d", len(result))
	}

	r := result[0]

	if r.Content != "chunk text" {
		t.Errorf("expected content 'chunk text', got %q", r.Content)
	}

	if r.Embedding == nil || len(r.Embedding.Data) != 3 {
		t.Errorf("expected embedding with 3 floats")
	}

	// Verify metadata fields
	if r.Metadata["data_source"] != "my-domain" {
		t.Errorf("expected data_source 'my-domain', got %v", r.Metadata["data_source"])
	}
	if r.Metadata["recorded_at"] != "2026-01-15T10:00:00Z" {
		t.Errorf("expected recorded_at '2026-01-15T10:00:00Z', got %v", r.Metadata["recorded_at"])
	}
	if r.Metadata["doc_index"] != 3 {
		t.Errorf("expected doc_index 3, got %v", r.Metadata["doc_index"])
	}
	if r.Metadata["chunk_index"] != 7 {
		t.Errorf("expected chunk_index 7, got %v", r.Metadata["chunk_index"])
	}
}

func TestTransformRagChunksToVectorRecords_UsesFirstEmbeddingVector(t *testing.T) {
	first := []float64{1.0, 2.0, 3.0}
	second := []float64{9.0, 8.0, 7.0}
	chunk := cognitionagentclient.RagChunk{
		Text:      "text",
		Metadata:  cognitionagentclient.RagChunkMetadata{Domain: "d", DocIndex: 0, ChunkIndex: 0},
		Embedding: [][]float64{first, second},
	}

	result := transformRagChunksToVectorRecords("ws1", "mas1", []cognitionagentclient.RagChunk{chunk})
	if len(result) != 1 {
		t.Fatalf("expected 1 record")
	}

	for i, v := range first {
		if result[0].Embedding.Data[i] != v {
			t.Errorf("expected first embedding vector to be used, index %d: want %f got %f", i, v, result[0].Embedding.Data[i])
		}
	}
}

// ---------------------------------------------------------------------------
// transformExtractionConcepts
// ---------------------------------------------------------------------------

func TestTransformExtractionConcepts_Empty(t *testing.T) {
	result := transformExtractionConcepts(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = transformExtractionConcepts([]cognitionagentclient.Concept{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

func TestTransformExtractionConcepts_MapsFields(t *testing.T) {
	src := []cognitionagentclient.Concept{
		{
			ID:          "concept-1",
			Name:        "TestConcept",
			Description: "a test concept",
			Type:        "agent",
			Attributes: cognitionagentclient.ConceptAttributes{
				ConceptType: "service",
				Embedding:   [][]float64{{0.1, 0.2}},
			},
		},
	}

	result := transformExtractionConcepts(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(result))
	}

	c := result[0]
	if c.ID != "concept-1" {
		t.Errorf("expected ID 'concept-1', got %q", c.ID)
	}
	if c.Name != "TestConcept" {
		t.Errorf("expected Name 'TestConcept', got %q", c.Name)
	}
	if c.Description == nil || *c.Description != "a test concept" {
		t.Errorf("expected Description 'a test concept', got %v", c.Description)
	}
	if c.Attributes["concept_type"] != "service" {
		t.Errorf("expected concept_type 'service', got %v", c.Attributes["concept_type"])
	}
	if c.Embeddings == nil {
		t.Error("expected embeddings to be non-nil")
	}
}

func TestTransformExtractionConcepts_EmptyDescriptionBecomesNil(t *testing.T) {
	src := []cognitionagentclient.Concept{
		{
			ID:          "concept-2",
			Name:        "NilDesc",
			Description: "", // empty → nil pointer
		},
	}

	result := transformExtractionConcepts(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(result))
	}
	if result[0].Description != nil {
		t.Errorf("expected nil Description for empty string, got %v", result[0].Description)
	}
}

// ---------------------------------------------------------------------------
// transformExtractionRelations
// ---------------------------------------------------------------------------

func TestTransformExtractionRelations_Empty(t *testing.T) {
	result := transformExtractionRelations(nil)
	if result != nil {
		t.Errorf("expected nil for nil input")
	}
}

func TestTransformExtractionRelations_MapsFields(t *testing.T) {
	src := []cognitionagentclient.Relation{
		{
			ID:           "rel-1",
			Relationship: "calls",
			NodeIDs:      []string{"a", "b"},
			Attributes:   map[string]interface{}{"weight": 1.0},
		},
	}

	result := transformExtractionRelations(src)
	if len(result) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(result))
	}

	r := result[0]
	if r.ID != "rel-1" {
		t.Errorf("expected ID 'rel-1', got %q", r.ID)
	}
	if r.Relation != "calls" {
		t.Errorf("expected Relation 'calls', got %q", r.Relation)
	}
	if len(r.NodeIDs) != 2 {
		t.Errorf("expected 2 NodeIDs, got %d", len(r.NodeIDs))
	}
}

// ---------------------------------------------------------------------------
// TransformExtractionResponseToRecords
// ---------------------------------------------------------------------------

func TestTransformExtractionResponseToRecords_NilResponse(t *testing.T) {
	result := TransformExtractionResponseToRecords(nil)
	if result != nil {
		t.Errorf("expected nil for nil response")
	}
}

func TestTransformExtractionResponseToRecords_ProducesRecords(t *testing.T) {
	resp := &cognitionagentclient.KnowledgeCognitionResponse{
		Concepts: []cognitionagentclient.Concept{
			{ID: "c1", Name: "Concept1"},
		},
		Relations: []cognitionagentclient.Relation{
			{ID: "r1", Relationship: "rel", NodeIDs: []string{"c1", "c2"}},
		},
	}

	result := TransformExtractionResponseToRecords(resp)
	if result == nil {
		t.Fatal("expected non-nil records")
	}
	if len(result.Concepts) != 1 {
		t.Errorf("expected 1 concept, got %d", len(result.Concepts))
	}
	if len(result.Relations) != 1 {
		t.Errorf("expected 1 relation, got %d", len(result.Relations))
	}
}

// ---------------------------------------------------------------------------
// transformConceptEmbedding
// ---------------------------------------------------------------------------

func TestTransformConceptEmbedding_ReturnsNilWhenEmpty(t *testing.T) {
	attrs := cognitionagentclient.ConceptAttributes{Embedding: nil}
	if transformConceptEmbedding(attrs) != nil {
		t.Error("expected nil for nil embedding")
	}

	attrs.Embedding = [][]float64{}
	if transformConceptEmbedding(attrs) != nil {
		t.Error("expected nil for empty outer slice")
	}

	attrs.Embedding = [][]float64{{}}
	if transformConceptEmbedding(attrs) != nil {
		t.Error("expected nil for empty inner slice")
	}
}

func TestTransformConceptEmbedding_UsesFirstVector(t *testing.T) {
	first := []float64{1.0, 2.0}
	attrs := cognitionagentclient.ConceptAttributes{
		Embedding: [][]float64{first, {9.0, 8.0}},
	}

	result := transformConceptEmbedding(attrs)
	if result == nil {
		t.Fatal("expected non-nil embedding config")
	}
	for i, v := range first {
		if result.Data[i] != v {
			t.Errorf("index %d: want %f got %f", i, v, result.Data[i])
		}
	}
}

// ---------------------------------------------------------------------------
// mapKGRecordToQueryRecord
// ---------------------------------------------------------------------------

func TestMapKGRecordToQueryRecord_EmptyRecord(t *testing.T) {
	record := iocmemoryprovider.KnowledgeGraphQueryResponseRecord{}
	result := mapKGRecordToQueryRecord(record)
	if result.Concepts != nil {
		t.Errorf("expected nil concepts for empty record")
	}
	if result.Relationships != nil {
		t.Errorf("expected nil relationships for empty record")
	}
}

func TestMapKGRecordToQueryRecord_MapsConceptsAndRelations(t *testing.T) {
	desc := "a concept"
	record := iocmemoryprovider.KnowledgeGraphQueryResponseRecord{
		Concepts: []iocmemoryprovider.Concept{
			{
				ID:          "c1",
				Name:        "ConceptOne",
				Description: &desc,
				Tags:        []string{"tag1"},
			},
		},
		Relationships: []iocmemoryprovider.Relation{
			{
				ID:      "r1",
				Relation: "knows",
				NodeIDs: []string{"c1", "c2"},
			},
		},
	}

	result := mapKGRecordToQueryRecord(record)

	if len(result.Concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(result.Concepts))
	}
	if result.Concepts[0].ID != "c1" {
		t.Errorf("expected concept ID 'c1', got %q", result.Concepts[0].ID)
	}
	if result.Concepts[0].Description == nil || *result.Concepts[0].Description != desc {
		t.Errorf("expected description %q", desc)
	}
	if len(result.Concepts[0].Tags) != 1 || result.Concepts[0].Tags[0] != "tag1" {
		t.Errorf("expected tags [tag1], got %v", result.Concepts[0].Tags)
	}

	if len(result.Relationships) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(result.Relationships))
	}
	if result.Relationships[0].Relation != "knows" {
		t.Errorf("expected relation 'knows', got %q", result.Relationships[0].Relation)
	}
}
