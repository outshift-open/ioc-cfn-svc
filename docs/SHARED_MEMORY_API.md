# Shared Memory API Documentation

The Shared Memory API enables multi-agent systems to share context, coordinate actions, and maintain persistent knowledge across agent interactions.

## Base URL

```
http://localhost:9010/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories
```

## Endpoints

### 1. Upsert Shared Memories

**Endpoint:** `POST /api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories`

**Description:** Stores or updates memories and relationships for a specific workspace and multi-agentic system.

**Path Parameters:**
- `workspaceId` (string, required) - Unique identifier for the workspace
- `systemId` (string, required) - Unique identifier for the multi-agentic system

**Request Body:**

```json
{
  "memories": [
    {
      "id": "mem-1",
      "content": "User prefers dark mode",
      "type": "preference",
      "timestamp": "2026-02-18T10:00:00Z",
      "metadata": {
        "confidence": 0.95,
        "source": "agent-ui-assistant"
      }
    },
    {
      "id": "mem-2",
      "content": "Project uses Go 1.21 with Fiber framework",
      "type": "technical",
      "tags": ["golang", "framework"]
    }
  ],
  "relationships": [
    {
      "from": "mem-1",
      "to": "mem-2",
      "type": "related_to",
      "strength": 0.8,
      "context": "UI preferences affect framework choice"
    }
  ]
}
```

**Response (201 Created):**

```json
{
  "status": "success",
  "message": "shared memories upserted successfully"
}
```

**Example cURL Command:**

```bash
curl -X POST http://localhost:9010/api/workspaces/ws-abc123/multi-agentic-systems/sys-xyz789/shared-memories \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "id": "mem-1",
        "content": "User prefers Python for data analysis",
        "type": "preference"
      },
      {
        "id": "mem-2",
        "content": "Current project: E-commerce platform",
        "type": "context"
      }
    ],
    "relationships": [
      {
        "from": "mem-1",
        "to": "mem-2",
        "type": "applies_to"
      }
    ]
  }'
```

---

### 2. Fetch Shared Memories

**Endpoint:** `POST /api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories/query`

**Description:** Queries and retrieves stored memories based on specified criteria.

**Path Parameters:**
- `workspaceId` (string, required) - Unique identifier for the workspace
- `systemId` (string, required) - Unique identifier for the multi-agentic system

**Request Body:**

```json
{
  "TODO": "Query parameters to be defined"
}
```

**Response (200 OK):**

```json
{
  "TODO": "Response format to be defined"
}
```

**Example cURL Command:**

```bash
curl -X POST http://localhost:9010/api/workspaces/ws-abc123/multi-agentic-systems/sys-xyz789/shared-memories/query \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## Data Models

### SharedMemoryUpsertRequest

```go
type SharedMemoryUpsertRequest struct {
    Memories      []map[string]any `json:"memories"`
    Relationships []map[string]any `json:"relationships"`
}
```

**Field Descriptions:**

- **memories** (array of objects): Flexible array of memory objects. Each memory can contain any key-value pairs.
  - Common fields: `id`, `content`, `type`, `timestamp`, `metadata`, `tags`

- **relationships** (array of objects): Flexible array defining relationships between memories.
  - Common fields: `from`, `to`, `type`, `strength`, `context`

### SharedMemoryUpsertResponse

```go
type SharedMemoryUpsertResponse struct {
    Status  string `json:"status"`
    Message string `json:"message"`
}
```

**Field Descriptions:**

- **status** (string): Operation status (e.g., "success", "error")
- **message** (string): Human-readable message describing the operation result

---

## Use Cases

### Inter-Agent Communication

Agents can share learned information with other agents in the system:

```bash
# Agent A stores a learned preference
curl -X POST http://localhost:9010/api/workspaces/ws-1/multi-agentic-systems/sys-1/shared-memories \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "id": "pref-123",
        "agent_id": "agent-a",
        "learned_at": "2026-02-18T10:00:00Z",
        "content": "User prefers concise responses",
        "confidence": 0.9
      }
    ],
    "relationships": []
  }'

# Agent B can later query and use this information
```

### Context Sharing

Share project context across multiple agents:

```bash
curl -X POST http://localhost:9010/api/workspaces/project-x/multi-agentic-systems/dev-team/shared-memories \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "id": "ctx-1",
        "type": "project_info",
        "content": "Tech stack: Go, PostgreSQL, React"
      },
      {
        "id": "ctx-2",
        "type": "project_info",
        "content": "Deployment: Kubernetes on AWS EKS"
      }
    ],
    "relationships": [
      {
        "from": "ctx-1",
        "to": "ctx-2",
        "type": "deployment_context"
      }
    ]
  }'
```

### Knowledge Graph Building

Build relationships between memories:

```bash
curl -X POST http://localhost:9010/api/workspaces/ws-1/multi-agentic-systems/sys-1/shared-memories \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "id": "user-1",
        "type": "entity",
        "name": "Alice"
      },
      {
        "id": "skill-1",
        "type": "skill",
        "name": "Python"
      },
      {
        "id": "project-1",
        "type": "project",
        "name": "ML Pipeline"
      }
    ],
    "relationships": [
      {
        "from": "user-1",
        "to": "skill-1",
        "type": "has_skill",
        "proficiency": "expert"
      },
      {
        "from": "user-1",
        "to": "project-1",
        "type": "works_on",
        "role": "lead"
      }
    ]
  }'
```

---

## Error Responses

### 400 Bad Request

```json
{
  "error": "invalid JSON body"
}
```

Returned when the request body is not valid JSON or doesn't match the expected schema.

### 500 Internal Server Error

```json
{
  "error": "internal server error"
}
```

Returned when an unexpected error occurs during processing.

---

## Notes

- **Flexible Schema**: The `memories` and `relationships` arrays accept any JSON object structure, allowing for flexible data modeling.
- **No Schema Validation**: Currently, there's no strict schema validation on the memory and relationship objects.
- **Future Enhancements**: Query filtering, pagination, and memory versioning are planned for future releases.
- **Persistence**: Currently returns mock responses. Database persistence is under development.

---

## OpenAPI Documentation

Interactive API documentation is available at:
```
http://localhost:9010/docs/index.html
```
