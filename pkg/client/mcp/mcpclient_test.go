package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This test runs against a real MCP server with real services and a mock database
// To run this test:
// 1. Check the Prerequisites section below
// 2. Run the client:
//    cd /Users/sushroff/Documents/AI/ioc/ioc-cfn-svc
//    go test -v ./pkg/client/mcp -run TestRetainTool

// Prerequisites-
// 1. MCP server (running on port 9002 default if env var MCP_PORT is not set)
// 	 - Env setup-
//    MCP_ENABLED=true
//   - Start the server
//      make run mcp-server
// 2. Cognition Engine service (running on port 9006 default if env var COGNITION_ENGINE_SVC_URL is not set)
// 3. Knowledge memory service (running on port 9003 default if env var KNOWLEDGE_MEMORY_SVC_URL is not set)

// This test verifies that the MCP client can successfully call the retain tool
// and that the tool executes without errors.
func TestRetainTool(t *testing.T) {
	t.Skip("Skipping test - requires external services")

	// Load .env file to pick up environment variables
	_ = godotenv.Load("../../../.env")

	ctx := context.Background()

	// Reset global state to ensure test isolation
	// sharedMemoryService = nil

	mcpServerPort := getEnvInt("MCP_PORT", 9002)
	t.Logf("Using MCP server port: %d", mcpServerPort)
	// Set up environment variables to use real services
	knowledgeMemURL := os.Getenv("KNOWLEDGE_MEMORY_SVC_URL")
	if knowledgeMemURL == "" {
		knowledgeMemURL = "http://localhost:9003" // Default fallback
	}
	cognitionEngineURL := os.Getenv("COGNITION_ENGINE_SVC_URL")
	if cognitionEngineURL == "" {
		cognitionEngineURL = "http://localhost:9006" // Default fallback
	}

	t.Logf("Using Knowledge Memory Service at: %s", knowledgeMemURL)
	t.Logf("Using Cognition Agents Service at: %s", cognitionEngineURL)

	//////////////////////////////////////////////////////

	// Create client and connect
	client := NewClient("mcp-test-client", "1.0.0")
	mcpServerURL := fmt.Sprintf("http://localhost:%d", mcpServerPort)
	session, err := Connect(ctx, client, mcpServerURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Logf("Connected to mcpserver at %s", mcpServerURL)

	// List tools to verify registration
	tools, err := ListTools(ctx, session)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	foundTool := false
	for _, tool := range tools {
		if tool.Name == TOOL_NAME_RETAIN {
			foundTool = true
			break
		}
	}

	if !foundTool {
		t.Fatal("retain tool not found in registered tools")
	}

	t.Logf("Found retain tool in registry")

	// Test calling the tool with mock App instance (should succeed)
	result, err := CallTool(ctx, session, TOOL_NAME_RETAIN, map[string]any{
		"workspace_id": "7f136aa0-143c-46a6-82f2-249eac489e52",
		"mas_id":       "223e4567-e89b-12d3-a456-426614174001",
		"request_id":   "test-request-123",
		"header": map[string]any{
			"agent_id": "test-agent",
		},
		"payload": map[string]any{
			"metadata": map[string]any{
				"format": "openclaw",
			},
			"data": []map[string]any{
				{
					"schema":      "openclaw-conversation-v1",
					"extractedAt": "2026-02-25T20:32:24.376Z",
					"session": map[string]any{
						"agentId":    "main",
						"sessionId":  "906630a9-bf57-48d8-bbae-9d41e7639d29",
						"sessionKey": "agent:main:matrix:channel:!ltghwkqehwwjyjyrhf:local",
						"channel":    "matrix",
						"cwd":        "/home/node/.openclaw/workspace",
					},
					"stats": map[string]any{
						"totalEntries":      8,
						"turns":             1,
						"toolCallCount":     1,
						"thinkingTurnCount": 0,
						"totalCost":         0,
					},
					"turns": []map[string]any{
						{
							"index":      0,
							"timestamp":  "2026-02-25T20:32:14.783Z",
							"model":      "bedrock/global.anthropic.claude-haiku-4-5-20251001-v1:0",
							"stopReason": "stop",
							"usage": map[string]any{
								"input":       9,
								"output":      469,
								"cacheRead":   13531,
								"cacheWrite":  14906,
								"totalTokens": 28915,
								"cost": map[string]any{
									"input":      0,
									"output":     0,
									"cacheRead":  0,
									"cacheWrite": 0,
									"total":      0,
								},
							},
							"userMessage": "Test message for MCP production test",
							"thinking":    nil,
							"toolCalls": []map[string]any{
								{
									"id":   "toolu_test_123",
									"name": "read",
									"input": map[string]any{
										"path": "/test/path",
									},
									"result":  "Test result",
									"isError": false,
								},
							},
							"response": "Test response from agent",
						},
					},
				},
			},
		},
	})

	// This should succeed since we have a mock App instance
	if err != nil {
		t.Fatalf("expected successful tool call, but got error: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content, but got none")
	}

	PrintToolResult(result)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent in result")
	}

	// Verify we got a successful response (not an error message)
	if strings.Contains(textContent.Text, "shared memory service not initialized") {
		t.Fatalf("unexpected error message in successful test: %s", textContent.Text)
	}

	// Parse the JSON response to verify structure
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	// Verify response contains expected fields from a successful operation
	if response["status"] == nil {
		t.Fatal("expected 'status' field in response")
	}

	if response["status"] != "success" {
		t.Fatalf("expected status 'success', got: %v", response["status"])
	}

	t.Logf("Production test successful! Response: %s", textContent.Text)
	t.Logf("Verified successful MCP tool execution with real services")
}
