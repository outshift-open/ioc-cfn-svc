// Package cognitionagentclient provides a client for the cognition agents API.
//
// It supports the following endpoints:
//   - POST /api/knowledge-mgmt/extraction          — ingest agent telemetry data and extract knowledge.
//   - POST /api/knowledge-mgmt/reasoning/evidence   — reasoning evidence request with an intent query.
//   - POST /api/semantic-negotiation/start          — initiate a semantic negotiation session.
//   - POST /api/semantic-negotiation/decide         — advance a semantic negotiation session.
//
// The client wraps httpclient.Client for retries and exponential backoff.
//
// NOTE: The Go struct fields / JSON tags in this package may change as the
// upstream cognition agents API evolves. Update the structs and paths here
// when the API contract is modified.
//
// TODO: Add audit CRUD operations for cognition agent API calls.
package cognitionagentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
)

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// ExtractionPayloadMetadata describes the format and labels of the incoming data.
type ExtractionPayloadMetadata struct {
	// Format specifies how the Data field should be interpreted.
	//
	// Supported values:
	// - "observe-sdk-otel": Data is a JSON array of ExtractionDataRecord
	// - "openclaw": Data is an opaque JSON payload
	Format string `json:"format"`
}

func (m ExtractionPayloadMetadata) Validate() error {
	switch m.Format {
	case common.FormatObserveSDKOTel, common.FormatOpenClaw:
		return nil
	default:
		return fmt.Errorf(
			"invalid metadata.format %q (supported: %q, %q)",
			m.Format,
			common.FormatObserveSDKOTel,
			common.FormatOpenClaw,
		)
	}
}

// ExtractionPayload holds the metadata and raw data array for an extraction request.
type ExtractionPayload struct {
	// Metadata describes the format and interpretation of the payload.
	Metadata ExtractionPayloadMetadata `json:"metadata"`
	// Data contains the extraction payload and its structure depends on Metadata.Format.
	//
	// Supported formats: "observe-sdk-otel" and "openclaw
	//
	// 1. format = "observe-sdk-otel"
	//    - Data MUST be a JSON array of ExtractionDataRecord objects.
	//    - Example:
	// [
	//   { TraceId, SpanId, ParentSpanId, SpanName, ServiceName, SpanAttributes, Duration }
	// ]
	//
	// 2. format = "openclaw"
	//    - Data is an opaque JSON payload.
	//    - The structure is not interpreted or validated by this service and is processed as-is.
	//
	// Clients MUST ensure the Data field matches the structure required by the specified Metadata.Format.
	Data json.RawMessage `json:"data" swaggertype:"object"`
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
	Header    common.Header     `json:"header"`
	RequestID string            `json:"request_id"`
	Payload   ExtractionPayload `json:"payload"`
}

// ReasoningEvidencePayloadMetadata describes the type of reasoning query.
type ReasoningEvidencePayloadMetadata struct {
	QueryType string `json:"query_type,omitempty"` // e.g. "Semantic Graph Traversal",
}

// ReasoningEvidencePayload holds the intent and optional context for a reasoning evidence request.
type ReasoningEvidencePayload struct {
	Metadata          ReasoningEvidencePayloadMetadata `json:"metadata,omitempty"`
	Intent            string                           `json:"intent"`
	AdditionalContext []interface{}                    `json:"additional_context,omitempty"`
}

// ReasoningEvidenceRequest is the request body for POST /api/knowledge-mgmt/reasoning/evidence.
type ReasoningEvidenceRequest struct {
	Header    common.Header            `json:"header"`
	RequestID *string                  `json:"request_id,omitempty"`
	Payload   ReasoningEvidencePayload `json:"payload"`
}

// SemanticNegotiationAgent represents a participant in a semantic negotiation session.
type SemanticNegotiationAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SemanticNegotiationAgentReply represents a single agent reply in a negotiation session.
type SemanticNegotiationAgentReply struct {
	AgentID string                 `json:"agent_id"`
	Action  string                 `json:"action"` // "accept", "reject", or "counter_offer"
	Offer   map[string]interface{} `json:"offer,omitempty"`
}

