package sharedmemory

import (
	"fmt"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

type Header struct {
	// ID that represents the agent, optional for query operations
	AgentID *string `json:"agent_id,omitempty"`
}

type OnboardVectorStoreRequest struct {
	// Header(s) of the request, optional.
	Header *Header `json:"header,omitempty"`
	// ID of the request, optional.
	// If not provided, a random UUID is used to represent the request.
	RequestId *string `json:"request_id,omitempty"`
}

type OnboardVectorStoreResponse struct {
	ResponseID *string `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string  `json:"status" description:"Status of the request"`
	Message    *string `json:"message,omitempty" description:"Optional message providing additional information"`
	StoreId    *string `json:"store_id,omitempty" description:"ID of the vector store"`
}

type DeleteVectorStoreRequest struct {
	// Header(s) of the request, optional.
	Header *Header `json:"header,omitempty"`
	// ID of the request, optional.
	// If not provided, a random UUID is used to represent the request.
	RequestId *string `json:"request_id,omitempty"`
}

type DeleteVectorStoreResponse struct {
	ResponseID *string `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string  `json:"status" description:"Status of the request"`
	Message    *string `json:"message,omitempty" description:"Optional message providing additional information"`
	StoreId    *string `json:"store_id,omitempty" description:"ID of the vector store"`
}

type CreateOrUpdateRequest struct {
	// Header(s) of the request, optional
	Header *Header `json:"header,omitempty"`
	// ID of the request, optional. If not provided, a random UUID is used to represent the request
	RequestId *string `json:"request_id,omitempty"`
	// Payload contains the extraction metadata and the raw data to be processed. The structure of the payload data is defined by Payload.Metadata.Format
	Payload cognitionagentclient.ExtractionPayload `json:"payload"`
}

type CreateOrUpdateResponse struct {
	// ID of the response, this gets populated from request_id
	ResponseID *string `json:"response_id,omitempty"`
	// Status of the request
	Status string `json:"status"`
	// Optional message from the graph store upsert operation
	GraphStoreMessage *string `json:"graph_store_message,omitempty"`
	// Optional message from the vector store upsert operation
	VectorStoreMessage *string `json:"vector_store_message,omitempty"`
	// Meta provides LLM token usage and performance metrics (optional, present when LLM calls are made)
	Meta *common.TokenUsageMeta `json:"meta,omitempty"`
}

type QueryRequest struct {
	// Header(s) of the request, required (must include agent_id)
	Header *Header `json:"header"`
	// ID of the request, optional. If not provided, a random UUID is used to represent the request
	RequestId *string `json:"request_id,omitempty"`
	// Search strategy to be used when executing the query. Currently supported values: "semantic_graph_traversal". If not specified, defaults to "semantic_graph_traversal"
	SearchStrategy *string `json:"search_strategy,omitempty"`
	// User intent or natural-language query describing what information is being requested. This field is required and is the primary signal used to construct and execute the query
	Intent *string `json:"intent"`
	// AdditionalContext provides optional contextual information to refine query execution. This may include prior conversation state, structured hints, or domain-specific metadata. The contents are treated as opaque by the API and interpreted by downstream components. Each element must be a structured object
	AdditionalContext []map[string]interface{} `json:"additional_context,omitempty"`
}

const (
	SearchStrategySemanticGraphTraversal = "semantic_graph_traversal"
)

// SearchStrategyConvertMap Reasoning service is using ""Semantic Graph Traversal" for its validation, hence we need a conversion here
var SearchStrategyConvertMap = map[string]string{
	SearchStrategySemanticGraphTraversal: "Semantic Graph Traversal",
}

func (r *QueryRequest) ValidateAndApplyDefault() error {
	// Validate required fields
	if r.Intent == nil || *r.Intent == "" {
		return fmt.Errorf("intent is required")
	}

	// Apply defaults
	if r.SearchStrategy == nil {
		r.SearchStrategy = common.StrToPtr(SearchStrategySemanticGraphTraversal)
	}

	if r.SearchStrategy != nil && *r.SearchStrategy != SearchStrategySemanticGraphTraversal {
		return fmt.Errorf("invalid search_strategy, valid value is %s", SearchStrategySemanticGraphTraversal)
	}

	return nil
}

type QueryResponse struct {
	// ID of the response, this gets populated from request_id
	ResponseID *string `json:"response_id,omitempty"`
	// Message provides detailed information from the query result
	Message *string `json:"message"`
	// Meta provides LLM token usage and performance metrics (optional, present when LLM calls are made)
	Meta *common.TokenUsageMeta `json:"meta,omitempty"`
}

type QueryResponseRecord struct {
	Relationships []QueryRelation `json:"relationships,omitempty"`
	Concepts      []QueryConcept  `json:"concepts,omitempty"`
}

type QueryConcept struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

type QueryRelation struct {
	ID         string                 `json:"id"`
	Relation   string                 `json:"relation"`
	NodeIDs    []string               `json:"node_ids"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// VectorSimilaritySearchPayload holds the search parameters nested under "payload"
type VectorSimilaritySearchPayload struct {
	EmbeddedText    string                                           `json:"embedded_text,omitempty"`
	EmbeddingVector []float64                                        `json:"embedding_vector"`
	Filters         *iocmemoryprovider.KnowledgeVectorMetadataFilter `json:"filters,omitempty"`
	TopK            *int                                             `json:"top_k,omitempty"`
	Metric          *string                                          `json:"search_metrics,omitempty"`
}

// VectorSimilaritySearchRequest is the request body for vector similarity search
type VectorSimilaritySearchRequest struct {
	Header    *Header                       `json:"header,omitempty"`
	RequestId *string                       `json:"request_id,omitempty"`
	Payload   VectorSimilaritySearchPayload `json:"payload"`
}

// VectorSimilaritySearchResult is a single result from a vector similarity search
type VectorSimilaritySearchResult struct {
	Score           float64   `json:"score"`
	EmbeddedText    string    `json:"embedded_text"`
	Timestamp       string    `json:"timestamp,omitempty"`
	DocIndex        int       `json:"doc_index"`
	ChunkIndex      int       `json:"chunk_index"`
	Domain          string    `json:"domain,omitempty"`
	EmbeddingVector []float64 `json:"embedding_vector,omitempty"`
}

// VectorSimilaritySearchResponse is the response for vector similarity search
type VectorSimilaritySearchResponse struct {
	Header    *Header                        `json:"header,omitempty"`
	RequestId *string                        `json:"request_id,omitempty"`
	Results   []VectorSimilaritySearchResult `json:"results,omitempty"`
}

// AgentVectorUpsertRecord is a single record in an agent vector upsert request.
// ID is optional — if omitted the server generates a UUID, enabling idempotent re-ingestion when provided.
type AgentVectorUpsertRecord struct {
	ID        string                 `json:"id,omitempty"`
	Content   string                 `json:"content"`
	Embedding VectorEmbeddingPayload `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// VectorEmbeddingPayload wraps the raw float64 embedding data.
type VectorEmbeddingPayload struct {
	Data []float64 `json:"data"`
}

// AgentVectorUpsertRequest is the request body for POST .../agents/{agentId}/rag/vectors
type AgentVectorUpsertRequest struct {
	RequestID *string                   `json:"request_id,omitempty"`
	Records   []AgentVectorUpsertRecord `json:"records"`
}

// VectorDeleteMetadataFilter represents metadata filters for bulk delete operations
type VectorDeleteMetadataFilter struct {
	DocIndex       *int              `json:"doc_index,omitempty"`
	ChunkIndex     *int              `json:"chunk_index,omitempty"`
	DataSource     *string           `json:"data_source,omitempty"`
	RecordedAtFrom *string           `json:"recorded_at_from,omitempty"`
	RecordedAtTo   *string           `json:"recorded_at_to,omitempty"`
	ExtraFilters   map[string]string `json:"extra_filters,omitempty"`
}

// AgentVectorDeleteRequest is the request body for DELETE .../agents/{agentId}/rag/vectors
// Supports two modes: delete by ID (single) or delete by filters (bulk)
type AgentVectorDeleteRequest struct {
	RequestID *string                     `json:"request_id,omitempty"`
	ID        *string                     `json:"id,omitempty"`
	Filters   *VectorDeleteMetadataFilter `json:"filters,omitempty"`
}

// NeighborsResponse is the response for GET /graph/neighbors/{conceptId}
type NeighborsResponse struct {
	Records []QueryResponseRecord `json:"records,omitempty"`
}

// ConceptsByIdsRequest is the request body for POST /graph/concepts/by_ids
type ConceptsByIdsRequest struct {
	IDs []string `json:"ids"`
}

// GraphConcept is a concept in a graph response (richer than QueryConcept)
type GraphConcept struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// ConceptsByIdsResponse is the response for POST /graph/concepts/by_ids
type ConceptsByIdsResponse struct {
	Concepts []GraphConcept `json:"concepts,omitempty"`
}

// GraphPathsRequest is the request body for POST /graph/paths
type GraphPathsRequest struct {
	SourceID  string   `json:"source_id"`
	TargetID  string   `json:"target_id"`
	MaxDepth  *int     `json:"max_depth,omitempty"`
	Relations []string `json:"relations,omitempty"`
	Limit     *int     `json:"limit,omitempty"`
}

// PathEdge represents a single directed edge in a path
type PathEdge struct {
	FromID   string `json:"from_id"`
	Relation string `json:"relation"`
	ToID     string `json:"to_id"`
	FromName string `json:"from_name,omitempty"`
	ToName   string `json:"to_name,omitempty"`
}

// Path represents an ordered path through the knowledge graph
type Path struct {
	NodeIDs    []string   `json:"node_ids,omitempty"`
	Edges      []PathEdge `json:"edges,omitempty"`
	PathLength int        `json:"path_length"`
	Symbolic   string     `json:"symbolic,omitempty"`
}

// GraphPathsResponse is the response for POST /graph/paths
type GraphPathsResponse struct {
	Status string `json:"status"`
	Paths  []Path `json:"paths,omitempty"`
}
