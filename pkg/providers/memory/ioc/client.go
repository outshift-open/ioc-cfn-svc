package iocmemoryprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	c.prettyPrintJSON(body)

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

// prettyPrintJSON logs JSON in a formatted way
func (c *Client) prettyPrintJSON(data []byte) {
	log := getLogger()

	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			log.Infof("Pretty JSON:\n%s", string(formatted))
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("knowledge memory service error (%d): %s", resp.StatusCode, string(body))
	}

	var response KnowledgeGraphSimilaritySearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
