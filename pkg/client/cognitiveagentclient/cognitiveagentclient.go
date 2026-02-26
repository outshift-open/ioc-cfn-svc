// Package cognitiveagentclient provides a client for the Cognitive Agents API.
//
// It supports the following endpoints:
//   - POST /api/knowledge-mgmt/extraction          — ingest agent telemetry data and extract knowledge.
//   - POST /api/knowledge-mgmt/reasoning/evidence   — reasoning evidence request with an intent query.
//   - POST /api/semantic-negotiation                 — semantic negotiation request.
//
// The client wraps httpclient.Client for retries and exponential backoff.
//
// NOTE: The Go struct fields / JSON tags in this package may change as the
// upstream Cognitive Agents API evolves. Update the structs and paths here
// when the API contract is modified.
//
// TODO: Add audit CRUD operations for cognitive agent API calls.
package cognitiveagentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
)

// ---------------------------------------------------------------------------
// Common types
// ---------------------------------------------------------------------------

// Header carries routing context for CFN requests and responses.
type Header struct {
	WorkspaceID string `json:"workspace_id"`       // Mandatory
	MASID       string `json:"mas_id"`             // Mandatory
	AgentID     string `json:"agent_id,omitempty"` // Optional
}

// ErrorDetail provides debugging information when an error occurs.
type ErrorDetail struct {
	Message string                 `json:"message"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// ExtractionPayloadMetadata describes the format and labels of the incoming data.
type ExtractionPayloadMetadata struct {
	Format string `json:"format"` // e.g. "observe-sdk-otel", "openclaw"
}

// ExtractionPayload holds the metadata and raw data array for an extraction request.
type ExtractionPayload struct {
	Metadata ExtractionPayloadMetadata `json:"metadata"`
	Data     []ExtractionDataRecord    `json:"data"`
}

// ExtractionDataRecord represents a single telemetry record (e.g. an OTel span).
type ExtractionDataRecord struct {
	TraceID        string            `json:"TraceId"`
	SpanID         string            `json:"SpanId"`
	ParentSpanID   string            `json:"ParentSpanId"`
	SpanName       string            `json:"SpanName"`
	ServiceName    string            `json:"ServiceName"`
	SpanAttributes map[string]string `json:"SpanAttributes"`
	Duration       int64             `json:"Duration"`
}

// ExtractionRequest is the request body for POST /api/knowledge-mgmt/extraction.
type ExtractionRequest struct {
	Header    Header            `json:"header"`
	RequestID string            `json:"request_id"`
	Payload   ExtractionPayload `json:"payload"`
}

// ReasoningEvidencePayloadMetadata describes the type of reasoning query.
type ReasoningEvidencePayloadMetadata struct {
	QueryType string `json:"query_type,omitempty"` // e.g. "Semantic Graph Traversal", "Vector Search"
}

// ReasoningEvidencePayload holds the intent and optional context for a reasoning evidence request.
type ReasoningEvidencePayload struct {
	Metadata          ReasoningEvidencePayloadMetadata `json:"metadata,omitempty"`
	Intent            string                           `json:"intent"`
	AdditionalContext []interface{}                    `json:"additional_context,omitempty"`
}

// ReasoningEvidenceRequest is the request body for POST /api/knowledge-mgmt/reasoning/evidence.
type ReasoningEvidenceRequest struct {
	Header    Header                   `json:"header"`
	RequestID string                   `json:"request_id,omitempty"`
	Payload   ReasoningEvidencePayload `json:"payload"`
}

// SemanticNegotiationPayload holds the payload for a semantic negotiation request.
// TODO: Define fields once the API contract is finalized.
type SemanticNegotiationPayload struct {
	// TBD
}

// SemanticNegotiationRequest is the request body for POST /api/semantic-negotiation.
type SemanticNegotiationRequest struct {
	Header    Header                     `json:"header"`
	RequestID string                     `json:"request_id,omitempty"`
	Payload   SemanticNegotiationPayload `json:"payload"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// KnowledgeCognitionResponse is the response from POST /api/knowledge-mgmt/extraction.
type KnowledgeCognitionResponse struct {
	Header     Header       `json:"header"`
	ResponseID string       `json:"response_id"`
	Error      *ErrorDetail `json:"error,omitempty"`
	Concepts   []Concept    `json:"concepts,omitempty"`
	Relations  []Relation   `json:"relations,omitempty"`
	Descriptor string       `json:"descriptor,omitempty"`
	Metadata   Meta         `json:"metadata,omitempty"`
}

// Concept represents an extracted concept from telemetry data.
type Concept struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// Relation represents a relationship between concepts.
type Relation struct {
	ID           string                 `json:"id"`
	NodeIDs      []string               `json:"node_ids"`
	Relationship string                 `json:"relationship"`
	Attributes   map[string]interface{} `json:"attributes"`
}

// ReasonerCognitionResponse is the response from POST /api/knowledge-mgmt/reasoning/evidence.
type ReasonerCognitionResponse struct {
	Header     Header                   `json:"header"`
	ResponseID string                   `json:"response_id"`
	Error      *ErrorDetail             `json:"error,omitempty"`
	Records    []map[string]interface{} `json:"records,omitempty"` // List of TKFKnowledgeRecord (TODO: define struct)
	Metadata   map[string]interface{}   `json:"metadata,omitempty"`
}

// SemanticNegotiationResponse is the response from POST /api/semantic-negotiation.
// TODO: Define additional fields once the API contract is finalized.
type SemanticNegotiationResponse struct {
	Header     Header       `json:"header"`
	ResponseID string       `json:"response_id"`
	Error      *ErrorDetail `json:"error,omitempty"`
}

// Meta contains metadata about the knowledge cognition processing.
type Meta struct {
	RecordsProcessed   int  `json:"records_processed"`
	ConceptsExtracted  int  `json:"concepts_extracted"`
	RelationsExtracted int  `json:"relations_extracted"`
	DedupEnabled       bool `json:"dedup_enabled"`
	ConceptsDeduped    int  `json:"concepts_deduped"`
	RelationsDeduped   int  `json:"relations_deduped"`
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is a client for the Cognitive Agents API.
// It wraps an httpclient.Client and a base URL for the target service.
type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

// New creates a new cognitive agent client with the given base URL and timeout.
// The underlying HTTP client uses default retry settings (3 retries, exponential backoff).
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		httpClient: httpclient.New(timeout),
		baseURL:    baseURL,
	}
}

