package mcpclient

import (
	"context"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewClient(name, version string) *mcp.Client {
	return mcp.NewClient(&mcp.Implementation{Name: name, Version: version}, nil)
}

func Connect(ctx context.Context, client *mcp.Client, command string, args ...string) (*mcp.Session, error) {
	transport := &mcp.CommandTransport{Command: exec.Command(command, args...)}
	return client.Connect(ctx, transport, nil)
}

func CallTool(ctx context.Context, session *mcp.Session, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
}
