// Package mem0 provides a simple HTTP proxy client for memory operations.
//
// The ProxyClient is designed to:
// 1. Forward generic HTTP requests (for multi-tenant handler use)
// 2. Provide typed convenience methods (for direct API usage)
// 3. Handle authentication, timeouts, and error responses consistently
package mem0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"go.uber.org/zap"
)

var (
	l    *zap.SugaredLogger
	once sync.Once
)

func getLogger() *zap.SugaredLogger {
	once.Do(func() {
		l = logger.SubPkg("mem0")
	})
	return l
}

// ProxyClientConfig holds configuration for the proxy client.
type ProxyClientConfig struct {
	APIKey  string        // Optional - only needed if memory provider requires auth
	Timeout time.Duration // Request timeout (default: 5 minutes for LLM operations)
}

// DefaultProxyClientConfig returns default config.
// Timeout: 5 minutes (memory operations involve multiple LLM calls)
// No retries: avoids duplicate POST requests
func DefaultProxyClientConfig() *ProxyClientConfig {
	return &ProxyClientConfig{
		Timeout: 5 * time.Minute,
	}
}

// ProxyClient proxies HTTP requests to memory providers with optional authentication.
type ProxyClient struct {
	httpClient *httpclient.Client
	cfg        *ProxyClientConfig
}

// NewProxyClient creates a new proxy client.
func NewProxyClient(cfg *ProxyClientConfig) *ProxyClient {
	if cfg == nil {
		cfg = DefaultProxyClientConfig()
	}

	httpCfg := httpclient.DefaultConfig()
	httpCfg.Timeout = cfg.Timeout
	httpCfg.MaxRetries = 0 // No retries - prevents duplicate POST requests

	// Never retry to avoid duplicate operations
	httpCfg.RetryableFunc = func(resp *http.Response, err error) bool {
		return false
	}

	return &ProxyClient{
		httpClient: httpclient.NewWithConfig(httpCfg),
		cfg:        cfg,
	}
}

// ──────────────────────────────────────────────────────────
// Generic HTTP Proxy
// ──────────────────────────────────────────────────────────

// ForwardRequest proxies an HTTP request to a memory provider.
// Used by the handler for multi-tenant routing with dynamic URLs.
func (c *ProxyClient) ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*ProxyResponse, error) {
	log := getLogger()

	method = strings.ToUpper(strings.TrimSpace(method))

	// Validate inputs
	if method == "" || targetURL == "" {
		return nil, fmt.Errorf("method and targetURL are required")
	}
	if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	// Add auth and content-type headers if needed
	if headers == nil {
		headers = make(map[string]string)
	}

	// SECURITY: Always strip any user-provided Authorization header
	// to prevent auth bypass/injection attacks
	delete(headers, "Authorization")

	// Only set Content-Type if caller hasn't provided it and we have a body
	if _, hasContentType := headers["Content-Type"]; !hasContentType && body != nil {
		headers["Content-Type"] = "application/json"
	}

	// SECURITY: Always use server-configured APIKey, never trust user input
	if c.cfg.APIKey != "" {
		headers["Authorization"] = "Token " + c.cfg.APIKey
	}

	log.Infof("proxying %s request to: %s", method, targetURL)

	resp, err := c.httpClient.Do(ctx, method, targetURL, body, headers)
	if err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Infof("memory provider responded: status=%d, bytes=%d", resp.StatusCode, len(respBody))

	// Parse JSON response
	var respJSON map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &respJSON); err != nil {
			// Not valid JSON - wrap as raw text
			respJSON = map[string]interface{}{"raw": string(respBody)}
		}
	}

	// Extract response headers (take first value)
	respHeaders := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			respHeaders[k] = vals[0]
		}
	}

	return &ProxyResponse{
		HTTPStatus:       resp.StatusCode,
		HTTPHeaders:      respHeaders,
		HTTPResponseBody: respJSON,
	}, nil
}

// ──────────────────────────────────────────────────────────
// Typed Convenience Methods
// These wrap the generic proxy with type-safe request/response structs.
// Used for direct API calls (e.g., test clients).
// ──────────────────────────────────────────────────────────

// CreateMemory stores a new memory.
// POST {baseURL}/api/v1/memories/
func (c *ProxyClient) CreateMemory(ctx context.Context, baseURL string, req *CreateMemoryRequest) (*CreateMemoryResponse, error) {
	log := getLogger()

	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp CreateMemoryResponse
	if err := c.doJSON(ctx, "POST", baseURL+"/api/v1/memories/", req, &resp); err != nil {
		return nil, err
	}

	// OpenMemory returns null for duplicate memories
	if resp.ID == "" {
		log.Infof("create memory returned null (likely duplicate)")
		return nil, nil
	}
	return &resp, nil
}

