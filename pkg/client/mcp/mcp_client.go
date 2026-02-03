// Package mcpclient provides MCP client and server utilities.
package mcpclient

import (
	"context"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var log = logger.SubPkg("mcp")

// NewClient creates an MCP client.
func NewClient(name, version string) *mcp.Client {
	return mcp.NewClient(&mcp.Implementation{Name: name, Version: version}, nil)
}

// Connect establishes a connection to an MCP server.
func Connect(ctx context.Context, client *mcp.Client, url string) (*mcp.ClientSession, error) {
	return client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
}

// ListTools returns available tools from the server.
func ListTools(ctx context.Context, session *mcp.ClientSession) ([]*mcp.Tool, error) {
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the server.
func CallTool(ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
}

// PrintToolResult logs text content from a tool result.
func PrintToolResult(result *mcp.CallToolResult) {
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			log.Infof("%s", textContent.Text)
		}
	}
}
