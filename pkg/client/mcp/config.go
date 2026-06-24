// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/outshift-open/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/outshift-open/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	TOOL_NAME_RETAIN = "retain"
	TOOL_NAME_RECALL = "recall"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// McpServiceSharedMemInterface defines the interface for MCP server shared memory operations.
type McpServiceSharedMemInterface interface {
	CreateOrUpdateSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.CreateOrUpdateRequest) (*sharedmemory.CreateOrUpdateResponse, error)
	FetchSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.QueryRequest) (*sharedmemory.QueryResponse, error)
}

// ServerConfig holds MCP server settings.
type ServerConfig struct {
	Name         string
	Version      string
	Host         string
	Port         int
	SharedMemSvc McpServiceSharedMemInterface // service instance for accessing business logic
}

// CreateOrUpdateSharedMemoriesParams defines the parameters for the MCP tool.
type CreateOrUpdateSharedMemoriesParams struct {
	WorkspaceID string               `json:"workspace_id" jsonschema:"Workspace identifier"`
	MasID       string               `json:"mas_id" jsonschema:"Multi-agent system identifier"`
	Header      *sharedmemory.Header `json:"header,omitempty"`
	RequestId   *string              `json:"request_id,omitempty" jsonschema:"Optional request ID"`
	Payload     interface{}          `json:"payload"`
}

// FetchSharedMemoriesParams defines the parameters for the recall MCP tool.
type FetchSharedMemoriesParams struct {
	WorkspaceID       string                   `json:"workspace_id" jsonschema:"Workspace identifier"`
	MasID             string                   `json:"mas_id" jsonschema:"Multi-agent system identifier"`
	Header            *sharedmemory.Header     `json:"header,omitempty"`
	RequestId         *string                  `json:"request_id,omitempty" jsonschema:"Optional request ID"`
	SearchStrategy    *string                  `json:"search_strategy,omitempty" jsonschema:"Search strategy"`
	Intent            *string                  `json:"intent" jsonschema:"User intent or natural-language query"`
	AdditionalContext []map[string]interface{} `json:"additional_context,omitempty" jsonschema:"Optional contextual information"`
}

func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ServerConfigFromEnv loads server config from environment variables.
func ServerConfigFromEnv() ServerConfig {
	return ServerConfig{
		Name:    getEnv("MCP_SERVER_NAME", "mcp-server"),
		Version: getEnv("MCP_SERVER_VERSION", "1.0.0"),
		Host:    getEnv("MCP_HOST", ""),
		Port:    getEnvInt("MCP_PORT", 9005),
	}
}

// ServerConfig loads server config from environment variables and uses the provided services.
func ServerConfigs(sharedMemSvc McpServiceSharedMemInterface) ServerConfig {
	config := ServerConfigFromEnv()
	config.SharedMemSvc = sharedMemSvc
	return config
}

// ClientConfig holds MCP client settings.
type ClientConfig struct {
	Name    string
	Version string
	Host    string
	Port    int
	Tool    string
	Args    map[string]any
}

func (c ClientConfig) URL() string {
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// ClientConfigFromEnv loads client config from environment variables.
func ClientConfigFromEnv() ClientConfig {
	return ClientConfig{
		Name:    getEnv("MCP_CLIENT_NAME", "mcp-client"),
		Version: getEnv("MCP_CLIENT_VERSION", "1.0.0"),
		Host:    getEnv("MCP_HOST", "localhost"),
		Port:    getEnvInt("MCP_PORT", 9001),
		Tool:    getEnv("MCP_TOOL", "echo"),
		Args:    map[string]any{"message": getEnv("MCP_MESSAGE", "Hello MCP!")},
	}
}

type EchoParams struct {
	Message string `json:"message" jsonschema:"Intent to echo back"`
}

func echoHandler(ctx context.Context, req *mcp.CallToolRequest, params *EchoParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Echo: %s", params.Message)},
		},
	}, nil, nil
}

var sharedMemoryService McpServiceSharedMemInterface

