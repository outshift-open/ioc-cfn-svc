// Package mem0 provides a Go client for the Mem0 / OpenMemory API.
//
// It acts as a typed proxy: the handler receives a generic MemoryOperationPayload
// envelope and this client forwards the request to the mem0 API, adding
// authentication, retries, logging, and response normalisation.
//
// Validated against the OpenMemory Docker endpoint (self-hosted mem0).
// Reference: https://docs.mem0.ai/api-reference
package mem0

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const (
	// DefaultBaseURL is the default self-hosted mem0 (OpenMemory) endpoint.
	DefaultBaseURL = "http://localhost:8765"

	// DefaultTimeout for HTTP requests to mem0.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries for transient failures.
	DefaultMaxRetries = 3

	// APIPrefix is the mem0 / OpenMemory API path prefix.
	APIPrefix = "/api/v1"
)

var log = logger.SubPkg("mem0")

// ClientConfig holds configuration for the mem0 client.
type ClientConfig struct {
	// BaseURL is the mem0 API base URL (e.g. http://localhost:8765).
	BaseURL string

	// APIKey is the mem0 API key used for Token authentication.
	// MUST be sourced from environment variables or a secret manager—never hardcoded.
	// For self-hosted OpenMemory without auth this can be set to any non-empty value.
	APIKey string

	// Timeout for individual HTTP requests.
	Timeout time.Duration

	// MaxRetries for transient failures (429, 5xx).
	MaxRetries int

	// OrgID is an optional organisation ID for multi-tenant setups.
	OrgID string

	// ProjectID is an optional project ID for multi-tenant setups.
	ProjectID string
}

// DefaultClientConfig returns a ClientConfig with sensible defaults.
// The caller MUST set APIKey (from env/vault) before using.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:    DefaultBaseURL,
		Timeout:    DefaultTimeout,
		MaxRetries: DefaultMaxRetries,
	}
}

// Validate ensures all required configuration is present.
func (c *ClientConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("mem0 client: base URL is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("mem0 client: API key is required (set via environment variable)")
	}
	return nil
}

// Client is the Agentic Memory Client for mem0 / OpenMemory.
type Client struct {
	httpClient *httpclient.Client
	config     *ClientConfig
}

// NewClient creates a new mem0 Client and validates configuration.
// APIKey should be sourced from environment variables or a secret manager.
func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Strip trailing slash from base URL
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	httpCfg := httpclient.DefaultConfig()
	httpCfg.Timeout = cfg.Timeout
	httpCfg.MaxRetries = cfg.MaxRetries

	client := &Client{
		httpClient: httpclient.NewWithConfig(httpCfg),
		config:     cfg,
	}

	log.Infof("mem0 agentic memory client initialised, baseURL=%s", cfg.BaseURL)
	return client, nil
}

// ──────────────────────────────────────────────────────────
// High-level typed API methods
// ──────────────────────────────────────────────────────────

// CreateMemory stores a new memory.
// POST /api/v1/memories/
//
// Note: if OpenMemory detects the memory as a duplicate, the API returns null.
// In that case this method returns (nil, nil) — no error, but no new memory created.
func (c *Client) CreateMemory(ctx context.Context, req *CreateMemoryRequest) (*CreateMemoryResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var resp CreateMemoryResponse
	if err := c.doJSON(ctx, http.MethodPost, c.apiURL("/memories/"), req, &resp); err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}

	// OpenMemory returns null for duplicate memories — detect by empty ID
	if resp.ID == "" {
		log.Infof("create memory returned null (likely duplicate), no new memory created")
		return nil, nil
	}
	return &resp, nil
}

// ListMemories retrieves all memories for a user.
// GET /api/v1/memories/?user_id=xxx
func (c *Client) ListMemories(ctx context.Context, req *ListMemoriesRequest) (*ListMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	params := url.Values{}
	params.Set("user_id", req.UserID)
	if req.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", req.Page))
	}
	if req.Size > 0 {
		params.Set("size", fmt.Sprintf("%d", req.Size))
	}

	endpoint := c.apiURL("/memories/") + "?" + params.Encode()

	var resp ListMemoriesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	return &resp, nil
}

