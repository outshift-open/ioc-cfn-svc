#!/bin/sh
# Script to start both HTTP and MCP servers on different ports

echo "Starting ioc-cfn-svc with dual server mode..."

# Start HTTP server in background
echo "Starting HTTP server on port 9002..."
MCP_ENABLED=false ./cfn-svc -port=9002 &
HTTP_PID=$!

# Start MCP server in background  
echo "Starting MCP server on port 9001..."
MCP_ENABLED=true MCP_PORT=9001 ./cfn-svc &
MCP_PID=$!

echo "HTTP server PID: $HTTP_PID"
echo "MCP server PID: $MCP_PID"

# Function to cleanup on exit
cleanup() {
    echo "Shutting down servers..."
    kill $HTTP_PID $MCP_PID 2>/dev/null
    wait $HTTP_PID $MCP_PID 2>/dev/null
    echo "Shutdown complete"
    exit 0
}

# Trap signals for graceful shutdown
trap cleanup SIGTERM SIGINT

# Wait for both processes
wait $HTTP_PID $MCP_PID
