# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

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

> **TODO:** Consolidate docker-compose with other CFN repos (e.g. shared docker-compose for mgmt-backend + cfn-svc + postgres)

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
# .env file is auto-loaded on startup
go run .                    # HTTP mode
MCP_ENABLED=true go run .   # MCP mode
```

### Option 3: Build binary

```bash
make build
make run        # HTTP mode
make run-mcp    # MCP mode
```

App runs on **http://localhost:9002**

## API Endpoints

### Health & Info

```bash
# Health check (TKF standard diagnostic)
curl http://localhost:9002/api/internal/diagnostics/health
# Response: {"status":"UP"}

# Get build/git info
curl http://localhost:9002/api/internal/diagnostics/info
# Response: {"git":{"commit":{"time":"2025-01-01T00:00:00-08:00","id":"abc1234"},"branch":"main"}}

```

### Shared Memory APIs

**Upsert Shared Memories** - Store memories and relationships for inter-agent communication

```bash
curl -X POST http://localhost:9002/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "id": "mem-1",
        "content": "User prefers dark mode",
        "type": "preference",
        "timestamp": "2026-02-18T10:00:00Z"
      },
      {
        "id": "mem-2",
        "content": "Project uses Go 1.21",
        "type": "technical"
      }
    ],
    "relationships": [
      {
        "from": "mem-1",
        "to": "mem-2",
        "type": "related_to",
        "strength": 0.8
      }
    ]
  }'

# Response (201 Created):
# {
#   "status": "success",
#   "message": "shared memories upserted successfully"
# }
```

**Fetch Shared Memories** - Query stored memories for agent coordination

```bash
curl -X POST http://localhost:9002/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories/query \
  -H "Content-Type: application/json" \
  -d '{}'

# Response (200 OK):
# TODO: Response format to be defined
```

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

### Audit Events

> For full audit system documentation (architecture, schema, enums, design decisions), see [AUDIT.md](AUDIT.md).

**POST /api/internal/audit-events** - Create an audit event

```bash
curl -X POST http://localhost:9002/api/internal/audit-events \
  -H "Content-Type: application/json" \
  -d '{
    "operation_id": "op-12345",
    "resource_type": "COGNITION_ENGINE",
    "resource_identifier": "ce-123",
    "audit_type": "RESOURCE_CREATED",
    "audit_resource_identifier": "ce-123",
    "created_by": "00000000-0000-0000-0000-000000000001",
    "last_modified_by": "00000000-0000-0000-0000-000000000001"
  }'
```

Response: `200 OK`
```json
{"message": "entry created"}
```

**Request Body:**
| Field | Required | Description |
|-------|----------|-------------|
| `resource_type` | Yes | `COGNITION_ENGINE`, `POLICY_ENFORCER`, `MEMORY_PROVIDER`, `MAS`, `MAS-AGENT`, `WORKFLOW`, `TASK` |
| `resource_identifier` | Yes | Identifier of the resource |
| `audit_type` | Yes | `RESOURCE_CREATED`, `RESOURCE_UPDATED`, `RESOURCE_DELETED`, `RESOURCE_PURGED`, `RESOURCE_PRUNED`, `KNOWLEDGE_INGESTION`, `KNOWLEDGE_QUERY`, `MEMORY_OPERATION` |
| `audit_resource_identifier` | Yes | Identifier of the audited resource |
| `operation_id` | No(TBD will change) | Optional operation correlation ID |
| `audit_information` | No | Optional JSON object with additional details |
| `audit_extra_information` | No | Optional string with extra context |
| `created_by` | Yes | UUID of the creator |
| `last_modified_by` | Yes | UUID of the last modifier |

**GET /api/internal/audit-events** - List audit events (with optional filters)

```bash
# List all
curl http://localhost:9002/api/internal/audit-events

# Response:
[
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
]

# Filter by resource_type
curl "http://localhost:9002/api/internal/audit-events?resource_type=COGNITION_ENGINE"

