// Package cognitiveagentclient provides a client for the Cognitive Agents API.
//
// It supports three endpoints:
//   - POST /api/_otel      — ingest OpenTelemetry span data and extract knowledge.
//   - POST /api/_general   — general knowledge cognition request.
//   - POST /api/_reasoner  — reasoner evidence request with an intent query.
//
// The client wraps httpclient.Client for retries and exponential backoff.
//
// NOTE: API endpoint suffixes (e.g. /api/_otel, /api/_general, /api/_reasoner)
// and the Go struct fields / JSON tags in this package may change as the
// upstream Cognitive Agents API evolves. Update the structs and paths here
// when the API contract is modified.
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
// Request types
// NOTE: Struct fields and JSON tags may change as the API evolves.
// ---------------------------------------------------------------------------

// OtelSpan represents a single OpenTelemetry span record sent to POST /api/_otel.
type OtelSpan struct {
	MASID          string            `json:"mas_id"`          // Multi-Agent System identifier
	WorkspaceID    string            `json:"workspace_id"`    // Workspace identifier
	TraceID        string            `json:"TraceId"`         // Distributed trace ID
	SpanID         string            `json:"SpanId"`          // Unique span ID within the trace
	ParentSpanID   string            `json:"ParentSpanId"`    // Parent span ID (empty for root spans)
	SpanName       string            `json:"SpanName"`        // Human-readable span name
	ServiceName    string            `json:"ServiceName"`     // Originating service name
	SpanAttributes map[string]string `json:"SpanAttributes"` // Arbitrary key-value span attributes
	Duration       int64             `json:"Duration"`        // Span duration in nanoseconds
}

// GeneralRequest represents a request to POST /api/_general.
type GeneralRequest struct {
	KnowledgeCognitionRequestID string `json:"knowledge_cognition_request_id"` // ID from a prior _otel response
	MASID                       string `json:"mas_id"`                         // Multi-Agent System identifier
	WorkspaceID                 string `json:"workspace_id"`                   // Workspace identifier
}

// ReasonerRequest represents a request to POST /api/_reasoner.
type ReasonerRequest struct {
	MASID             string                 `json:"mas_id"`              // Multi-Agent System identifier
	WorkspaceID       string                 `json:"workspace_id"`        // Workspace identifier
	Intent            string                 `json:"intent"`              // Natural-language query / intent
	AdditionalContext []interface{}           `json:"additional_context"` // Extra context (to be updated in the API)
	Meta              map[string]interface{} `json:"meta"`               // Arbitrary metadata
}

// ---------------------------------------------------------------------------
// Response types
// NOTE: Struct fields and JSON tags may change as the API evolves.
// ---------------------------------------------------------------------------

// ReasonerCognitionResponse is the response from POST /api/_reasoner.
// TODO: define fields once the API response schema is finalized.
type ReasonerCognitionResponse struct {
	Raw json.RawMessage `json:"raw"` // Raw JSON until schema is defined
}

// GeneralCognitionResponse is the response from POST /api/_general.
// TODO: define fields once the API response schema is finalized.
type GeneralCognitionResponse struct {
	Raw json.RawMessage `json:"raw"` // Raw JSON until schema is defined
}

// KnowledgeCognitionResponse is the response from POST /api/_otel.
type KnowledgeCognitionResponse struct {
	KnowledgeCognitionRequestID string     `json:"knowledge_cognition_request_id"`
	Concepts                    []Concept  `json:"concepts"`
	Relations                   []Relation `json:"relations"`
	Descriptor                  string     `json:"descriptor"`
	Meta                        Meta       `json:"meta"`
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
// NOTE: Endpoint paths may change as the API evolves.
// ---------------------------------------------------------------------------

// SendOtelSpans sends OpenTelemetry span data to POST /api/_otel
// and returns the extracted knowledge cognition response.
func (c *Client) SendOtelSpans(ctx context.Context, spans []OtelSpan) (*KnowledgeCognitionResponse, error) {
	body, err := json.Marshal(spans)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal otel spans: %w", err)
	}

	var result KnowledgeCognitionResponse
	if err := c.post(ctx, "/api/_otel", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendGeneral sends a general knowledge cognition request to POST /api/_general
// and returns the general cognition response.
func (c *Client) SendGeneral(ctx context.Context, requests []GeneralRequest) (*GeneralCognitionResponse, error) {
	body, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal general requests: %w", err)
	}

	var result GeneralCognitionResponse
	if err := c.post(ctx, "/api/_general", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendReasonerEvidence sends a reasoner evidence request to POST /api/_reasoner
// and returns the reasoner cognition response.
func (c *Client) SendReasonerEvidence(ctx context.Context, request *ReasonerRequest) (*ReasonerCognitionResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reasoner request: %w", err)
	}

	var result ReasonerCognitionResponse
	if err := c.post(ctx, "/api/_reasoner", body, &result); err != nil {
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