// ListMemories retrieves all memories for a user.
// GET {baseURL}/api/v1/memories/?user_id=xxx
func (c *ProxyClient) ListMemories(ctx context.Context, baseURL string, req *ListMemoriesRequest) (*ListMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("user_id", req.UserID)
	if req.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", req.Page))
	}
	if req.Size > 0 {
		params.Set("size", fmt.Sprintf("%d", req.Size))
	}

	endpoint := baseURL + "/api/v1/memories/?" + params.Encode()

	var resp ListMemoriesResponse
	if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetMemory retrieves a single memory by ID.
// GET {baseURL}/api/v1/memories/{memory_id}
func (c *ProxyClient) GetMemory(ctx context.Context, baseURL, memoryID string) (*GetMemoryResponse, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}

	var resp GetMemoryResponse
	if err := c.doJSON(ctx, "GET", baseURL+"/api/v1/memories/"+memoryID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateMemory updates a memory's content.
// PUT {baseURL}/api/v1/memories/{memory_id}
func (c *ProxyClient) UpdateMemory(ctx context.Context, baseURL, memoryID string, req *UpdateMemoryRequest) (*UpdateMemoryResponse, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp UpdateMemoryResponse
	if err := c.doJSON(ctx, "PUT", baseURL+"/api/v1/memories/"+memoryID, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteMemories deletes one or more memories.
// DELETE {baseURL}/api/v1/memories/
func (c *ProxyClient) DeleteMemories(ctx context.Context, baseURL string, req *DeleteMemoriesRequest) (*DeleteMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp DeleteMemoriesResponse
	if err := c.doJSON(ctx, "DELETE", baseURL+"/api/v1/memories/", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FilterMemories performs semantic search across memories.
// POST {baseURL}/api/v1/memories/filter
func (c *ProxyClient) FilterMemories(ctx context.Context, baseURL string, req *FilterMemoriesRequest) (*FilterMemoriesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp FilterMemoriesResponse
	if err := c.doJSON(ctx, "POST", baseURL+"/api/v1/memories/filter", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCategories retrieves memory categories for a user.
// GET {baseURL}/api/v1/memories/categories?user_id=xxx
func (c *ProxyClient) GetCategories(ctx context.Context, baseURL, userID string) (*CategoriesResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	endpoint := baseURL + "/api/v1/memories/categories?user_id=" + url.QueryEscape(userID)

	var resp CategoriesResponse
	if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRelatedMemories retrieves memories related to a given memory.
// GET {baseURL}/api/v1/memories/{memory_id}/related?user_id=xxx
func (c *ProxyClient) GetRelatedMemories(ctx context.Context, baseURL, memoryID, userID string) (*RelatedMemoriesResponse, error) {
	if memoryID == "" || userID == "" {
		return nil, fmt.Errorf("memory_id and user_id are required")
	}

	endpoint := baseURL + "/api/v1/memories/" + memoryID + "/related?user_id=" + url.QueryEscape(userID)

	var resp RelatedMemoriesResponse
	if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStats retrieves memory statistics for a user.
// GET {baseURL}/api/v1/stats/?user_id=xxx
func (c *ProxyClient) GetStats(ctx context.Context, baseURL, userID string) (*StatsResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	endpoint := baseURL + "/api/v1/stats/?user_id=" + url.QueryEscape(userID)

	var resp StatsResponse
	if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ──────────────────────────────────────────────────────────
// Internal helper for typed methods
// ──────────────────────────────────────────────────────────

// doJSON marshals request, sends it, and unmarshals response.
func (c *ProxyClient) doJSON(ctx context.Context, method, endpoint string, reqBody, respDest interface{}) error {
	log := getLogger()

	var bodyBytes []byte
	var err error

	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	// Build headers with auth
	headers := make(map[string]string)
	if bodyBytes != nil {
		headers["Content-Type"] = "application/json"
	}
	// Only set Authorization if we have an API key (typed methods always use config auth)
	if c.cfg.APIKey != "" {
		headers["Authorization"] = "Token " + c.cfg.APIKey
	}

	resp, err := c.httpClient.Do(ctx, method, endpoint, bodyBytes, headers)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	log.Debugf("%s %s → %d (%d bytes)", method, endpoint, resp.StatusCode, len(respBody))

	// Handle API errors
	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Detail = string(respBody)
		}
		return apiErr
	}

	// Unmarshal success response
	if respDest != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, respDest); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