# Response:
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "operation_id": "op-12345",
    "resource_type": "COGNITION_ENGINE",
    "resource_identifier": "engine-123",
    "audit_type": "RESOURCE_CREATED",
    "audit_resource_identifier": "cognitive-engine-456",
    "created_by": "550e8400-e29b-41d4-a716-446655440001",
    "created_on": "2024-02-18T15:30:00Z",
    "last_modified_by": "550e8400-e29b-41d4-a716-446655440001",
    "last_modified_on": "2024-02-18T15:30:00Z"
  }
]

# Filter by audit_type
curl "http://localhost:9002/api/internal/audit-events?audit_type=RESOURCE_CREATED"

# Response:
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "operation_id": "op-67890",
    "resource_type": "MAS",
    "resource_identifier": "mas-456",
    "audit_type": "RESOURCE_CREATED",
    "audit_resource_identifier": "mas-agent-789",
    "created_by": "550e8400-e29b-41d4-a716-446655440002",
    "created_on": "2024-02-18T16:45:00Z",
    "last_modified_by": "550e8400-e29b-41d4-a716-446655440002",
    "last_modified_on": "2024-02-18T16:45:00Z"
  }
]

# Filter by both
curl "http://localhost:9002/api/internal/audit-events?resource_type=MAS&audit_type=KNOWLEDGE_QUERY"

# Response:
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "operation_id": "op-11111",
    "resource_type": "MAS",
    "resource_identifier": "mas-456",
    "audit_type": "KNOWLEDGE_QUERY",
    "audit_resource_identifier": "knowledge-query-123",
    "audit_information": {"query": "test query", "results": 5},
    "created_by": "550e8400-e29b-41d4-a716-446655440003",
    "created_on": "2024-02-18T17:20:00Z",
    "last_modified_by": "550e8400-e29b-41d4-a716-446655440003",
    "last_modified_on": "2024-02-18T17:20:00Z"
  }
]
```

**GET /api/internal/audit-events/{eventId}** - Get a single audit event by ID

```bash
curl http://localhost:9002/api/internal/audit-events/<event-id>
```

**Response:**
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

**DELETE /api/internal/audit-events/{eventId}** - Delete an audit event

```bash
curl -X DELETE http://localhost:9002/api/internal/audit-events/<event-id>
```

Response: `204 No Content`

## Environment Setup

### 1. Create .env file

```bash
cp .env.sample .env
```

### 2. Get credentials from IoC Management Plane UI

> **Note:** These credentials may not be needed in the future. Revisit when mgmt plane auth changes.

1. **API Key**: Create an API key manually and copy it
2. **Workspace ID**: Create a workspace and copy its ID

### 3. Update .env with your values

```bash
# .env
WORKSPACE_ID=your-workspace-id-here
X_API_KEY=your-api-key-here
```

### 4. Run with .env

The app automatically loads `.env` on startup via [godotenv](https://github.com/joho/godotenv).

**Go local:**
```bash
go run .   # .env is auto-loaded
```

**Docker Compose:** (uses port `9002`)
```bash
make dc-up           # Uses .env file
make dc-up-build     # Build locally and run
```

### 5. Access API documentation
- **OpenAPI/Swagger UI**: http://localhost:9002/docs/index.html
- **Shared Memory API Guide**: [docs/SHARED_MEMORY_API.md](docs/SHARED_MEMORY_API.md)

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
| `PORT` | 9002 | App server port |
| `APP_NAME` | ioc-cfn-svc | Service name |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |
| `MGMT_URL` | http://localhost:9000 | Management plane URL |
| `WORKSPACE_ID` | - | Workspace ID from IoC Mgmt Plane |
| `X_API_KEY` | - | API key from IoC Mgmt Plane |
| `CFN_NAME` | cfn-local | CFN instance name |
| `HEARTBEAT_INTERVAL_SECONDS` | 29 | Heartbeat interval in seconds |
| `MCP_ENABLED` | false | Enable MCP server mode |
| `MCP_PORT` | 9002 | MCP server port |
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
    httpapi/
      cognitionagents/  # DTOs for cognition agents memory API
      sharedmemory/     # DTOs for shared memory API
  audit/            # Audit logging
  client/
    cognitionagentclient/  # Client for external cognition agents API
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
