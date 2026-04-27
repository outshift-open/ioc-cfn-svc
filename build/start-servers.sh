#!/bin/sh
# Script to start ioc-cfn-svc with both HTTP and MCP servers

echo "Starting ioc-cfn-svc with dual server mode..."

# Start the service with both HTTP and MCP servers
echo "Starting service with HTTP server on port 9002 and MCP server on port 9001..."
./cfn-svc -port=9002 -mcp_port=9001 &
SERVICE_PID=$!

echo "Service PID: $SERVICE_PID"

# Function to cleanup on exit
cleanup() {
    echo "Shutting down service..."
    kill $SERVICE_PID 2>/dev/null
    wait $SERVICE_PID 2>/dev/null
    echo "Shutdown complete"
    exit 0
}

# Trap signals for graceful shutdown
trap cleanup SIGTERM SIGINT

# Wait for the service process
wait $SERVICE_PID
