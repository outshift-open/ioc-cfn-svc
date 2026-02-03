# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

## Quick Start

### Option 1: Docker (Recommended)

```bash
# Run with pre-built image from GHCR (no local build)
docker compose --file build/docker-compose.yaml up

# Or build and run locally
docker compose --file build/docker-compose.yaml up --build

# Or run directly without compose
docker run -p 9010:9010 ghcr.io/cisco-eti/ioc-cfn-svc:latest

# Run in MCP mode
MCP_ENABLED=true docker compose --file build/docker-compose.yaml up
```

### Option 2: Go directly

```bash
go run .
```

### Option 3: Build binary

```bash
make build
./ioc-cfn-svc.bin
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

The service supports MCP (Model Context Protocol) for AI tool integration.

```bash
# Run in MCP mode (go)
MCP_ENABLED=true go run .

# Run in MCP mode (binary)
make build
make run-mcp

# Run in MCP mode (docker compose)
make dc-up-mcp

# Run in MCP mode (docker run)
docker run -p 9010:9010 -e MCP_ENABLED=true -e MCP_PORT=9010 ghcr.io/cisco-eti/ioc-cfn-svc:latest

# Run tests (includes MCP client-server communication test)
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
make build          # Build binary
make run            # Run binary (HTTP mode)
make run-mcp        # Run binary (MCP mode)
make test           # Run tests
make docker         # Build docker image
make dc-up          # Docker compose up (HTTP mode)
make dc-up-mcp      # Docker compose up (MCP mode)
make dc-stop        # Docker compose stop
make dc-down        # Docker compose down
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
