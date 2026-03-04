// Package graphiti provides a simple HTTP proxy client for Graphiti knowledge graph operations.
//
// The ProxyClient is designed to:
// 1. Forward generic HTTP requests (for multi-tenant handler use)
// 2. Provide typed convenience methods (for direct API usage)
// 3. Handle timeouts and error responses consistently
package graphiti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		l = logger.SubPkg("graphiti")
	})
	return l
}

// ProxyClientConfig holds configuration for the proxy client.
type ProxyClientConfig struct {
	Timeout time.Duration // Request timeout (default: 5 minutes for LLM operations)
}

// DefaultProxyClientConfig returns default config.
// Timeout: 5 minutes (Graphiti operations involve multiple LLM calls)
// No retries: avoids duplicate POST requests
func DefaultProxyClientConfig() *ProxyClientConfig {
	return &ProxyClientConfig{
		Timeout: 5 * time.Minute,
	}
}

// ProxyClient proxies HTTP requests to a Graphiti server.
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
	httpCfg.MaxRetries = 0

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

// ForwardRequest proxies an HTTP request to a Graphiti server.
// Used by the handler for multi-tenant routing with dynamic URLs.
func (c *ProxyClient) ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*ProxyResponse, error) {
	log := getLogger()

	method = strings.ToUpper(strings.TrimSpace(method))

	if method == "" || targetURL == "" {
		return nil, fmt.Errorf("method and targetURL are required")
	}
	if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	// SECURITY: Strip any user-provided Authorization header
	delete(headers, "Authorization")

	if _, hasContentType := headers["Content-Type"]; !hasContentType && body != nil {
		headers["Content-Type"] = "application/json"
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

	log.Infof("graphiti server responded: status=%d, bytes=%d", resp.StatusCode, len(respBody))

	var respJSON map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &respJSON); err != nil {
			respJSON = map[string]interface{}{"raw": string(respBody)}
		}
	}

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
// ──────────────────────────────────────────────────────────

// HealthCheck checks if the Graphiti server is healthy.
// GET {baseURL}/healthcheck
func (c *ProxyClient) HealthCheck(ctx context.Context, baseURL string) (*HealthCheckResponse, error) {
	var resp HealthCheckResponse
	if err := c.doJSON(ctx, "GET", baseURL+"/healthcheck", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddMessages ingests conversational messages into the knowledge graph.
// POST {baseURL}/messages
func (c *ProxyClient) AddMessages(ctx context.Context, baseURL string, req *AddMessagesRequest) (*Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp Result
	if err := c.doJSON(ctx, "POST", baseURL+"/messages", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddEntityNode saves an entity node to the knowledge graph.
// POST {baseURL}/entity-node
func (c *ProxyClient) AddEntityNode(ctx context.Context, baseURL string, req *AddEntityNodeRequest) (map[string]interface{}, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := c.doJSON(ctx, "POST", baseURL+"/entity-node", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Search performs a semantic search across the knowledge graph.
// POST {baseURL}/search
func (c *ProxyClient) Search(ctx context.Context, baseURL string, req *SearchQuery) (*SearchResults, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp SearchResults
	if err := c.doJSON(ctx, "POST", baseURL+"/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetMemory retrieves facts from the knowledge graph based on conversation messages.
// POST {baseURL}/get-memory
func (c *ProxyClient) GetMemory(ctx context.Context, baseURL string, req *GetMemoryRequest) (*GetMemoryResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var resp GetMemoryResponse
	if err := c.doJSON(ctx, "POST", baseURL+"/get-memory", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEpisodes retrieves recent episodes for a group.
// GET {baseURL}/episodes/{group_id}?last_n=N
func (c *ProxyClient) GetEpisodes(ctx context.Context, baseURL, groupID string, lastN int) ([]interface{}, error) {
	if groupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}

	endpoint := fmt.Sprintf("%s/episodes/%s?last_n=%d", baseURL, groupID, lastN)

	var resp []interface{}
	if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetEntityEdge retrieves a specific entity edge by UUID.
// GET {baseURL}/entity-edge/{uuid}
func (c *ProxyClient) GetEntityEdge(ctx context.Context, baseURL, uuid string) (*FactResult, error) {
	if uuid == "" {
		return nil, fmt.Errorf("uuid is required")
	}

	var resp FactResult
	if err := c.doJSON(ctx, "GET", baseURL+"/entity-edge/"+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteEntityEdge deletes an entity edge by UUID.
// DELETE {baseURL}/entity-edge/{uuid}
func (c *ProxyClient) DeleteEntityEdge(ctx context.Context, baseURL, uuid string) (*Result, error) {
	if uuid == "" {
		return nil, fmt.Errorf("uuid is required")
	}

	var resp Result
	if err := c.doJSON(ctx, "DELETE", baseURL+"/entity-edge/"+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteGroup deletes all data for a group.
// DELETE {baseURL}/group/{group_id}
func (c *ProxyClient) DeleteGroup(ctx context.Context, baseURL, groupID string) (*Result, error) {
	if groupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}

	var resp Result
	if err := c.doJSON(ctx, "DELETE", baseURL+"/group/"+groupID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteEpisode deletes an episode by UUID.
// DELETE {baseURL}/episode/{uuid}
func (c *ProxyClient) DeleteEpisode(ctx context.Context, baseURL, uuid string) (*Result, error) {
	if uuid == "" {
		return nil, fmt.Errorf("uuid is required")
	}

	var resp Result
	if err := c.doJSON(ctx, "DELETE", baseURL+"/episode/"+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Clear removes all data from the knowledge graph.
// POST {baseURL}/clear
func (c *ProxyClient) Clear(ctx context.Context, baseURL string) (*Result, error) {
	var resp Result
	if err := c.doJSON(ctx, "POST", baseURL+"/clear", nil, &resp); err != nil {
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

	headers := make(map[string]string)
	if bodyBytes != nil {
		headers["Content-Type"] = "application/json"
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

	log.Debugf("%s %s -> %d (%d bytes)", method, endpoint, resp.StatusCode, len(respBody))

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Detail = string(respBody)
		}
		return apiErr
	}

	if respDest != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, respDest); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
