# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

## Public API Contracts

This service provides **versioned OpenAPI specifications** for external SDK generation:
- Specifications are located in `docs/public-api/`
- Updated when cutting SDK releases (not on every commit)
- Go implementation is the source of truth
- See [docs/public-api/README.md](docs/public-api/README.md) for SDK release workflow

**Current version:** `public-api-v1.0.yaml`

## Prerequisites

1. **IoC Management Plane**: Start the backend and UI (required for CFN registration):

```bash
# Clone and run the management backend
git clone https://github.com/cisco-eti/ioc-cfn-mgmt-backend-svc
cd ioc-cfn-mgmt-backend-svc
task docker-compose-full-stack-up    # Start complete stack (application + databases)
```

See [ioc-cfn-mgmt-backend-svc deployment options](https://github.com/cisco-eti/ioc-cfn-mgmt-backend-svc?tab=readme-ov-file#deployment-options) for more details.

2. **PostgreSQL**: Ensure a PostgreSQL instance is running and the `cfn_cp` database exists. Tables are auto-migrated by the service on startup.

## Quick Start

### Option 1: Go directly

`.env` is auto-loaded on startup via [godotenv](https://github.com/joho/godotenv).

```bash
CGO_ENABLED=0 go run -ldflags "-X main.buildVersion=latest -X main.gitCommitSHA=$(git rev-parse --short HEAD) -X main.gitCommitTime=$(git log -1 --format=%cI) -X main.gitBranch=$(git rev-parse --abbrev-ref HEAD)" .
```

### Option 2: Using Make

```bash
make dev                    # runs both HTTP and MCP servers
```

### Option 3: Build binary

```bash
make build
make run        # runs both HTTP and MCP servers
```

App runs on **http://localhost:9002**

## API Endpoints

### Health & Info

```bash
# Health check (standard diagnostic)
curl http://localhost:9002/api/internal/diagnostics/health
# Response: {"status":"UP"}

# Get build/git info
curl http://localhost:9002/api/internal/diagnostics/info
# Response: {"git":{"commit":{"time":"2025-01-01T00:00:00-08:00","id":"abc1234"},"branch":"main"}}

```

### OTLP Span Ingestion

**POST /v1/traces** — Accepts OpenTelemetry spans (protobuf or JSON).

### Metrics APIs

Time-series metrics storage and querying for CE infrastructure and MAS operations.

#### Push Metrics

**POST /api/internal/cognition-engine/metrics** — Ingest metrics batch (CE or MAS)

Auto-detects metric type:
- Has `ce_id`? → CE infrastructure metrics
- Has `workspace_id` + `mas_id` + `agent_id`? → MAS operation metrics

```bash
# CE metrics example
curl -X POST http://localhost:9002/api/internal/cognition-engine/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "ce_id": "550e8400-e29b-41d4-a716-446655440000",
    "attributes": {"hostname": "ce-prod-01", "region": "us-west-2"},
    "metrics": [
      {"name": "ce.queue.depth", "value": 12},
      {"name": "ce.memory.usage_pct", "value": 67.5}
    ]
  }'

# MAS metrics example
curl -X POST http://localhost:9002/api/internal/cognition-engine/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "workspace_id": "770fa621-04bd-42f6-a938-668877662222",
    "mas_id": "880fb732-15ce-43a7-b049-779988773333",
    "agent_id": "agent-1",
    "metrics": [
      {"name": "llm.tokens.total", "value": 1234}
    ]
  }'
```

#### Query Metrics

**GET /api/cognition-engine/metrics** — Query metrics with smart routing

Smart routing based on filters:
- Only `ce_id`? → CE metrics only
- Only `workspace_id`/`mas_id`/`agent_id`? → MAS metrics only
- Both? → Both tables
- Neither? → Both tables (time-range query)

```bash
# Query CE metrics only
curl "http://localhost:9002/api/cognition-engine/metrics?\
ce_id=550e8400-e29b-41d4-a716-446655440000&\
start_time=2026-05-27T00:00:00Z&\
end_time=2026-05-27T23:59:59Z"

# Query MAS metrics only
curl "http://localhost:9002/api/cognition-engine/metrics?\
workspace_id=770fa621-04bd-42f6-a938-668877662222&\
start_time=2026-05-27&\
end_time=2026-05-28"

# Query both (time-range only)
curl "http://localhost:9002/api/cognition-engine/metrics?\
start_time=2026-05-27T14:00:00Z&\
end_time=2026-05-27T15:00:00Z"
```

**Response structure:**
```json
{
  "period": {"start": "...", "end": "..."},
  "filters": {"ce_id": "...", "workspace_id": "..."},
  "ce_metrics": {
    "total": 245,
    "limit": 1000,
    "offset": 0,
    "data": [
      {
        "timestamp": "2026-05-27T10:30:00Z",
        "ce_id": "550e8400-...",
        "metric_name": "ce.queue.depth",
        "value": 12.0,
        "attributes": {"hostname": "ce-prod-01"}
      }
    ]
  },
  "mas_metrics": {
    "total": 1823,
    "limit": 1000,
    "offset": 0,
    "data": [
      {
        "timestamp": "2026-05-27T10:30:05Z",
        "workspace_id": "770fa621-...",
        "mas_id": "880fb732-...",
        "agent_id": "negotiator-1",
        "metric_name": "llm.tokens.prompt",
        "value": 1234.0,
        "attributes": {"model": "claude-sonnet-4-6"}
      }
    ]
  }
}
```

**Query Parameters:**
- `start_time`, `end_time` (required): Unix timestamp, RFC3339, or date
- `ce_id` (optional): Filter CE metrics by instance
- `workspace_id`, `mas_id`, `agent_id` (optional): Filter MAS metrics
- `metric_name` (optional): Filter by name (supports `*` wildcard, e.g., `llm.tokens.*`)
- `limit` (optional): Max results per table (default 1000, max 10000)
- `offset` (optional): Pagination offset per table (default 0)

**Storage:**
- **TimescaleDB** hypertables (`ce_metrics`, `mas_metrics`)
- **Retention**: 90 days
- **Compression**: After 7 days
- **Indexing**: Optimized for time-range + entity ID queries

### Shared Memory APIs

See [shared-memory-operations](./docs/shared-memory-operations.md)

**Notes:**
- Replace `{workspaceId}` and `{systemId}` with actual IDs
- Memories and relationships accept flexible key-value structures
- Designed for multi-agent systems to share context and coordinate actions

### Remote Agent Memory Operations

**POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations**

Proxy HTTP requests to a remote memory provider (Mem0, Graphiti, etc.) for agent-specific memory operations. The memory provider base URL and auth credentials are auto-resolved from management plane config.

```bash
# Example: Add memories (POST)
curl -X 'POST' \
  'http://localhost:9002/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "header": {},
  "payload": {
    "http-request-type": "POST",
    "http-url": "/v1/memories/",
    "http-request-body": {
      "messages": [{"role": "user", "content": "I prefer dark mode in all my apps"}],
      "user_id": "curl-test-user"
    },
    "http-headers": {}
  }
}'

# Example: Retrieve memories (GET)
curl -X 'POST' \
  'http://localhost:9002/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "header": {},
  "payload": {
    "http-request-type": "GET",
    "http-url": "v1/memories/?user_id=curl-test-user",
    "http-request-body": {},
    "http-headers": {}
  }
}'

# Example: Delete a memory (DELETE)
curl -X 'POST' \
  'http://localhost:9002/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "header": {},
  "payload": {
    "http-request-type": "DELETE",
    "http-url": "/v1/memories/mem-12345/",
    "http-request-body": {},
    "http-headers": {}
  }
}'
```

**Request Body:**
| Field | Required | Description |
|-------|----------|-------------|
| `header` | No | Reserved for future SSTP header (pass `{}`) |
| `payload.http-request-type` | Yes | HTTP method: `GET`, `POST`, `PUT`, `DELETE`, `PATCH` |
| `payload.http-url` | No | Relative path + query params appended to the provider base URL (e.g., `v1/memories/?user_id=123`). If omitted, uses base URL from config. |
| `payload.http-request-body` | No | JSON payload to send to memory provider (pass `{}` for empty) |
| `payload.http-headers` | No | Custom HTTP headers to forward (pass `{}` for none) |

**Notes:**
- Replace `{workspaceId}`, `{masId}`, and `{agentId}` with actual IDs from the management plane
- **URL + Auth Auto-Resolution:** The memory provider base URL and authentication credentials are automatically resolved from the management plane config based on workspace/MAS/agent IDs. No need to pass auth headers in the request.
- The outer HTTP status is always `200` for successful proxying; the actual provider status is in `http-status`
- User-supplied `Authorization` headers are stripped for security; auth is injected server-side from config
- All request/response bodies are JSON

> **Mem0 provider setup:** Create an account at [https://mem0.ai/](https://mem0.ai/), copy your API key, and configure it in the management plane UI under the agent's memory provider registration settings with auth type `API KEY`.

> **Note:** This API is under active development and subject to change. Please check the Swagger docs (`/docs/index.html`) for the latest contract and keep this section updated accordingly.

### Log Level Management

**GET /api/internal/diagnostics/loggers** - Get current log levels for ROOT and all packages

```bash
curl http://localhost:9002/api/internal/diagnostics/loggers
```

Response:
```json
{"ROOT":"info","app":"info","config":"info","mcp":"info"}
```

**POST /api/internal/diagnostics/loggers** - Set log level dynamically

```bash
# Set ROOT level (affects ALL loggers)
curl -X POST http://localhost:9002/api/internal/diagnostics/loggers \
  -H "Content-Type: application/json" \
  -d '{"module-name": "ROOT", "log-level": "debug"}'

# Set specific package level (only affects that package)
curl -X POST http://localhost:9002/api/internal/diagnostics/loggers \
  -H "Content-Type: application/json" \
  -d '{"module-name": "app", "log-level": "debug"}'
```

Response: `204 No Content` on success

**Request Body:**
| Field | Required | Description |
|-------|----------|-------------|
| `module-name` | No | Package name or "ROOT" (default: ROOT) |
| `log-level` | Yes | Valid levels: debug, info, warn, error, dpanic, panic, fatal |

**Error Responses (400 Bad Request):**
```json
{"error": "log-level is required"}
{"error": "invalid log level: verbose. Valid levels: debug, info, warn, error, dpanic, panic, fatal"}
{"error": "unknown module: typo. Use GET /api/internal/diagnostics/loggers to see available modules"}
```

### Audit Events (Internal API)

> For full audit system documentation (architecture, schema, enums, design decisions), see [AUDIT.md](AUDIT.md).

Audit events are created internally by handlers (e.g. shared memory, memory operations) — there are no create or delete HTTP endpoints. The API is read-only.

**GET /api/internal/mgmt/audit** - List audit events (with optional filters and pagination)

```bash
# List all (defaults: page=0, pageSize=20)
curl http://localhost:9002/api/internal/mgmt/audit

# With pagination
curl "http://localhost:9002/api/internal/mgmt/audit?page=0&pageSize=50"

# Filter by resource_type
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=COGNITION_ENGINE"

# Filter by audit_type
curl "http://localhost:9002/api/internal/mgmt/audit?audit_type=RESOURCE_CREATED"

# Filter by both
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=KNOWLEDGE_QUERY"

# Both filters + pagination
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=RESOURCE_CREATED&page=0&pageSize=50"
```

**Query Parameters:**
| Param | Required | Default | Description |
|-------|----------|---------|-------------|
| `page` | No | `0` | 0-based page number (must be `>= 0`) |
| `pageSize` | No | `20` | Records per page (must be `>= 1`, capped at `MAX_PAGE_SIZE`, default `100`) |
| `resource_type` | No | *(none)* | Filter by resource type (e.g. `COGNITION_ENGINE`, `MAS`, `MAS-AGENT`) |
| `audit_type` | No | *(none)* | Filter by audit type (e.g. `RESOURCE_CREATED`, `SHARED_MEMORY_OPERATION`) |

**Response:** `200 OK`
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "operation_id": "op-12345",
      "resource_type": "COGNITION_ENGINE",
      "resource_identifier": "engine-123",
      "audit_type": "RESOURCE_CREATED",
      "audit_resource_identifier": "cognitive-engine-456",
      "audit_information": {"config": {"version": "1.0"}},
      "created_by": "550e8400-e29b-41d4-a716-446655440001",
      "created_on": "2024-02-18T15:30:00Z",
      "last_modified_by": "550e8400-e29b-41d4-a716-446655440001",
      "last_modified_on": "2024-02-18T15:30:00Z"
    }
  ],
  "pageInfo": {
    "page": 0,
    "pageSize": 20,
    "pageCount": 1,
    "totalElements": 1
  }
}
```

**GET /api/internal/mgmt/audit/{eventId}** - Get a single audit event by UUID

```bash
curl http://localhost:9002/api/internal/mgmt/audit/<event-id>
```

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "operation_id": "op-12345",
  "resource_type": "COGNITION_ENGINE",
  "resource_identifier": "ce-123",
  "audit_type": "RESOURCE_CREATED",
  "audit_resource_identifier": "ce-123",
  "audit_information": {"config": {"version": "1.0"}},
  "created_by": "00000000-0000-0000-0000-000000000001",
  "created_on": "2024-02-18T15:30:00Z",
  "last_modified_by": "00000000-0000-0000-0000-000000000001",
  "last_modified_on": "2024-02-18T15:30:00Z"
}
```

