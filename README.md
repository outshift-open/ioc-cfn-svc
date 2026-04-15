# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

## Architecture

This service follows a **schema-first** API design approach:
- OpenAPI 3.0 specification (`docs/openapi.yaml`) is the single source of truth
- Go types and server interfaces are generated from the spec using `oapi-codegen`
- API contract is validated at build time
- Documentation is always accurate and never drifts from implementation

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
make dev                    # same as above
MCP_ENABLED=true make dev   # MCP mode
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
# Health check (standard diagnostic)
curl http://localhost:9002/api/internal/diagnostics/health
# Response: {"status":"UP"}

# Get build/git info
curl http://localhost:9002/api/internal/diagnostics/info
# Response: {"git":{"commit":{"time":"2025-01-01T00:00:00-08:00","id":"abc1234"},"branch":"main"}}

```

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

**GET /api/internal/mgmt/audit** - List audit events (with optional filters)

```bash
# List all (default: skip=0, limit=100)
curl http://localhost:9002/api/internal/mgmt/audit

# With pagination
curl "http://localhost:9002/api/internal/mgmt/audit?skip=0&limit=50"

# Filter by resource_type
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=COGNITION_ENGINE"

# Filter by audit_type
curl "http://localhost:9002/api/internal/mgmt/audit?audit_type=RESOURCE_CREATED"

# Filter by both
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=KNOWLEDGE_QUERY"

# Both filters + pagination
curl "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=RESOURCE_CREATED&skip=0&limit=50"
```

**Query Parameters:**
| Param | Required | Default | Description |
|-------|----------|---------|-------------|
| `resource_type` | No | *(none)* | Filter by resource type (e.g. `COGNITION_ENGINE`, `MAS`, `MAS-AGENT`) |
| `audit_type` | No | *(none)* | Filter by audit type (e.g. `RESOURCE_CREATED`, `SHARED_MEMORY_OPERATION`) |
| `skip` | No | `0` | Number of records to skip for pagination (must be `>= 0`) |
| `limit` | No | `100` | Maximum number of records to return (must be `>= 1`) |

**Response:** `200 OK`
```json
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
make run       # build binary then run
make run-mcp   # build and run in MCP mode
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
| `CFN_NAME` | My Cognition Fabric Node | CFN instance name |
| `HEARTBEAT_INTERVAL_SECONDS` | 29 | Heartbeat interval in seconds |
| `MCP_ENABLED` | false | Enable MCP server mode |
| `MCP_PORT` | 9002 | MCP server port |
| `MCP_HOST` | (empty) | MCP server host |

## Commands

```bash
# Build & Run
make dev            # go run with git info (loads .env)
make build          # Build binary
make run            # Build and run binary
make run-mcp        # Build and run in MCP mode

# Code Generation & Documentation
make generate       # Generate Go code from OpenAPI spec
make docs           # Alias for generate (schema-first approach)

# Other
make test           # Run tests
make docker         # Build docker image
make clean          # Remove build artifacts
```

## Schema-First Development Workflow

### Modifying the API

1. **Edit the OpenAPI spec**: `docs/openapi.yaml`
   - Add/modify endpoints, request/response schemas
   - Add validation rules (minLength, required, enums, etc.)
   - Use operationId to control generated method names

2. **Regenerate Go code**:
   ```bash
   make generate
   ```
   This creates:
   - `pkg/generated/api/server.gen.go` - Types and ServerInterface

3. **Implement handlers**: Update adapter methods in `pkg/app/openapi_handlers.go`

4. **Test**: Run tests to ensure changes work
   ```bash
   make test
   ```

5. **View docs**: Start server and visit http://localhost:9002/docs/

### Benefits of Schema-First

- **Contract-driven development**: API contract is explicit and reviewed before implementation
- **Automatic validation**: Requests are validated against the schema automatically
- **No documentation drift**: Spec is the source of truth
- **Breaking change detection**: Schema changes are explicit and reviewable
- **Client SDK generation**: Generate client SDKs from the same spec

### OpenAPI Spec Location

- **Source**: `docs/openapi.yaml` (edit this file)
- **Generated code**: `pkg/generated/api/*.gen.go` (do not edit, regenerate instead)

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
build/              # Dockerfile, build scripts
deploy/             # Helm charts
docs/               # Swagger docs
```

## CI/CD

- **PR**: Builds `ghcr.io/cisco-eti/ioc-cfn-svc:latest` (no push)
- **Merge to main**: Builds and pushes to GHCR/ECR
