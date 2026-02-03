# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

## Quick Start

### Option 1: Docker (Recommended)

```bash
# HTTP mode (default)
make dc-up

# MCP mode
make dc-up-mcp

# Build locally and run
make dc-up-build
MCP_ENABLED=true make dc-up-build   # MCP mode with local build

# Or without make
docker compose --file build/docker-compose.yaml up                    # HTTP mode
docker compose --file build/docker-compose.yaml up --build            # Build locally
MCP_ENABLED=true docker compose --file build/docker-compose.yaml up   # MCP mode
```

### Option 2: Go directly

```bash
go run .                    # HTTP mode
MCP_ENABLED=true go run .   # MCP mode
```

### Option 3: Build binary

```bash
make build
make run        # HTTP mode
make run-mcp    # MCP mode
```

App runs on **http://localhost:9010**

## API Endpoints

```bash
# Health check
curl http://localhost:9010/healthz

# Readiness check
curl http://localhost:9010/ready

# CFN dummy API
curl http://localhost:9010/api/v1/cfn/dummy
```

## Startup Registration

On startup, the service registers itself with a management service via HTTP POST:

```go
POST $MGMT_URL (default: http://mgmt/register)
{
  "mgmt_host_ip": "192.168.1.100",
  "mgmt_port": 6001,
  "cfn_id": "cfn-12345-abcde",
  "cfn_name": "my-cfn-service",
  "config": {"key": "value"}
}
```

Uses robust HTTP client with 3 retries and exponential backoff.

> **Note:** Payload values are currently hardcoded. TODO: Replace with config/env vars.

## MCP Server Mode

The service supports MCP (Model Context Protocol) for AI tool integration. Toggle between HTTP and MCP mode using `MCP_ENABLED` environment variable.

```bash
# Test MCP client-server communication
go test -v ./pkg/client/mcp/...
```

MCP server provides an `echo` tool by default. Logs all client requests with session ID, method, and duration.

## Configuration

Environment variables (uppercase):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 9010 | App server port |
| `APP_NAME` | ioc-cfn-svc | Service name |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |
| `MGMT_URL` | http://mgmt/register | Registration endpoint |
| `MCP_ENABLED` | false | Enable MCP server mode |
| `MCP_PORT` | 9010 | MCP server port |
| `MCP_HOST` | (empty) | MCP server host |

## Commands

```bash
# Build & Run
make build          # Build binary
make run            # Run HTTP mode (default)
make run-mcp        # Run MCP mode

# Docker Compose
make dc-up          # HTTP mode (default)
make dc-up-mcp      # MCP mode
make dc-up-build    # Build and run
make dc-stop        # Stop containers
make dc-down        # Remove containers

# Other
make test           # Run tests
make docker         # Build docker image
make docs           # Generate swagger docs
make clean          # Remove build artifacts
```

## Project Structure

```
main.go             # Entry point
pkg/
  app/              # Routes, handlers, startup registration
  audit/            # Audit logging
  client/
    http/           # Robust HTTP client with retries
    mcp/            # MCP client/server implementation
  config/           # Configuration
  mapper/           # Data mappers
  metric/           # Prometheus metrics
  model/            # Data models
  task/             # Task management
  tools/            # Utilities (logger, http)
build/              # Dockerfile, docker-compose, scripts
deploy/             # Helm charts
docs/               # Swagger docs
```

## CI/CD

- **PR**: Builds `ghcr.io/cisco-eti/ioc-cfn-svc:latest` (no push)
- **Merge to main**: Builds and pushes to GHCR/ECR