// GetMemory retrieves a single memory by ID.
// GET /api/v1/memories/{memory_id}
func (c *Client) GetMemory(ctx context.Context, memoryID string) (*GetMemoryResponse, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}

	var resp GetMemoryResponse
	if err := c.doJSON(ctx, http.MethodGet, c.apiURL("/memories/"+memoryID), nil, &resp); err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}
	return &resp, nil
}

// UpdateMemory updates a specific memory's content.
// PUT /api/v1/memories/{memory_id}
func (c *Client) UpdateMemory(ctx context.Context, memoryID string, req *UpdateMemoryRequest) (*UpdateMemoryResponse, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var resp UpdateMemoryResponse
	if err := c.doJSON(ctx, http.MethodPut, c.apiURL("/memories/"+memoryID), req, &resp); err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}
	return &resp, nil
}

// DeleteMemories deletes one or more memories by ID.
// DELETE /api/v1/memories/
func (c *Client) DeleteMemories(ctx context.Context, req *DeleteMemoriesRequest) (*DeleteMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var resp DeleteMemoriesResponse
	if err := c.doJSON(ctx, http.MethodDelete, c.apiURL("/memories/"), req, &resp); err != nil {
		return nil, fmt.Errorf("delete memories: %w", err)
	}
	return &resp, nil
}

// FilterMemories performs semantic search / filtering across memories.
// POST /api/v1/memories/filter
func (c *Client) FilterMemories(ctx context.Context, req *FilterMemoriesRequest) (*FilterMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var resp FilterMemoriesResponse
	if err := c.doJSON(ctx, http.MethodPost, c.apiURL("/memories/filter"), req, &resp); err != nil {
		return nil, fmt.Errorf("filter memories: %w", err)
	}
	return &resp, nil
}

// GetCategories retrieves memory categories for a user.
// GET /api/v1/memories/categories?user_id=xxx
func (c *Client) GetCategories(ctx context.Context, userID string) (*CategoriesResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	endpoint := c.apiURL("/memories/categories") + "?user_id=" + url.QueryEscape(userID)

	var resp CategoriesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	return &resp, nil
}

// GetRelatedMemories retrieves memories related to a given memory.
// GET /api/v1/memories/{memory_id}/related?user_id=xxx
func (c *Client) GetRelatedMemories(ctx context.Context, memoryID, userID string) (*RelatedMemoriesResponse, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	endpoint := c.apiURL("/memories/"+memoryID+"/related") + "?user_id=" + url.QueryEscape(userID)

	var resp RelatedMemoriesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("get related memories: %w", err)
	}
	return &resp, nil
}

// GetStats retrieves memory statistics for a user.
// GET /api/v1/stats/?user_id=xxx
func (c *Client) GetStats(ctx context.Context, userID string) (*StatsResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	endpoint := c.apiURL("/stats/") + "?user_id=" + url.QueryEscape(userID)

	var resp StatsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	return &resp, nil
}

// ──────────────────────────────────────────────────────────
// Proxy: generic envelope-based forwarding
// ──────────────────────────────────────────────────────────

