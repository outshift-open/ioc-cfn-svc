# Shared memory operations

This document shows how to **create/update** and **query** shared memories for inter-agent coordination.

Supported input formats:

- **Otel Trace** (`observe-sdk-otel`)
- **OpenClaw** (`openclaw`)

## Create or update shared memories (async)

Send a payload to persist concepts/relationships. The API is **asynchronous** — it returns `202 Accepted` immediately and processes the extraction and storage in the background.

### Example: Otel Trace input

Source: [../pkg/app/testdata/otel.json](../pkg/app/testdata/otel.json)

```bash
cat ../pkg/app/testdata/otel.json | jq -s '{
  "header": {
    "agent_id": "agent-1"
  },
  "payload": {
    "metadata": {
      "format": "observe-sdk-otel"
    },
    "data": .[0]
  }
}' | curl -X POST \
  http://localhost:9002/api/workspaces/ws1/multi-agentic-systems/mas_otel/shared-memories \
  -H "Content-Type: application/json" \
  --data-binary @-

# Response (202 Accepted)
# {
#   "response_id": "my-trace-id-123",
#   "status": "accepted",
#   "message": "Request accepted for asynchronous processing"
# }
```

### Example: OpenClaw input

Source: [../pkg/app/testdata/openclaw.json](../pkg/app/testdata/openclaw.json)

```bash
cat ../pkg/app//testdata/openclaw.json | jq -s '{
  "header": {
    "agent_id": "agent-1"
  },
  "payload": {
    "metadata": {
      "format": "openclaw"
    },
    "data": .[0]
  }
}' | curl -X POST \
  http://localhost:9002/api/workspaces/ws1/multi-agentic-systems/mas_openclaw/shared-memories \
  -H "Content-Type: application/json" \
  --data-binary @-

# Response (202 Accepted)
# {
#   "response_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
#   "status": "accepted",
#   "message": "Request accepted for asynchronous processing"
# }
```

> **Note:** The `response_id` in the response can be used to correlate logs. If not provided in the request, a UUID is auto-generated.

## Fetch shared memories

Query stored memories from an existing graph. Note that the graph name is implied from the MAS ID in the API path.

### Example: query Otel graph

```bash
curl -X POST \
  http://localhost:9002/api/workspaces/ws1/multi-agentic-systems/mas_otel/shared-memories/query \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "agent_id": "agent-1"
    },
    "search_strategy": "semantic_graph_traversal",
    "intent": "What does the website_selector_agent do?"
  }' | jq

# Response (200 OK)
# {
#   "response_id": "d414f287-aa78-4a79-9e9e-8c7c7226a3eb",
#   "message": "The website_selector_agent performs internet searches using the search_serper function to identify relevant websites."
# }
```

### Example: query OpenClaw graph

```bash
curl -X POST \
  http://localhost:9002/api/workspaces/ws1/multi-agentic-systems/mas_openclaw/shared-memories/query \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "agent_id": "agent-1"
    },
    "search_strategy": "semantic_graph_traversal",
    "intent": "Tell me something about Q2 budget planning."
  }' | jq

# Response (200 OK)
# {
#   "response_id": "4252795b-70ca-4101-8de5-8a5b944fbe35",
#   "message": "The Q2 budget planning session is constrained by a total budget of $200,000. Alex, the Head of Engineering, is advocating for a $95,000 allocation, while Sam, the Head of Sales, is advocating for a $90,000 allocation."
# }
```
