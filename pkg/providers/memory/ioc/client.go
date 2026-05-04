package iocmemoryprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"go.uber.org/zap"
)

const (
	DefaultKnowledgeMemorySvcRestEndpoint = "http://localhost:9003"
)

var (
	l    *zap.SugaredLogger
	once sync.Once
)

// ErrNotFound is returned when the upstream knowledge memory service responds with 404.
var ErrNotFound = fmt.Errorf("not found")

func getLogger() *zap.SugaredLogger {
	once.Do(func() {
		l = logger.SubPkg("app")
	})
	return l
}

// Client represents a knowledge memory service client that uses schema types
type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

// NewClient creates a new knowledge memory service client with health check
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultKnowledgeMemorySvcRestEndpoint
	}
	// Create HTTP client with required configuration
	config := httpclient.DefaultConfig()
	config.Timeout = 30 * time.Second
	config.MaxRetries = 3

	client := &Client{
		httpClient: httpclient.NewWithConfig(config),
		baseURL:    baseURL,
	}

	// Perform health check
	if err := client.healthCheck(); err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}

	return client, nil
}

// healthCheck performs a health check against the service
func (c *Client) healthCheck() error {
	log := getLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthURL := c.baseURL + "/api/internal/diagnostics/health"
	resp, err := c.httpClient.Get(ctx, healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to call health endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status: %s", resp.Status)
	}

	log.Infof("Health check passed for service at %s", c.baseURL)
	return nil
}

// UpsertKnowledgeGraph sends a POST request to upsert knowledge graph data using schema types
func (c *Client) UpsertKnowledgeGraph(ctx context.Context, request *KnowledgeGraphStoreRequest) (*KnowledgeGraphStoreResponse, error) {
	return c.upsertKnowledgeGraph(ctx, request, false)
}

// UpsertKnowledgeGraphUpdate upserts concepts and relations into an existing graph,
// skipping the cross-request node-id check so relations may reference nodes already
// present in the graph (not just nodes in this batch).
func (c *Client) UpsertKnowledgeGraphUpdate(ctx context.Context, request *KnowledgeGraphStoreRequest) (*KnowledgeGraphStoreResponse, error) {
	return c.upsertKnowledgeGraph(ctx, request, true)
}

func (c *Client) upsertKnowledgeGraph(ctx context.Context, request *KnowledgeGraphStoreRequest, skipNodeIDCheck bool) (*KnowledgeGraphStoreResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(skipNodeIDCheck); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := c.baseURL + "/api/knowledge/graphs"
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("POST request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"knowledge memory service error (%d): %s",
			resp.StatusCode,
			string(body),
		)
	}

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeGraphStoreResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// NewClientForTest creates a Client pointed at baseURL with retries disabled and no health check.
// Intended for use in unit tests only.
func NewClientForTest(baseURL string) *Client {
	config := httpclient.DefaultConfig()
	config.Timeout = 5 * time.Second
	config.MaxRetries = 0
	config.RetryableFunc = func(_ *http.Response, _ error) bool { return false }
	return &Client{
		httpClient: httpclient.NewWithConfig(config),
		baseURL:    baseURL,
	}
}

// QueryKnowledgeGraphPath sends a POST request to query knowledge graph path using schema types
func (c *Client) QueryKnowledgeGraphPath(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graphs/query")
}

// QueryKnowledgeGraphNeighbor sends a POST request to query knowledge graph neighbors using schema types
func (c *Client) QueryKnowledgeGraphNeighbor(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graphs/query")
}

// QueryKnowledgeGraphConcept sends a POST request to query knowledge graph concept using schema types
func (c *Client) QueryKnowledgeGraphConcept(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graphs/query")
}

