// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package mem0

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ──────────────────────────────────────────────────────────
// Mem0 / OpenMemory API schema types
// Validated against live OpenMemory Docker endpoint (self-hosted mem0).
// Reference: https://docs.mem0.ai/api-reference
//
// API prefix: /api/v1
// ──────────────────────────────────────────────────────────

// --- Common types ---

// Metadata holds arbitrary key-value metadata attached to memories.
// Note: the API serialises this field as "metadata_" (trailing underscore).
type Metadata map[string]interface{}

// MemoryState represents the state of a memory.
type MemoryState string

const (
	MemoryStateActive   MemoryState = "active"
	MemoryStateDeleted  MemoryState = "deleted"
	MemoryStateArchived MemoryState = "archived"
	MemoryStatePaused   MemoryState = "paused"
)

// --- Memory item shapes (vary slightly per endpoint) ---

// MemoryListItem is the shape returned inside the "items" array from
// GET /api/v1/memories/ and POST /api/v1/memories/filter.
type MemoryListItem struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	CreatedAt  int64    `json:"created_at"`
	State      string   `json:"state"`
	AppID      string   `json:"app_id,omitempty"`
	AppName    string   `json:"app_name,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Metadata   Metadata `json:"metadata_,omitempty"`
}

// MemoryDetail is the shape returned from GET /api/v1/memories/{memory_id}.
// Note: the text field is "text" here (not "content").
type MemoryDetail struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	CreatedAt  int64    `json:"created_at"`
	State      string   `json:"state"`
	AppID      string   `json:"app_id,omitempty"`
	AppName    string   `json:"app_name,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Metadata   Metadata `json:"metadata_,omitempty"`
}

// MemoryMutationResult is the shape returned by POST (create) and PUT (update).
type MemoryMutationResult struct {
	UserID     string  `json:"user_id,omitempty"`
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	State      string  `json:"state"`
	UpdatedAt  string  `json:"updated_at,omitempty"`
	CreatedAt  string  `json:"created_at,omitempty"`
	DeletedAt  *string `json:"deleted_at,omitempty"`
	ArchivedAt *string `json:"archived_at,omitempty"`
	AppID      string  `json:"app_id,omitempty"`
	Metadata   Metadata `json:"metadata_,omitempty"`
}

// --- Pagination ---

// PaginatedResponse is the common wrapper for list endpoints.
type PaginatedResponse struct {
	Items []MemoryListItem `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Size  int              `json:"size"`
	Pages int              `json:"pages"`
}

// --- Create Memory ---
// POST /api/v1/memories/

// CreateMemoryRequest is the payload for creating a memory.
type CreateMemoryRequest struct {
	Text   string   `json:"text"`
	UserID string   `json:"user_id"`
	Metadata Metadata `json:"metadata_,omitempty"`
}

// Validate checks that the request is well-formed.
func (r *CreateMemoryRequest) Validate() error {
	if strings.TrimSpace(r.Text) == "" {
		return errors.New("text is required")
	}
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}

// CreateMemoryResponse is returned from POST /api/v1/memories/.
type CreateMemoryResponse = MemoryMutationResult

// --- List Memories ---
// GET /api/v1/memories/?user_id=xxx

// ListMemoriesRequest holds query parameters for listing memories.
type ListMemoriesRequest struct {
	UserID string `json:"user_id"`
	Page   int    `json:"page,omitempty"`
	Size   int    `json:"size,omitempty"`
}

// Validate checks that user_id is provided.
func (r *ListMemoriesRequest) Validate() error {
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}

// ListMemoriesResponse is returned from GET /api/v1/memories/.
type ListMemoriesResponse = PaginatedResponse

// --- Get Single Memory ---
// GET /api/v1/memories/{memory_id}

// GetMemoryResponse is returned from GET /api/v1/memories/{memory_id}.
type GetMemoryResponse = MemoryDetail

// --- Update Memory ---
// PUT /api/v1/memories/{memory_id}

// UpdateMemoryRequest is the payload for updating a memory.
type UpdateMemoryRequest struct {
	MemoryContent string `json:"memory_content"`
	UserID        string `json:"user_id"`
}

// Validate checks that required fields are present.
func (r *UpdateMemoryRequest) Validate() error {
	if strings.TrimSpace(r.MemoryContent) == "" {
		return errors.New("memory_content is required")
	}
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}

// UpdateMemoryResponse is returned from PUT /api/v1/memories/{memory_id}.
type UpdateMemoryResponse = MemoryMutationResult

// --- Delete Memories ---
// DELETE /api/v1/memories/

// DeleteMemoriesRequest is the payload for deleting memories.
type DeleteMemoriesRequest struct {
	MemoryIDs []string `json:"memory_ids"`
	UserID    string   `json:"user_id"`
}

// Validate checks that required fields are present.
func (r *DeleteMemoriesRequest) Validate() error {
	if len(r.MemoryIDs) == 0 {
		return errors.New("at least one memory_id is required in memory_ids")
	}
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}

// DeleteMemoriesResponse is returned from DELETE /api/v1/memories/.
type DeleteMemoriesResponse struct {
	Message string `json:"message"`
}

// --- Filter / Search Memories ---
// POST /api/v1/memories/filter

// FilterMemoriesRequest is the payload for filtering/searching memories.
type FilterMemoriesRequest struct {
	Query  string `json:"query"`
	UserID string `json:"user_id"`
}

// Validate checks that required fields are present.
func (r *FilterMemoriesRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return errors.New("query is required")
	}
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}

// FilterMemoriesResponse is returned from POST /api/v1/memories/filter.
type FilterMemoriesResponse = PaginatedResponse

// --- Categories ---
// GET /api/v1/memories/categories?user_id=xxx

// Category represents a memory category.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// CategoriesResponse is returned from GET /api/v1/memories/categories.
type CategoriesResponse struct {
	Categories []Category `json:"categories"`
	Total      int        `json:"total"`
}

// --- Related Memories ---
// GET /api/v1/memories/{memory_id}/related?user_id=xxx

// RelatedMemoriesResponse is returned from GET /api/v1/memories/{memory_id}/related.
type RelatedMemoriesResponse = PaginatedResponse

// --- Memory Access Log ---
// GET /api/v1/memories/{memory_id}/access-log

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	ID        string `json:"id"`
	MemoryID  string `json:"memory_id"`
	AppID     string `json:"app_id,omitempty"`
	AppName   string `json:"app_name,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// --- Stats ---
// GET /api/v1/stats/?user_id=xxx

// StatsResponse is returned from GET /api/v1/stats/.
type StatsResponse struct {
	TotalMemories int              `json:"total_memories"`
	TotalApps     int              `json:"total_apps"`
	Apps          []map[string]interface{} `json:"apps,omitempty"`
}

// --- Error Response ---

// APIError represents an error response from the mem0 API.
type APIError struct {
	StatusCode int         `json:"status_code"`
	Detail     interface{} `json:"detail"` // can be string or []ValidationError
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mem0 API error (status %d): %v", e.StatusCode, e.Detail)
}

// --- Envelope types matching the existing MemoryOperationPayload ---

// ProxyRequest maps incoming memory-operations envelope to mem0 API calls.
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

// ProxyResponse wraps the mem0 API response for the envelope.
type ProxyResponse struct {
	HTTPStatus       int                    `json:"http-status"`
	HTTPHeaders      map[string]string      `json:"http-headers,omitempty"`
	HTTPResponseBody map[string]interface{} `json:"http-response-body,omitempty"`
}
