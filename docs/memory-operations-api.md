# Remote Agent Memory Operations API

This API endpoint allows you to interact with remote Agent Memory providers through a REST interface. It acts as a proxy that forwards HTTP requests to the memory provider.

## Endpoint

```
POST /api/workspaces/{workspace-id}/multi-agentic-systems/{mas-id}/agents/{agent-id}/memory-operations
```

### Path Parameters

- `workspace-id` - The workspace identifier
- `mas-id` - The multi-agentic system identifier
- `agent-id` - The agent identifier

## Request Format

```json
{
  "header": {
    // Optional header element
    // In the future this may become the SSTP header
  },
  "payload": {
    "http-request-type": "POST",
    "http-url": "https://memory-provider.example.com/api/memories?filter=recent",
    "http-request-body": {
      "memory-key": "user-preferences",
      "memory-value": {
        "theme": "dark",
        "language": "en"
      }
    },
    "http-headers": {
      "Authorization": "Bearer token123",
      "X-Custom-Header": "custom-value"
    }
  }
}
```

### Request Fields

- `header` (optional) - Reserved for future use (SSTP header)
- `payload` (required) - Contains the HTTP request details
  - `http-request-type` (required) - HTTP method (POST, PUT, GET, DELETE, PATCH, etc.)
  - `http-url` (required) - Full URL including query parameters (URL encoded)
  - `http-request-body` (optional) - JSON payload to send to the memory provider
  - `http-headers` (optional) - Custom HTTP headers to include in the request

## Response Format

```json
{
  "http-status": 201,
  "http-headers": {
    "Content-Type": "application/json",
    "X-Memory-Id": "mem-12345"
  },
  "http-response-body": {
    "status": "success",
    "message": "Memory stored successfully",
    "memory-id": "mem-12345"
  }
}
```

### Response Fields

- `http-status` - HTTP status code returned by the memory provider
- `http-headers` - Response headers from the memory provider
- `http-response-body` - JSON response body from the memory provider

## Examples

### Create a Memory (POST)

```bash
curl -X POST http://localhost:8080/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "http-request-type": "POST",
      "http-url": "https://memory-provider.example.com/api/memories",
      "http-request-body": {
        "content": "User prefers dark mode",
        "category": "preferences"
      },
      "http-headers": {
        "Authorization": "Bearer token123"
      }
    }
  }'
```

### Retrieve Memories (GET)

```bash
curl -X POST http://localhost:8080/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "http-request-type": "GET",
      "http-url": "https://memory-provider.example.com/api/memories?category=preferences",
      "http-headers": {
        "Authorization": "Bearer token123"
      }
    }
  }'
```

### Update a Memory (PUT)

```bash
curl -X POST http://localhost:8080/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "http-request-type": "PUT",
      "http-url": "https://memory-provider.example.com/api/memories/mem-12345",
      "http-request-body": {
        "content": "User prefers light mode",
        "category": "preferences"
      },
      "http-headers": {
        "Authorization": "Bearer token123"
      }
    }
  }'
```

### Delete a Memory (DELETE)

```bash
curl -X POST http://localhost:8080/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "http-request-type": "DELETE",
      "http-url": "https://memory-provider.example.com/api/memories/mem-12345",
      "http-headers": {
        "Authorization": "Bearer token123"
      }
    }
  }'
```

## Error Responses

### Validation Errors (400 Bad Request)

```json
{
  "error": "http-request-type is required"
}
```

### Gateway Errors (502 Bad Gateway)

When the memory provider is unreachable:

```json
{
  "error": "failed to forward request to memory provider: connection refused"
}
```

## Notes

- The endpoint always returns HTTP 200 for successful proxying
- The actual status from the memory provider is in `http-status` field
- All request/response bodies are assumed to be JSON
- Query parameters should be included in the `http-url` field (URL encoded)
- The memory provider's response headers are preserved in the response
- If the memory provider returns non-JSON content, it will be wrapped in a `raw` field

## Future Enhancements

- The `header` field will be used for SSTP (Secure Service-to-Service Protocol) in future versions
- Support for non-JSON payloads (multipart/form-data, binary data)
- MCP (Model Context Protocol) client integration