// SemanticNegotiationStartRequest is the request body for POST /negotiate/initiate.
type SemanticNegotiationStartRequest struct {
	SessionID   string                     `json:"session_id"`
	ContentText string                     `json:"content_text"`
	Agents      []SemanticNegotiationAgent `json:"agents"`
	NSteps      *int                       `json:"n_steps,omitempty"`
}

// SemanticNegotiationDecideRequest is the request body for POST /negotiate/decide.
type SemanticNegotiationDecideRequest struct {
	SessionID    string                          `json:"session_id"`
	AgentReplies []SemanticNegotiationAgentReply `json:"agent_replies"`
}

// ---------------------------------------------------------------------------
// SSTP envelope types (required by the semantic negotiation API)
// ---------------------------------------------------------------------------

// sstpOrigin identifies the actor and tenant for an SSTP message.
type sstpOrigin struct {
	ActorID  string `json:"actor_id"`
	TenantID string `json:"tenant_id"`
}

// sstpNegotiateSemanticContext carries the SAO session identifier.
// Only session_id is required on outbound requests; issues and options_per_issue
// are populated by the server on response messages.
type sstpNegotiateSemanticContext struct {
	SessionID string `json:"session_id"`
}

// sstpPolicyLabels carries data-handling policy annotations.
type sstpPolicyLabels struct {
	Sensitivity     string `json:"sensitivity"`
	Propagation     string `json:"propagation"`
	RetentionPolicy string `json:"retention_policy"`
}

// sstpProvenance carries message lineage information.
type sstpProvenance struct {
	Sources    []string `json:"sources"`
	Transforms []string `json:"transforms"`
}

// sstpNegotiateMessage is the SSTPNegotiateMessage envelope expected by
// POST /negotiate/initiate and POST /negotiate/decide.
type sstpNegotiateMessage struct {
	Kind            string                       `json:"kind"`
	Version         string                       `json:"version"`
	MessageID       string                       `json:"message_id"`
	DtCreated       string                       `json:"dt_created"`
	Origin          sstpOrigin                   `json:"origin"`
	SemanticContext sstpNegotiateSemanticContext `json:"semantic_context"`
	PayloadHash     string                       `json:"payload_hash"`
	PolicyLabels    sstpPolicyLabels             `json:"policy_labels"`
	Provenance      sstpProvenance               `json:"provenance"`
	Payload         map[string]interface{}       `json:"payload"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// KnowledgeCognitionResponse is the response from POST /api/knowledge-mgmt/extraction.
type KnowledgeCognitionResponse struct {
	Header     common.Header       `json:"header"`
	ResponseID string              `json:"response_id"`
	Error      *common.ErrorDetail `json:"error,omitempty"`
	Concepts   []Concept           `json:"concepts,omitempty"`
	Relations  []Relation          `json:"relations,omitempty"`
	RagChunks  []RagChunk          `json:"rag_chunks,omitempty"`
	Descriptor string              `json:"descriptor,omitempty"`
	Metadata   Meta                `json:"metadata,omitempty"`
}

// RagChunkMetadata contains metadata fields associated with a RAG chunk.
type RagChunkMetadata struct {
	Domain     string `json:"domain"`
	Timestamp  string `json:"timestamp"`
	DocIndex   int    `json:"doc_index"`
	ChunkIndex int    `json:"chunk_index"`
}

// RagChunk represents a single text chunk with its embedding returned by the extraction service.
// Embedding is a list of embedding vectors (one per model); we use the first one.
type RagChunk struct {
	Text      string           `json:"text"`
	Metadata  RagChunkMetadata `json:"metadata"`
	Embedding [][]float64      `json:"embedding,omitempty"`
}

type ConceptAttributes struct {
	ConceptType string      `json:"concept_type"`
	Embedding   [][]float64 `json:"embedding,omitempty"`

	// Extra captures arbitrary additional fields (equivalent to extra="allow" in Pydantic)
	Extra map[string]any `json:"-"`
}

// Concept represents an extracted concept from telemetry data.
type Concept struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Attributes  ConceptAttributes `json:"attributes"`
}

type RelationAttributes struct {
	SourceName        string `json:"source_name"`
	TargetName        string `json:"target_name"`
	SummarizedContext string `json:"summarized_context"`

	// Extra captures arbitrary additional fields (equivalent to extra="allow" in Pydantic)
	Extra map[string]any `json:"-"`
}

// Relation represents a relationship between concepts.
type Relation struct {
	ID           string                 `json:"id"`
	NodeIDs      []string               `json:"node_ids"`
	Relationship string                 `json:"relationship"`
	Attributes   map[string]interface{} `json:"attributes"`
}

type ReasonerRecord struct {
	Content ReasonerContent `json:"content"`
}

type ReasonerContent struct {
	Evidence ReasonerEvidence `json:"evidence"`
}

type ReasonerEvidence struct {
	Details       ReasonerDetails `json:"details"`
	Status        string          `json:"status,omitempty"`
	FinalResponse string          `json:"final_response,omitempty"`
}

type ReasonerDetails struct {
	Concepts  []ReasonerConcept  `json:"concepts"`
	Relations []ReasonerRelation `json:"relations"`
}

type ReasonerConcept struct {
	ConceptID   string                 `json:"concept_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Attributes  map[string]interface{} `json:"attributes"`
}

