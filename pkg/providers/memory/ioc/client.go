package iocmemoryprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const (
	DefaultKnowledgeMemorySvcRestEndpoint = "http://localhost:8001"
)

var log = logger.SubPkg("app")

// Client represents a knowledge memory service client that uses schema types
type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

// NewClient creates a new knowledge memory service client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultKnowledgeMemorySvcRestEndpoint
	}
	// Create HTTP client with required configuration
	config := httpclient.DefaultConfig()
	config.Timeout = 30 * time.Second
	config.MaxRetries = 3

	return &Client{
		httpClient: httpclient.NewWithConfig(config),
		baseURL:    baseURL,
	}
}

// UpsertKnowledgeGraph sends a POST request to upsert knowledge graph data using schema types
func (c *Client) UpsertKnowledgeGraph(ctx context.Context, request *KnowledgeGraphStoreRequest) (*KnowledgeGraphStoreResponse, error) {
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
	url := c.baseURL + "/api/knowledge/graph"
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
	var response KnowledgeGraphStoreResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// QueryKnowledgeGraphPath sends a POST request to query knowledge graph path using schema types
func (c *Client) QueryKnowledgeGraphPath(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graph/query")
}

// QueryKnowledgeGraphNeighbor sends a POST request to query knowledge graph neighbors using schema types
func (c *Client) QueryKnowledgeGraphNeighbor(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graph/query")
}

// QueryKnowledgeGraphConcept sends a POST request to query knowledge graph concept using schema types
func (c *Client) QueryKnowledgeGraphConcept(ctx context.Context, request *KnowledgeGraphQueryRequest) (*KnowledgeGraphQueryResponse, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return c.executeQuery(ctx, request, "/api/knowledge/graph/query")
}

// DeleteKnowledgeGraph sends a DELETE request to delete knowledge graph data using schema types
func (c *Client) DeleteKnowledgeGraph(ctx context.Context, request *KnowledgeGraphDeleteRequest) (*KnowledgeGraphDeleteResponse, error) {
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
	url := c.baseURL + "/api/knowledge/graph"
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

	return &response, nil
}

// executeQuery is a helper method to execute query requests
func (c *Client) executeQuery(ctx context.Context, request *KnowledgeGraphQueryRequest, endpoint string) (*KnowledgeGraphQueryResponse, error) {
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

	// Pretty print JSON response
	c.prettyPrintJSON(body)

	// Parse response
	var response KnowledgeGraphQueryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// prettyPrintJSON logs JSON in a formatted way
func (c *Client) prettyPrintJSON(data []byte) {
	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			log.Infof("Pretty JSON:\n%s", string(formatted))
		}
	}
}

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
