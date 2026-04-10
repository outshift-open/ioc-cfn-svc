# Shared memory operations

This document shows how to **create/update** and **query** shared memories for inter-agent coordination.

Supported input formats:

- **Otel Trace** (`observe-sdk-otel`)
- **OpenClaw** (`openclaw`)

## Create or update shared memories

Send a payload to persist concepts/relationships.

### Example: Otel Trace input

Source: [../pkg/app/testdata/otel.json](../pkg/app/testdata/otel.json)

```bash
cat ../pkg/app//testdata/otel.json | jq -s '{
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

# Response (201 Created)
# {
#   "response_id": "9af99ba5-e8aa-47aa-a217-b87dd928ac59",
#   "message": "Successfully saved 10 nodes and 16 edges to graph 'graph_mas_otel'"
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

# Response (201 Created)
# {
#   "response_id": "9af99ba5-e8aa-47aa-a217-b87dd928ac59",
#   "message": "Successfully saved 15 nodes and 17 edges to graph 'graph_mas_openclaw'"
# }
```

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
