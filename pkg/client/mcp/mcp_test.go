package mcpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestClientServerCommunication(t *testing.T) {
	// Reserve port by keeping listener open
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	// Ensure listener is always closed, even on test failure
	t.Cleanup(func() {
		if listener != nil {
			listener.Close()
		}
	})

	// Start server with shutdown capability
	server := NewServer("test-server", "1.0.0")
	AddTool(server, "echo", "Echo back the message", echoHandler)

	httpServer, shutdown := ServeHTTPWithShutdown(server, addr)

	// Ensure server is stopped after test completes (success or failure)
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Logf("Error shutting down server: %v", err)
		}
	})

	// Channel to capture server startup errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		} else {
			serverErr <- nil
		}
	}()

	// Close reservation listener now that server is starting
	listener.Close()
	listener = nil // Prevent double-close in cleanup

	// Wait for server to start and check for errors
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("Server failed to start: %v", err)
		}
	default:
		// Server is still starting, which is fine
	}

	// Create client and connect
	ctx := context.Background()
	client := NewClient("test-client", "1.0.0")

	session, err := Connect(ctx, client, url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Logf("Connected to server at %s", url)

	// List tools
	tools, err := ListTools(ctx, session)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Fatalf("expected tool name 'echo', got '%s'", tools[0].Name)
	}
	t.Logf("Tools: %v", tools[0].Name)

	// Call echo tool
	result, err := CallTool(ctx, session, "echo", map[string]any{"message": "Hello MCP from tests!"})
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}

	expected := "Echo: Hello MCP from tests!"
	if textContent.Text != expected {
		t.Fatalf("expected '%s', got '%s'", expected, textContent.Text)
	}

	t.Logf("Result: %s", textContent.Text)
}

func TestRetainTool_Found(t *testing.T) {
	// Reserve port by keeping listener open
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("localhost:%d", port)

	// Ensure listener is always closed, even on test failure
	t.Cleanup(func() {
		if listener != nil {
			listener.Close()
		}
	})

	ctx := context.Background()

	// Reset global state to ensure test isolation
	sharedMemoryService = nil

	// Create and start server with shutdown capability
	server := NewServer("test-server", "1.0.0")
	sharedMemoryService = nil // Set the global service to nil
	AddTool(server, "echo", "Echo back the message", echoHandler)
	AddTool(server, TOOL_NAME_RETAIN, "Retain shared memories", createOrUpdateSharedMemoriesToolHandler)

	httpServer, shutdown := ServeHTTPWithShutdown(server, addr)

	// Ensure server is stopped after test completes (success or failure)
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Logf("Error shutting down server: %v", err)
		}
	})

	// Channel to capture server startup errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		} else {
			serverErr <- nil
		}
	}()

	// Close reservation listener now that server is starting
	listener.Close()
	listener = nil // Prevent double-close in cleanup

	// Wait for server to start and check for errors
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("Server failed to start: %v", err)
		}
	default:
		// Server is still starting, which is fine
	}

	// Create client and connect
	client := NewClient("test-client", "1.0.0")
	url := fmt.Sprintf("http://%s", addr)
	session, err := Connect(ctx, client, url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Logf("Connected to server at %s", url)

	// List tools to verify our new tool is registered
	tools, err := ListTools(ctx, session)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	// Check that retain tool is registered
	var foundTool bool
	for _, tool := range tools {
		if tool.Name == TOOL_NAME_RETAIN {
			foundTool = true
			break
		}
	}

	if !foundTool {
		t.Fatal("retain tool not found in registered tools")
	}

	t.Logf("Successfully found retain tool in registry - test passed!")
}

func TestRetainTool_ExecutionError(t *testing.T) {
	// Reserve port by keeping listener open
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("localhost:%d", port)

	// Ensure listener is always closed, even on test failure
	t.Cleanup(func() {
		if listener != nil {
			listener.Close()
		}
	})

	ctx := context.Background()

	// Reset global state to ensure test isolation
	sharedMemoryService = nil

	// Create and start server with shutdown capability
	server := NewServer("test-server", "1.0.0")
	sharedMemoryService = nil // Set the global service to nil (will cause execution error)
	AddTool(server, "echo", "Echo back the message", echoHandler)
	AddTool(server, TOOL_NAME_RETAIN, "Retain shared memories", createOrUpdateSharedMemoriesToolHandler)

	httpServer, shutdown := ServeHTTPWithShutdown(server, addr)

	// Ensure server is stopped after test completes (success or failure)
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Logf("Error shutting down server: %v", err)
		}
	})

	// Channel to capture server startup errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		} else {
			serverErr <- nil
		}
	}()

	// Close reservation listener now that server is starting
	listener.Close()
	listener = nil // Prevent double-close in cleanup

	// Wait for server to start and check for errors
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("Server failed to start: %v", err)
		}
	default:
		// Server is still starting, which is fine
	}

	// Create client and connect
	client := NewClient("test-client", "1.0.0")
	url := fmt.Sprintf("http://%s", addr)
	session, err := Connect(ctx, client, url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Logf("Connected to server at %s", url)

	// Test calling the retain tool - this should fail since service is nil
	result, err := CallTool(ctx, session, TOOL_NAME_RETAIN, map[string]any{
		"workspace_id": "test-workspace",
		"mas_id":       "test-mas",
		"payload": map[string]any{
			"metadata": map[string]any{
				"format": "openclaw",
			},
			"data": []map[string]any{
				{
					"schema": "test-schema",
				},
			},
		},
	})

	expectedError := "shared memory service not initialized"

	// Check if we got a Go error from CallTool
	if err != nil {
		if !contains(err.Error(), expectedError) {
			t.Fatalf("expected error containing '%s', got: %v", expectedError, err)
		}
		t.Logf("Retain tool correctly returned Go error: %v", err)
		return
	}

	// If no Go error, check if the error is in the MCP result content
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected either Go error or MCP result with error content, but got neither")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent in MCP result")
	}

	// Check if the content contains the expected error message
	if !contains(textContent.Text, expectedError) {
		t.Fatalf("expected error message '%s' in result content, got: %s", expectedError, textContent.Text)
	}

	t.Logf("Retain tool correctly returned error in MCP result content: %s", textContent.Text)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
