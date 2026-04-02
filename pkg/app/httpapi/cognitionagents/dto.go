// Package cognitionagents provides DTOs for the cognition agents API handler.
//
// NOTE: Struct fields and JSON tags may change as the API evolves.
package cognitionagents

import (
	"encoding/json"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
)

//////////////////////////////////////////////////////////////////

// RequestType represents the type of memory operation
type RequestType string

// RequestType constants for memory operations
const (
	ReqTypeKnowledgeVectorsUpsert RequestType = "KNOWLEDGE_VECTORS_UPSERT"
	ReqTypeKnowledgeVectorsQuery  RequestType = "KNOWLEDGE_VECTORS_QUERY"
)

// SharedMemoryVectorsRequest represents a request for shared memory vector operations.
// The Body field contains the raw JSON payload as defined in:
// https://github.com/cisco-eti/ioc-cfn-svc/blob/main/pkg/providers/memory/ioc/schema.go
//
// Example JSON structure:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "request_type": "KNOWLEDGE_VECTORS_UPSERT" or "KNOWLEDGE_VECTORS_QUERY",
//	  "request_body": {
//	    // Raw JSON payload matching KnowledgeVectorStoreRequest or KnowledgeVectorQueryRequest
//	    // from iocmemoryprovider schema
//	  }
//	}
type SharedMemoryVectorsRequest struct {
	Header common.Header    `json:"header"`
	Type   RequestType      `json:"request_type"`
	Body   *json.RawMessage `json:"request_body"`
}

// SharedMemoryVectorsResponse represents a response from shared memory vector operations.
// The Results field contains the raw JSON response returned from iocmemoryprovider.
//
// Example JSON structure for success:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "results": {
//	    // Raw JSON response matching KnowledgeVectorStoreResponse or KnowledgeVectorQueryResponse
//	    // from iocmemoryprovider schema
//	  }
//	}
//
// Example JSON structure for error:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "error": {
//	    "message": "Error description",
//	    "detail": { "additional": "error details" }
//	  }
//	}
type SharedMemoryVectorsResponse struct {
	Header  common.Header       `json:"header"`
	Error   *common.ErrorDetail `json:"error,omitempty"`
	Results *json.RawMessage    `json:"results,omitempty"`
}

//////////////////////////////////////////////////////////////////

// MemoryCreateRequest is the request body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory.
// TODO: Define fields once the API contract is finalized.
type MemoryCreateRequest struct {
	Header common.Header `json:"header"`
}

// MemoryCreateResponse is the response body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory.
// TODO: Define fields once the API contract is finalized.
type MemoryCreateResponse struct {
	Header     common.Header       `json:"header"`
	ResponseID string              `json:"response_id"`
	Error      *common.ErrorDetail `json:"error,omitempty"`
}

// ConceptsSearchRequest is the request body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/concepts/search.
// TODO: Define additional fields once the API contract is finalized.
type ConceptsSearchRequest struct {
	Header common.Header `json:"header"`
}

// ConceptsSearchResponse is the response body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/concepts/search.
// TODO: Define fields once the response contract (TKFKnowledgeRecord) is finalized.
type ConceptsSearchResponse struct {
	Header     common.Header            `json:"header"`
	ResponseID string                   `json:"response_id"`
	Error      *common.ErrorDetail      `json:"error,omitempty"`
	Results    []map[string]interface{} `json:"results,omitempty"`
}

// PathsSearchPayload holds the payload fields for a paths search request.
type PathsSearchPayload struct {
	FromID    string   `json:"from_id"`
	ToID      string   `json:"to_id"`
	MaxDepth  int      `json:"max_depth"`
	Relations []string `json:"relations,omitempty"`
}

// PathsSearchRequest is the request body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/paths/search.
type PathsSearchRequest struct {
	Header  common.Header      `json:"header"`
	Payload PathsSearchPayload `json:"payload"`
}

// PathEdge represents a single edge in a discovered path.
type PathEdge struct {
	FromID   string  `json:"from_id"`
	Relation string  `json:"relation"`
	ToID     string  `json:"to_id"`
	FromName *string `json:"from_name"`
	ToName   *string `json:"to_name"`
}

// PathResult represents a single path between two nodes.
type PathResult struct {
	NodeIDs    []string   `json:"node_ids"`
	Edges      []PathEdge `json:"edges"`
	PathLength int        `json:"path_length"`
	Symbolic   string     `json:"symbolic"`
}

// PathsSearchResponse is the response body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/paths/search.
type PathsSearchResponse struct {
	Header     common.Header       `json:"header"`
	ResponseID string              `json:"response_id"`
	Error      *common.ErrorDetail `json:"error,omitempty"`
	Paths      []PathResult        `json:"paths,omitempty"`
}

// MemorySearchRequest is the request body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/search.
type MemorySearchRequest struct {
	Header    common.Header `json:"header"`
	Queries   []string      `json:"queries"`
	Embedding []float64     `json:"embeding"` // Kept as "embeding" per API spec
	K         int           `json:"k"`
}

// MemorySearchResponse is the response body for POST /api/internal/cognition-fabric-node/{cfn-id}/memory/search.
type MemorySearchResponse struct {
	Header     common.Header       `json:"header"`
	ResponseID string              `json:"response_id"`
	Error      *common.ErrorDetail `json:"error,omitempty"`
	Results    []QueryResult       `json:"results,omitempty"`
}

// QueryResult holds the results for a single query.
type QueryResult struct {
	Query string                   `json:"query"`
	Hits  []map[string]interface{} `json:"hits"`
}
