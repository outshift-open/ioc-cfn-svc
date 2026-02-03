package mcpclient

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestClientServerCommunication(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	// Start server
	server := NewServer("test-server", "1.0.0")
	AddTool(server, "echo", "Echo back the message", echoHandler)

	go func() {
		ServeHTTP(server, addr)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

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