// NewWithHTTPClient creates a new cognitive agent client with a pre-configured
// httpclient.Client. Use this when you need custom retry or timeout settings.
func NewWithHTTPClient(baseURL string, httpClient *httpclient.Client) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

// SendExtraction sends a knowledge extraction request to POST /api/knowledge-mgmt/extraction
// and returns the knowledge cognition response.
func (c *Client) SendExtraction(ctx context.Context, req *ExtractionRequest) (*KnowledgeCognitionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal extraction request: %w", err)
	}

	var result KnowledgeCognitionResponse
	if err := c.post(ctx, "/api/knowledge-mgmt/extraction", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendReasoningEvidence sends a reasoning evidence request to POST /api/knowledge-mgmt/reasoning/evidence
// and returns the reasoner cognition response.
func (c *Client) SendReasoningEvidence(ctx context.Context, req *ReasoningEvidenceRequest) (*ReasonerCognitionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reasoning evidence request: %w", err)
	}

	var result ReasonerCognitionResponse
	if err := c.post(ctx, "/api/knowledge-mgmt/reasoning/evidence", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendSemanticNegotiation sends a semantic negotiation request to POST /api/semantic-negotiation
// and returns the semantic negotiation response.
func (c *Client) SendSemanticNegotiation(ctx context.Context, req *SemanticNegotiationRequest) (*SemanticNegotiationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal semantic negotiation request: %w", err)
	}

	var result SemanticNegotiationResponse
	if err := c.post(ctx, "/api/semantic-negotiation", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// jsonHeaders are the default headers sent with every request.
var jsonHeaders = map[string]string{
	"Content-Type": "application/json",
	"Accept":       "application/json",
}

// post sends a POST request with a JSON body to baseURL+path, checks for a
// 200 OK response, and decodes the JSON body into dest.
func (c *Client) post(ctx context.Context, path string, body []byte, dest interface{}) error {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	resp, err := c.httpClient.Post(ctx, url, body, jsonHeaders)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, string(errBody))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("failed to decode response from %s: %w", path, err)
	}

	return nil
}
