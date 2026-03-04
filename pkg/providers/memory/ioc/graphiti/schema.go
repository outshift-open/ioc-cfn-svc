package graphiti

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────
// Graphiti Knowledge Graph API schema types
// Validated against getzep/graphiti FastAPI server (zepai/graphiti Docker image).
// Reference: https://github.com/getzep/graphiti
//
// The server exposes two routers (ingest + retrieve) with no API prefix.
// ──────────────────────────────────────────────────────────

// --- Common types ---

// Message represents a conversational message used for ingestion and retrieval.
type Message struct {
	Content           string  `json:"content"`
	UUID              *string `json:"uuid,omitempty"`
	Name              string  `json:"name,omitempty"`
	RoleType          string  `json:"role_type"`
	Role              *string `json:"role,omitempty"`
	Timestamp         *string `json:"timestamp,omitempty"`
	SourceDescription string  `json:"source_description,omitempty"`
}

// Result is the generic success/failure response from mutation endpoints.
type Result struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// --- Ingest types ---

// AddMessagesRequest is the payload for POST /messages.
type AddMessagesRequest struct {
	GroupID  string    `json:"group_id"`
	Messages []Message `json:"messages"`
}

// Validate checks that the request is well-formed.
func (r *AddMessagesRequest) Validate() error {
	if strings.TrimSpace(r.GroupID) == "" {
		return errors.New("group_id is required")
	}
	if len(r.Messages) == 0 {
		return errors.New("at least one message is required")
	}
	for i, m := range r.Messages {
		if strings.TrimSpace(m.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
		if m.RoleType != "user" && m.RoleType != "assistant" && m.RoleType != "system" {
			return fmt.Errorf("messages[%d].role_type must be user, assistant, or system", i)
		}
	}
	return nil
}

// AddEntityNodeRequest is the payload for POST /entity-node.
type AddEntityNodeRequest struct {
	UUID    string `json:"uuid"`
	GroupID string `json:"group_id"`
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

// Validate checks that the request is well-formed.
func (r *AddEntityNodeRequest) Validate() error {
	if strings.TrimSpace(r.UUID) == "" {
		return errors.New("uuid is required")
	}
	if strings.TrimSpace(r.GroupID) == "" {
		return errors.New("group_id is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

// --- Retrieve types ---

// SearchQuery is the payload for POST /search.
type SearchQuery struct {
	GroupIDs []string `json:"group_ids,omitempty"`
	Query    string   `json:"query"`
	MaxFacts int      `json:"max_facts,omitempty"`
}

// Validate checks that the request is well-formed.
func (r *SearchQuery) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return errors.New("query is required")
	}
	return nil
}

// FactResult represents a single fact from the knowledge graph.
type FactResult struct {
	UUID      string     `json:"uuid"`
	Name      string     `json:"name"`
	Fact      string     `json:"fact"`
	ValidAt   *time.Time `json:"valid_at,omitempty"`
	InvalidAt *time.Time `json:"invalid_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiredAt *time.Time `json:"expired_at,omitempty"`
}

// SearchResults is the response from POST /search.
type SearchResults struct {
	Facts []FactResult `json:"facts"`
}

// GetMemoryRequest is the payload for POST /get-memory.
type GetMemoryRequest struct {
	GroupID        string    `json:"group_id"`
	MaxFacts       int       `json:"max_facts,omitempty"`
	CenterNodeUUID *string   `json:"center_node_uuid"`
	Messages       []Message `json:"messages"`
}

// Validate checks that the request is well-formed.
func (r *GetMemoryRequest) Validate() error {
	if strings.TrimSpace(r.GroupID) == "" {
		return errors.New("group_id is required")
	}
	if len(r.Messages) == 0 {
		return errors.New("at least one message is required")
	}
	return nil
}

// GetMemoryResponse is the response from POST /get-memory.
type GetMemoryResponse struct {
	Facts []FactResult `json:"facts"`
}

// HealthCheckResponse is the response from GET /healthcheck.
type HealthCheckResponse struct {
	Status string `json:"status"`
}

// --- Envelope types matching the existing MemoryOperationPayload ---

// ProxyRequest maps incoming memory-operations envelope to Graphiti API calls.
type ProxyRequest struct {
	HTTPMethod  string                 `json:"http-request-type"`
	HTTPURL     string                 `json:"http-url"`
	HTTPBody    map[string]interface{} `json:"http-request-body,omitempty"`
	HTTPHeaders map[string]string      `json:"http-headers,omitempty"`
}

// Validate performs basic validation on the proxy request.
func (r *ProxyRequest) Validate() error {
	if r.HTTPMethod == "" {
		return errors.New("http-request-type is required")
	}
	method := strings.ToUpper(r.HTTPMethod)
	allowed := map[string]bool{
		http.MethodGet:    true,
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodDelete: true,
		http.MethodPatch:  true,
	}
	if !allowed[method] {
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}
	if r.HTTPURL == "" {
		return errors.New("http-url is required")
	}
	return nil
}

// ProxyResponse wraps the Graphiti API response for the envelope.
type ProxyResponse struct {
	HTTPStatus       int                    `json:"http-status"`
	HTTPHeaders      map[string]string      `json:"http-headers,omitempty"`
	HTTPResponseBody map[string]interface{} `json:"http-response-body,omitempty"`
}

// --- Error Response ---

// APIError represents an error response from the Graphiti API.
type APIError struct {
	StatusCode int         `json:"status_code"`
	Detail     interface{} `json:"detail"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("graphiti API error (status %d): %v", e.StatusCode, e.Detail)
}