type ReasonerRelation struct {
	ID         string                 `json:"id"`
	NodeIDs    []string               `json:"node_ids"`
	Relation   string                 `json:"relationship"`
	Attributes map[string]interface{} `json:"attributes"`
}

// ReasonerCognitionResponse is the response from POST /api/knowledge-mgmt/reasoning/evidence.
type ReasonerCognitionResponse struct {
	Header     common.Header          `json:"header"`
	ResponseID string                 `json:"response_id"`
	Error      *common.ErrorDetail    `json:"error,omitempty"`
	Records    []ReasonerRecord       `json:"records,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SemanticNegotiationResponse is the response from POST /negotiate/initiate and
// POST /negotiate/decide. Both endpoints return an SSTPNegotiateMessage envelope
// whose payload carries the negotiation result.
//
// For /negotiate/initiate the payload contains the full InitiateResponse (status,
// current_round, trace, etc.). For /negotiate/decide the payload carries either
// {session_id, status, round, messages} (ongoing) or {session_id, status, round,
// final_result} (terminal).
type SemanticNegotiationResponse struct {
	// Top-level SSTP envelope fields
	Kind      string                 `json:"kind,omitempty"`
	MessageID string                 `json:"message_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`

	// Flat fields returned by /negotiate/decide (non-envelope path)
	Status      string                 `json:"status,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Round       *int                   `json:"round,omitempty"`
	Messages    []interface{}          `json:"messages,omitempty"`
	FinalResult map[string]interface{} `json:"final_result,omitempty"`

	Error *common.ErrorDetail `json:"error,omitempty"`
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

// Client is a client for the cognition agents API.
// It wraps an httpclient.Client and a base URL for the target service.
type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

// New creates a new cognition agent client with the given base URL and timeout.
// The underlying HTTP client uses default retry settings (3 retries, exponential backoff).
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		httpClient: httpclient.New(timeout),
		baseURL:    baseURL,
	}
}

// NewWithHTTPClient creates a new cognition agent client with a pre-configured
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

// SendSemanticNegotiationStart initiates a new semantic negotiation session.
// POST /negotiate/initiate
//
// The Python service expects a full SSTPNegotiateMessage envelope where:
//   - semantic_context.session_id carries the caller-supplied session ID
//   - origin.actor_id / origin.tenant_id are set to a placeholder (the server
//     reads workspace/mas from the SSTP origin on the initiate path)
//   - payload carries content_text, agents, and optional n_steps
func (c *Client) SendSemanticNegotiationStart(ctx context.Context, req *SemanticNegotiationStartRequest, workspaceID, masID string) (*SemanticNegotiationResponse, error) {
	agentsRaw := make([]map[string]string, len(req.Agents))
	for i, a := range req.Agents {
		agentsRaw[i] = map[string]string{"id": a.ID, "name": a.Name}
	}

	payload := map[string]interface{}{
		"content_text": req.ContentText,
		"agents":       agentsRaw,
	}
	if req.NSteps != nil {
		payload["n_steps"] = *req.NSteps
	}

	envelope := sstpNegotiateMessage{
		Kind:      "negotiate",
		Version:   "0",
		MessageID: req.SessionID,
		DtCreated: time.Now().UTC().Format(time.RFC3339),
		Origin: sstpOrigin{
			ActorID:  masID,
			TenantID: workspaceID,
		},
		SemanticContext: sstpNegotiateSemanticContext{
			SessionID: req.SessionID,
		},
		PayloadHash:  "",
		PolicyLabels: sstpPolicyLabels{Sensitivity: "internal", Propagation: "restricted", RetentionPolicy: "default"},
		Provenance:   sstpProvenance{Sources: []string{}, Transforms: []string{}},
		Payload:      payload,
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal semantic negotiation initiate envelope: %w", err)
	}

	var result SemanticNegotiationResponse
	if err := c.post(ctx, "/api/semantic-negotiation/negotiate/initiate", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendSemanticNegotiationDecide advances an existing semantic negotiation session.
// POST /negotiate/decide
//
// The Python service expects an SSTPNegotiateMessage envelope where:
//   - payload.session_id identifies the active session
//   - payload.agent_replies holds the list of agent reply dicts
func (c *Client) SendSemanticNegotiationDecide(ctx context.Context, req *SemanticNegotiationDecideRequest, workspaceID, masID string) (*SemanticNegotiationResponse, error) {
	repliesRaw := make([]map[string]interface{}, len(req.AgentReplies))
	for i, r := range req.AgentReplies {
		m := map[string]interface{}{
			"agent_id": r.AgentID,
			"action":   r.Action,
		}
		if r.Offer != nil {
			m["offer"] = r.Offer
		}
		repliesRaw[i] = m
	}

	payload := map[string]interface{}{
		"session_id":    req.SessionID,
		"agent_replies": repliesRaw,
	}

	envelope := sstpNegotiateMessage{
		Kind:      "negotiate",
		Version:   "0",
		MessageID: req.SessionID,
		DtCreated: time.Now().UTC().Format(time.RFC3339),
		Origin: sstpOrigin{
			ActorID:  masID,
			TenantID: workspaceID,
		},
		SemanticContext: sstpNegotiateSemanticContext{
			SessionID: req.SessionID,
		},
		PayloadHash:  "",
		PolicyLabels: sstpPolicyLabels{Sensitivity: "internal", Propagation: "restricted", RetentionPolicy: "default"},
		Provenance:   sstpProvenance{Sources: []string{}, Transforms: []string{}},
		Payload:      payload,
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal semantic negotiation decide envelope: %w", err)
	}

	var result SemanticNegotiationResponse
	if err := c.post(ctx, "/api/semantic-negotiation/negotiate/decide", body, &result); err != nil {
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

// post sends a POST request with a JSON body to baseURL+path and decodes the
// JSON response into dest. If the server returns a non-200 response, we attempt
// to decode the body as the same response envelope (which may contain an
// error field) so callers get actionable error details.
func (c *Client) post(ctx context.Context, path string, body []byte, dest interface{}) error {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	resp, err := c.httpClient.Post(ctx, url, body, jsonHeaders)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body from %s: %w", path, readErr)
	}

	if resp.StatusCode != http.StatusOK {
		if err := json.Unmarshal(respBody, dest); err == nil {
			return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
		}
		return fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, string(respBody))
	}

	if err := json.Unmarshal(respBody, dest); err != nil {
		return fmt.Errorf("failed to decode response from %s: %w", path, err)
	}

	return nil
}
