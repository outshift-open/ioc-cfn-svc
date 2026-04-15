#!/bin/bash
# Generate 100 audit entries then verify pagination works
# Usage: ./docs/test-audit-pagination.sh
# Prereq: Server on :9002, data loaded via test-openclaw.sh step 1

WORKSPACE_ID="eb47e066-0414-426f-b1a0-e5222fcf50b6"
MAS_ID="5b4c2a20-7bf8-4afe-af5e-ad919cdbe624"
AGENT_ID="pqr"
AUDIT="http://localhost:9002/api/internal/mgmt/audit"
FETCH="http://localhost:9002/api/workspaces/${WORKSPACE_ID}/multi-agentic-systems/${MAS_ID}/shared-memories/query"

echo "=== Baseline ==="
BEFORE=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq '.pageInfo.totalElements')
echo "Before: ${BEFORE} audit events"

echo ""
echo "=== Sending 100 fetch queries ==="
for i in $(seq 1 100); do
  curl -s -o /dev/null -X POST "${FETCH}" \
    -H "Content-Type: application/json" \
    -d '{"header":{"agent_id":"'"${AGENT_ID}"'"},"search_strategy":"semantic_graph_traversal","intent":"Tell me about the Q2 budget planning."}'
  [ $((i % 10)) -eq 0 ] && echo "  ${i}/100 done"
done

echo ""
echo "=== Verify count ==="
AFTER=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq '.pageInfo.totalElements')
echo "After: ${AFTER} audit events (+$((AFTER - BEFORE)) new)"

echo ""
echo "=== Pagination checks ==="

echo "-- page 0, pageSize=20 (default) --"
curl -s "${AUDIT}" | jq '.pageInfo'

echo "-- page 0, pageSize=10 --"
curl -s "${AUDIT}?page=0&pageSize=10" | jq '.pageInfo'

echo "-- page 1, pageSize=10 --"
curl -s "${AUDIT}?page=1&pageSize=10" | jq '.pageInfo'

echo "-- page past end --"
curl -s "${AUDIT}?page=9999&pageSize=10" | jq '.pageInfo'

echo "-- pageSize=99999 (should clamp to 100) --"
curl -s "${AUDIT}?pageSize=99999" | jq '.pageInfo.pageSize'

echo "-- filter resource_type=MAS --"
curl -s "${AUDIT}?resource_type=MAS&page=0&pageSize=5" | jq '.pageInfo'

echo "-- get single event by ID --"
ID=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq -r '.data[0].id')
echo "ID: ${ID}"
curl -s "${AUDIT}/${ID}" | jq '{id, resource_type, audit_type}'

echo ""
echo "Done. Total: ${AFTER} audit events."