// DeleteKnowledgeGraph sends a DELETE request to delete knowledge graph data using schema types
func (c *Client) DeleteKnowledgeGraph(ctx context.Context, request *KnowledgeGraphDeleteRequest) (*KnowledgeGraphDeleteResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make DELETE request
	url := c.baseURL + "/api/knowledge/graphs"
	resp, err := c.httpClient.Delete(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("DELETE request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeGraphDeleteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// executeQuery is a helper method to execute query requests
func (c *Client) executeQuery(ctx context.Context, request *KnowledgeGraphQueryRequest, endpoint string) (*KnowledgeGraphQueryResponse, error) {
	log := getLogger()

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := c.baseURL + endpoint
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("POST request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"knowledge memory service error (%d): %s",
			resp.StatusCode,
			string(body),
		)
	}

	// Pretty print JSON response
	c.prettyPrintJSON(body, "debug")

	// Parse response
	var response KnowledgeGraphQueryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// prettyPrintJSON logs JSON in a formatted way.
// An optional log level may be passed as the second argument ("debug", "warn", "error").
// Defaults to "info" if omitted or unrecognised.
func (c *Client) prettyPrintJSON(data []byte, level ...string) {
	log := getLogger()

	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			lvl := "info"
			if len(level) > 0 && level[0] != "" {
				lvl = level[0]
			}
			switch lvl {
			case "debug":
				log.Debugf("Pretty JSON:\n%s", string(formatted))
			case "warn":
				log.Warnf("Pretty JSON:\n%s", string(formatted))
			case "error":
				log.Errorf("Pretty JSON:\n%s", string(formatted))
			default:
				log.Infof("Pretty JSON:\n%s", string(formatted))
			}
		}
	}
}

// OnboardKnowledgeVectorStore sends a POST request to onboard knowledge vector store using schema types
func (c *Client) OnboardKnowledgeVectorStore(ctx context.Context, request *KnowledgeVectorStoreOnboardRequest) (*KnowledgeVectorStoreOnboardResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := c.baseURL + "/api/knowledge/vectors/stores/" + request.MasID
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("POST request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeVectorStoreOnboardResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// UpsertKnowledgeVectors sends a POST request to upsert knowledge vectors using schema types
func (c *Client) UpsertKnowledgeVectors(ctx context.Context, request *KnowledgeVectorStoreRequest) (*KnowledgeVectorStoreResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := c.baseURL + "/api/knowledge/vectors"
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("POST request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeVectorStoreResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// QueryKnowledgeVectors sends a POST request to query knowledge vectors using schema types
func (c *Client) QueryKnowledgeVectors(ctx context.Context, request *KnowledgeVectorQueryRequest) (*KnowledgeVectorQueryResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := c.baseURL + "/api/knowledge/vectors/query"
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("POST request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeVectorQueryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// DeleteKnowledgeVectors sends a DELETE request to delete knowledge vectors using schema types
func (c *Client) DeleteKnowledgeVectors(ctx context.Context, request *KnowledgeVectorDeleteRequest) (*KnowledgeVectorDeleteResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make DELETE request
	url := c.baseURL + "/api/knowledge/vectors"
	resp, err := c.httpClient.Delete(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("DELETE request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeVectorDeleteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// DeleteKnowledgeVectorStore sends a DELETE request to delete knowledge vector store using schema types
func (c *Client) DeleteKnowledgeVectorStore(ctx context.Context, request *KnowledgeVectorStoreOnboardDeleteRequest) (*KnowledgeVectorStoreOnboardDeleteResponse, error) {
	log := getLogger()

	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make DELETE request
	url := c.baseURL + "/api/internal/knowledge/vectors/stores/" + request.MasID
	resp, err := c.httpClient.Delete(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	log.Infof("DELETE request to %s completed", url)
	log.Infof("Response status: %s", resp.Status)
	log.Debugf("Response headers: %v", resp.Header)
	log.Debugf("Response body: %s", string(body))

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeVectorStoreOnboardDeleteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check response status
	if response.Status != ResponseStatusSuccess {
		return &response, fmt.Errorf("operation failed with status: %s", response.Status)
	}

	return &response, nil
}

// SimilaritySearchVectors sends a POST request to search for similar document embeddings.
// Set includeEmbeddings to true to include raw embedding vectors in the response (debug only).
func (c *Client) SimilaritySearchVectors(ctx context.Context, request *KnowledgeVectorSimilaritySearchRequest, includeEmbeddings bool) (*KnowledgeVectorSimilaritySearchResponse, error) {
	log := getLogger()

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := c.baseURL + "/api/knowledge/vectors/query/similarity"
	if includeEmbeddings {
		url += "?include_embeddings=true"
	}
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Infof("POST request to %s completed with status %s", url, resp.Status)
	log.Debugf("Response body: %s", string(body))

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("knowledge memory service error (%d): %s", resp.StatusCode, string(body))
	}

	var response KnowledgeVectorSimilaritySearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// SimilaritySearchConcepts sends a POST request to search for similar concepts by embedding vector.
// Set includeEmbeddings to true to include raw embedding vectors in the response (debug only).
func (c *Client) SimilaritySearchConcepts(ctx context.Context, request *KnowledgeGraphSimilaritySearchRequest, includeEmbeddings bool) (*KnowledgeGraphSimilaritySearchResponse, error) {
	log := getLogger()

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := c.baseURL + "/api/knowledge/graphs/query/similarity"
	if includeEmbeddings {
		url += "?include_embeddings=true"
	}
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Infof("POST request to %s completed with status %s", url, resp.Status)
	log.Debugf("Response body: %s", string(body))

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("knowledge memory service error (%d): %s", resp.StatusCode, string(body))
	}

	var response KnowledgeGraphSimilaritySearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// FetchKnowledgeGraph fetches all nodes and edges for a given MAS from the knowledge memory service.
// Returns the raw response body to be passed through to callers.
func (c *Client) FetchKnowledgeGraph(ctx context.Context, masID string) ([]byte, int, error) {
	log := getLogger()

	url := fmt.Sprintf("%s/api/knowledge/graphs/query", c.baseURL)
	payload := map[string]interface{}{
		"mas_id": masID,
		"query_criteria": map[string]string{
			"query_type": "full_graph",
		},
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := c.httpClient.Post(ctx, url, jsonData, headers)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to call knowledge graph endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Infof("POST %s completed with status %s", url, resp.Status)
	log.Debugf("Response body: %s", string(body))

	return body, resp.StatusCode, nil
}

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