// ForwardRequest takes the generic MemoryOperationPayload envelope from the
// handler and proxies it to mem0, injecting authentication and logging.
// This is the primary integration point used by memoryOperationsHandler.
//
// The targetURL must match the configured mem0 base host, or be a relative
// path (starting with "/") which is resolved against the base URL.
func (c *Client) ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*ProxyResponse, error) {
	if method == "" {
		return nil, errors.New("method is required")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if !isSupportedForwardMethod(method) {
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	safeTargetURL, err := c.resolveAndValidateTargetURL(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy target URL: %w", err)
	}

	// Inject auth and standard headers
	mergedHeaders := c.buildHeaders(headers)

	log.Infof("forwarding %s to mem0: %s", method, safeTargetURL)

	resp, err := c.httpClient.Do(ctx, method, safeTargetURL, body, mergedHeaders)
	if err != nil {
		return nil, fmt.Errorf("mem0 proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read mem0 response body: %w", err)
	}

	log.Infof("mem0 responded: status=%d, bodyLen=%d", resp.StatusCode, len(respBody))
	log.Debugf("mem0 response body: %s", string(respBody))

	// Parse response body as JSON
	var respJSON map[string]interface{}
	if len(respBody) > 0 {
		if jsonErr := json.Unmarshal(respBody, &respJSON); jsonErr != nil {
			// If not valid JSON, wrap raw text
			respJSON = map[string]interface{}{
				"raw": string(respBody),
			}
		}
	}

	// Extract response headers
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	return &ProxyResponse{
		HTTPStatus:       resp.StatusCode,
		HTTPHeaders:      respHeaders,
		HTTPResponseBody: respJSON,
	}, nil
}

// ──────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────

// apiURL constructs a full URL: baseURL + /api/v1 + path.
func (c *Client) apiURL(path string) string {
	return c.config.BaseURL + APIPrefix + path
}

// buildHeaders returns merged headers with authentication injected.
func (c *Client) buildHeaders(extra map[string]string) map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json",
		"Authorization": "Token " + c.config.APIKey,
	}

	// Add org/project headers if configured
	if c.config.OrgID != "" {
		headers["X-Org-Id"] = c.config.OrgID
	}
	if c.config.ProjectID != "" {
		headers["X-Project-Id"] = c.config.ProjectID
	}

	// Merge caller-supplied headers (they override defaults except Authorization)
	for k, v := range extra {
		// Never allow callers to override the auth header
		if strings.EqualFold(k, "Authorization") {
			continue
		}
		headers[k] = v
	}
	return headers
}

func isSupportedForwardMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

// resolveAndValidateTargetURL ensures requests can only be proxied to the
// configured mem0 endpoint. This prevents open-proxy behaviour and
// accidental credential leakage.
func (c *Client) resolveAndValidateTargetURL(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("target URL is required")
	}

	baseParsed, err := url.Parse(c.config.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid configured base URL: %w", err)
	}
	if baseParsed.Scheme == "" || baseParsed.Host == "" {
		return "", errors.New("configured base URL must include scheme and host")
	}

	var targetParsed *url.URL
	if strings.HasPrefix(target, "/") {
		// Support relative path in envelope payload, e.g. "/api/v1/memories/"
		targetParsed = baseParsed.ResolveReference(&url.URL{Path: target})
	} else {
		targetParsed, err = url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("failed to parse target URL: %w", err)
		}
		if !targetParsed.IsAbs() {
			return "", errors.New("target URL must be absolute or start with '/'")
		}
	}

	// Ensure the target endpoint exactly matches the configured mem0 host/scheme/port.
	if !strings.EqualFold(targetParsed.Scheme, baseParsed.Scheme) || !strings.EqualFold(targetParsed.Host, baseParsed.Host) {
		return "", fmt.Errorf("target host must match configured mem0 endpoint (%s://%s)", baseParsed.Scheme, baseParsed.Host)
	}

	return targetParsed.String(), nil
}

// doJSON performs a JSON request and unmarshals the response into dest.
func (c *Client) doJSON(ctx context.Context, method, endpoint string, body interface{}, dest interface{}) error {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	headers := c.buildHeaders(nil)

	resp, err := c.httpClient.Do(ctx, method, endpoint, reqBody, headers)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debugf("%s %s → %d (%d bytes)", method, endpoint, resp.StatusCode, len(respBody))

	// Check for API errors
	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if jsonErr := json.Unmarshal(respBody, apiErr); jsonErr != nil {
			apiErr.Detail = string(respBody)
		}
		return apiErr
	}

	// Unmarshal successful response
	if dest != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// prettyJSON returns a formatted JSON string for logging.
func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data)
	}
	return buf.String()
}