## Environment Setup

### 1. Create .env file

```bash
cp .env.sample .env
```

### 2. Run the app

The app automatically loads `.env` on startup via [godotenv](https://github.com/joho/godotenv).

**Using Make:**
```bash
make dev       # go run with git info injected
make run       # build binary then run (both HTTP and MCP servers)
```

**Using Go directly:**
```bash
CGO_ENABLED=0 go run -ldflags "-X main.buildVersion=latest -X main.gitCommitSHA=$(git rev-parse --short HEAD) -X main.gitCommitTime=$(git log -1 --format=%cI) -X main.gitBranch=$(git rev-parse --abbrev-ref HEAD)" .
```

The service starts on port `9002` by default.

### 3. Access API documentation
- **OpenAPI/Swagger UI**: http://localhost:9002/docs/index.html

## Startup Registration

On startup, the service registers itself with the IoC Management Plane:

```
POST {MGMT_URL}/api/workspaces/{WORKSPACE_ID}/cognitive-fabric-node/register
Header: X-API-Key: {X_API_KEY}
Body: {
  "cfn_id": "<auto-generated-uuid>",
  "cfn_name": "cfn-local",
  "cfn_config": {}
}
```

After successful registration, a periodic heartbeat is sent every 29 seconds:

```
PUT {MGMT_URL}/api/workspaces/{WORKSPACE_ID}/cognitive-fabric-node/{cfn_id}/heartbeat
Header: X-API-Key: {X_API_KEY}
```

Uses robust HTTP client with 3 retries and exponential backoff.

## MCP Server Mode

The service runs both HTTP and MCP (Model Context Protocol) servers simultaneously for AI tool integration.

```bash
# Test MCP client-server communication
go test -v ./pkg/client/mcp/...
```

MCP server provides an `echo` tool by default. Logs all client requests with session ID, method, and duration.

## Configuration

Environment variables (uppercase):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 9002 | App server port |
| `APP_NAME` | ioc-cfn-svc | Service name |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |
| `MGMT_URL` | http://localhost:9000 | Management plane URL |
| `CFN_NAME` | My Cognition Fabric Node | CFN instance name |
| `HEARTBEAT_INTERVAL_SECONDS` | 29 | Heartbeat interval in seconds |
| `MCP_PORT` | 9001 | MCP server port |
| `MCP_HOST` | (empty) | MCP server host |
| `DEFAULT_PAGE_SIZE` | 20 | Default number of records per page (audit list) |
| `MAX_PAGE_SIZE` | 100 | Maximum allowed records per page (audit list) |
| `ENABLE_TIMESCALEDB` | false | Enable TimescaleDB hypertable features (compression, retention). Requires TimescaleDB extension. Set to `true` in production. |

## Commands

```bash
# Build & Run
make dev            # go run with git info (loads .env)
make build          # Build binary
make run            # Build and run binary (both HTTP and MCP servers)

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
    httpapi/
      cognitionagents/  # DTOs for cognition agents memory API
      sharedmemory/     # DTOs for shared memory API
  audit/            # Audit logging
  client/
    database/       # GORM database client and migrations
    cognitionagentclient/  # Client for external cognition agents API
    http/           # Robust HTTP client with retries
    mcp/            # MCP client/server implementation
  config/           # Configuration
  mapper/           # Data mappers
  otelreceiver/     # OTLP/HTTP receiver, span mapper, and TimescaleDB exporter
  metric/           # Prometheus metrics
  model/            # Data models
  task/             # Task management
  tools/            # Utilities (logger, http)
build/              # Dockerfile, build scripts
deploy/             # Helm charts
docs/               # Swagger docs
```

## CI/CD

- **PR**: Builds `ghcr.io/cisco-eti/ioc-cfn-svc:latest` (no push)
- **Merge to main**: Builds and pushes to GHCR/ECR
