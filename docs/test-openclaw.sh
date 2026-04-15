#!/bin/bash
# Test script for OpenClaw shared memory: create/upsert and fetch/query
# Usage: ./docs/test-openclaw.sh

# --- Params ---
WORKSPACE_ID="eb47e066-0414-426f-b1a0-e5222fcf50b6"
MAS_ID="5b4c2a20-7bf8-4afe-af5e-ad919cdbe624"
CFN_ID="7847d280-a37c-4367-a035-34367bab04cc"
AGENT_ID="pqr"

# --- 1. Create/Upsert shared memories (OpenClaw) using testdata ---
echo "=== 1. Create/Upsert (openclaw testdata) ==="

cat pkg/app/testdata/openclaw.json | jq -s '{
  "header": {
    "agent_id": "'"${AGENT_ID}"'"
  },
  "payload": {
    "metadata": {
      "format": "openclaw"
    },
    "data": .[0]
  }
}' | curl -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories" \
  -H "Content-Type: application/json" \
  --data-binary @-

echo ""

# --- 2. Fetch/Query shared memories ---
echo "=== 2. Fetch/Query ==="

curl -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "agent_id": "'"${AGENT_ID}"'"
    },
    "search_strategy": "semantic_graph_traversal",
    "intent": "Tell me about the Q2 budget planning."
  }' | jq

echo ""

# --- 3. Fetch/Query (different intent) ---
echo "=== 3. Fetch/Query (different intent) ==="

curl -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "agent_id": "'"${AGENT_ID}"'"
    },
    "search_strategy": "semantic_graph_traversal",
    "intent": "What does the engineering team need?"
  }' | jq

echo ""

# --- 4. Additional fetch queries (generates more audit entries) ---
echo "=== 4a. Fetch — project status ==="
curl -s -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": { "agent_id": "'"${AGENT_ID}"'" },
    "search_strategy": "semantic_graph_traversal",
    "intent": "What is the current project status?"
  }' | jq

echo ""
echo "=== 4b. Fetch — team members ==="
curl -s -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": { "agent_id": "'"${AGENT_ID}"'" },
    "search_strategy": "semantic_graph_traversal",
    "intent": "Who are the team members involved?"
  }' | jq

echo ""
echo "=== 4c. Fetch — action items ==="
curl -s -X POST \
  "http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": { "agent_id": "'"${AGENT_ID}"'" },
    "search_strategy": "semantic_graph_traversal",
    "intent": "What are the pending action items?"
  }' | jq

echo ""

# --- 5. Check audit trail ---
echo "=== 5a. Audit — all events (page 0, pageSize 5) ==="
curl -s "http://localhost:9002/api/internal/mgmt/audit?page=0&pageSize=5" | jq

echo ""
echo "=== 5b. Audit — filter by resource_type=MAS ==="
curl -s "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&page=0&pageSize=10" | jq

echo ""
echo "=== 5c. Audit — filter by audit_type=SHARED_MEMORY_OPERATION ==="
curl -s "http://localhost:9002/api/internal/mgmt/audit?audit_type=SHARED_MEMORY_OPERATION&page=0&pageSize=10" | jq

echo ""
echo "=== 5d. Audit — both filters + pagination ==="
curl -s "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=SHARED_MEMORY_OPERATION&page=0&pageSize=2" | jq

echo ""
echo "=== 5e. Audit — page 1 (next page) ==="
curl -s "http://localhost:9002/api/internal/mgmt/audit?resource_type=MAS&audit_type=SHARED_MEMORY_OPERATION&page=1&pageSize=2" | jq

echo ""
echo "=== 5f. Audit — get single event by ID (first from list) ==="
EVENT_ID=$(curl -s "http://localhost:9002/api/internal/mgmt/audit?page=0&pageSize=1" | jq -r '.data[0].id')
if [ "$EVENT_ID" != "null" ] && [ -n "$EVENT_ID" ]; then
  echo "Fetching event: ${EVENT_ID}"
  curl -s "http://localhost:9002/api/internal/mgmt/audit/${EVENT_ID}" | jq
else
  echo "No audit events found to fetch by ID."
fi

echo ""
echo "=== 5g. Audit — error cases ==="
echo "--- Invalid resource_type ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit?resource_type=BOGUS" | jq

echo "--- Invalid page ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit?page=abc" | jq

echo "--- Invalid pageSize ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit?pageSize=-1" | jq

echo "--- pageSize exceeds max (should clamp) ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit?pageSize=99999" | jq '.pageInfo'

echo "--- Invalid UUID ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit/not-a-uuid" | jq

echo "--- Event not found ---"
curl -s "http://localhost:9002/api/internal/mgmt/audit/00000000-0000-0000-0000-000000000000" | jq

echo ""
echo "Done."
