package mcpclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

type ServerConfig struct {
	Name    string
	Version string
	Host    string
	Port    int
}

func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func ServerConfigFromEnv() ServerConfig {
	return ServerConfig{
		Name:    getEnv("MCP_SERVER_NAME", "mcp-server"),
		Version: getEnv("MCP_SERVER_VERSION", "1.0.0"),
		Host:    getEnv("MCP_HOST", ""),
		Port:    getEnvInt("MCP_PORT", 9010),
	}
}

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

func ClientConfigFromEnv() ClientConfig {
	return ClientConfig{
		Name:    getEnv("MCP_CLIENT_NAME", "mcp-client"),
		Version: getEnv("MCP_CLIENT_VERSION", "1.0.0"),
		Host:    getEnv("MCP_HOST", "localhost"),
		Port:    getEnvInt("MCP_PORT", 9010),
		Tool:    getEnv("MCP_TOOL", "echo"),
		Args:    map[string]any{"message": getEnv("MCP_MESSAGE", "Hello MCP!")},
	}
}

type EchoParams struct {
	Message string `json:"message" jsonschema:"Message to echo back"`
}

func echoHandler(ctx context.Context, req *mcp.CallToolRequest, params *EchoParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Echo: %s", params.Message)},
		},
	}, nil, nil
}

func RunServer(cfg ServerConfig) {
	server := NewServer(cfg.Name, cfg.Version)
	AddTool(server, "echo", "Echo back the message", echoHandler)
	log.Fatal(ServeHTTP(server, cfg.Addr()))
}

func RunClient(cfg ClientConfig) {
	ctx := context.Background()
	client := NewClient(cfg.Name, cfg.Version)

	session, err := Connect(ctx, client, cfg.URL())
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	log.Printf("Connected to server at %s", cfg.URL())

	tools, err := ListTools(ctx, session)
	if err != nil {
		log.Fatalf("failed to list tools: %v", err)
	}

	log.Println("Available tools:")
	for _, tool := range tools {
		log.Printf("  - %s: %s", tool.Name, tool.Description)
	}

	result, err := CallTool(ctx, session, cfg.Tool, cfg.Args)
	if err != nil {
		log.Fatalf("failed to call tool: %v", err)
	}

	log.Println("Tool result:")
	PrintToolResult(result)
}
