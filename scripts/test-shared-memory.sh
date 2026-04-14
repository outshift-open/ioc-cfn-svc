#!/usr/bin/env bash
set -euo pipefail

# Resolve repo root (one level up from scripts/)
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

BASE_URL="${BASE_URL:-http://localhost:9002}"
WORKSPACE_ID="1809bba0-4270-401d-af0e-7dd2cb8719bb"
MAS_ID="8e99e11b-8298-4bcd-9434-32ba98b6cc4f"
AGENT_ID="test12"
CFN_ID="009287e3-1f79-471e-bddb-463f1cda88dd"

API_PATH="${BASE_URL}/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories"

echo "=== 1. Create/Update Shared Memories (Otel) ==="
cat "${REPO_ROOT}/pkg/app/testdata/otel.json" | jq -s '{
  "header": {
    "agent_id": "'"${AGENT_ID}"'"
  },
  "payload": {
    "metadata": {
      "format": "observe-sdk-otel"
    },
    "data": .[0]
  }
}' | curl -s -X POST \
  "${API_PATH}" \
  -H "Content-Type: application/json" \
  --data-binary @- | jq .

echo ""
echo "=== 2. Query Shared Memories ==="
curl -s -X POST \
  "${API_PATH}/query" \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "agent_id": "'"${AGENT_ID}"'"
    },
    "search_strategy": "semantic_graph_traversal",
    "intent": "What does the website_selector_agent do?"
  }' | jq .