func createOrUpdateSharedMemoriesToolHandler(ctx context.Context, req *mcp.CallToolRequest, params *CreateOrUpdateSharedMemoriesParams) (*mcp.CallToolResult, any, error) {
	log := getLogger()
	log.Infof("createOrUpdateSharedMemoriesToolHandler called, sharedMemoryService is nil: %v", sharedMemoryService == nil)

	if sharedMemoryService == nil {
		log.Infof("Returning error: shared memory service not initialized")
		return nil, nil, fmt.Errorf("shared memory service not initialized")
	}

	log.Infof("Proceeding with tool execution - sharedMemoryService is not nil")

	// Convert MCP params to internal request format
	// Convert interface{} payload to ExtractionPayload
	payloadBytes, err := json.Marshal(params.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload cognitionagentclient.ExtractionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Ensure we have a request ID
	requestId := params.RequestId
	if requestId == nil {
		uuid := "mcp-request-" + fmt.Sprintf("%d", time.Now().UnixNano())
		requestId = &uuid
	}

	request := sharedmemory.CreateOrUpdateRequest{
		Header:    params.Header,
		RequestId: requestId,
		Payload:   payload,
	}

	// Call core business logic
	response, err := sharedMemoryService.CreateOrUpdateSharedMemoriesCore(
		ctx,
		params.WorkspaceID,
		params.MasID,
		request,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create or update shared memories: %w", err)
	}

	// Convert response to JSON for MCP
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(responseJSON)},
		},
	}, response, nil
}

func recallToolHandler(ctx context.Context, req *mcp.CallToolRequest, params *FetchSharedMemoriesParams) (*mcp.CallToolResult, any, error) {
	log := getLogger()
	log.Infof("recallToolHandler called, sharedMemoryService is nil: %v", sharedMemoryService == nil)

	if sharedMemoryService == nil {
		log.Infof("Returning error: shared memory service not initialized")
		return nil, nil, fmt.Errorf("shared memory service not initialized")
	}

	log.Infof("Proceeding with recall tool execution - sharedMemoryService is not nil")

	// Ensure we have a request ID
	requestId := params.RequestId
	if requestId == nil {
		uuid := "mcp-recall-request-" + fmt.Sprintf("%d", time.Now().UnixNano())
		requestId = &uuid
	}

	// Create QueryRequest from params
	queryRequest := sharedmemory.QueryRequest{
		Header:            params.Header,
		RequestId:         requestId,
		SearchStrategy:    params.SearchStrategy,
		Intent:            params.Intent,
		AdditionalContext: params.AdditionalContext,
	}

	// Call core business logic
	response, err := sharedMemoryService.FetchSharedMemoriesCore(
		ctx,
		params.WorkspaceID,
		params.MasID,
		queryRequest,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch shared memories: %w", err)
	}

	// Convert response to JSON for MCP
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(responseJSON)},
		},
	}, response, nil
}

// RunServer starts an MCP server with the tools.
func RunServer(cfg ServerConfig) {
	log := getLogger()
	server := NewServer(cfg.Name, cfg.Version)

	// Set the service instance
	sharedMemoryService = cfg.SharedMemSvc
	log.Infof("Set shared memory service instance: %v", sharedMemoryService != nil)

	// Register tools
	log.Info("Registering MCP tools...")
	AddTool(server, "echo", "Echo back the message", echoHandler)
	log.Infof("Registered tool: %s", "echo")

	AddTool(server, TOOL_NAME_RETAIN, "Retain shared memories", createOrUpdateSharedMemoriesToolHandler)
	log.Infof("Registered tool: %s", TOOL_NAME_RETAIN)

	AddTool(server, TOOL_NAME_RECALL, "Recall shared memories", recallToolHandler)
	log.Infof("Registered tool: %s", TOOL_NAME_RECALL)

	if err := ServeHTTP(server, cfg.Addr()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// RunClient connects to an MCP server and calls a tool.
func RunClient(cfg ClientConfig) {
	log := getLogger()

	ctx := context.Background()
	client := NewClient(cfg.Name, cfg.Version)

	session, err := Connect(ctx, client, cfg.URL())
	if err != nil {
		log.Errorf("failed to connect: %v", err)
		return
	}
	defer session.Close()

	log.Infof("Connected to server at %s", cfg.URL())

	tools, err := ListTools(ctx, session)
	if err != nil {
		log.Errorf("failed to list tools: %v", err)
		return
	}

	log.Info("Available tools:")
	for _, tool := range tools {
		log.Infof("  - %s: %s", tool.Name, tool.Description)
	}

	result, err := CallTool(ctx, session, cfg.Tool, cfg.Args)
	if err != nil {
		log.Errorf("failed to call tool: %v", err)
		return
	}

	log.Info("Tool result:")
	PrintToolResult(result)
}
