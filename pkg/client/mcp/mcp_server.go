package mcpclient

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewServer(name, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
	server.AddReceivingMiddleware(LoggingMiddleware())
	return server
}

func LoggingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()
			sessionID := req.GetSession().ID()

			log.Printf("[REQUEST] Session: %s | Method: %s", sessionID, method)

			result, err := next(ctx, method, req)

			duration := time.Since(start)
			if err != nil {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: ERROR | Duration: %v | Error: %v",
					sessionID, method, duration, err)
			} else {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: OK | Duration: %v",
					sessionID, method, duration)
			}

			return result, err
		}
	}
}

func AddTool[T any](server *mcp.Server, name, description string, handler func(ctx context.Context, req *mcp.CallToolRequest, params *T) (*mcp.CallToolResult, any, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, handler)
}

func ServeHTTP(server *mcp.Server, addr string) error {
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)
	log.Printf("MCP server listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}

func ServeStdio(ctx context.Context, server *mcp.Server) error {
	return server.Run(ctx, &mcp.StdioTransport{})
}
