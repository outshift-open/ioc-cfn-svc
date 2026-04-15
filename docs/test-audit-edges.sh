#!/bin/bash
# Test all audit edge cases: pagination, errors, empty results
# Usage: ./docs/test-audit-edges.sh
# Prereq: Server on :9002 with some audit data (run test-audit-pagination.sh first)

AUDIT="http://localhost:9002/api/internal/mgmt/audit"

echo "========================================="
echo " AUDIT EDGE CASES"
echo "========================================="

# --- Pagination ---

echo ""
echo "-- Default (no params) --"
curl -s "${AUDIT}" | jq '{pageInfo, data: [.data[] | {id, resource_type, audit_type, status: .audit_information.status}]}'

echo "-- page=0, pageSize=2 --"
curl -s "${AUDIT}?page=0&pageSize=2" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type, status: .audit_information.status}]}'

echo "-- page=1, pageSize=2 --"
curl -s "${AUDIT}?page=1&pageSize=2" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type, status: .audit_information.status}]}'

echo "-- page=0, pageSize=100 (max) --"
curl -s "${AUDIT}?page=0&pageSize=100" | jq '{pageInfo, first: (.data[0] | {id: .id[:8], resource_type, audit_type}), last: (.data[-1] | {id: .id[:8], resource_type, audit_type})}'

echo "-- last page (partial) --"
TOTAL=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq '.pageInfo.totalElements')
LAST_PAGE=$(( (TOTAL - 1) / 3 ))
echo "  totalElements=${TOTAL}, last page at page=${LAST_PAGE} with pageSize=3"
curl -s "${AUDIT}?page=${LAST_PAGE}&pageSize=3" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type}]}'

echo "-- page past end --"
curl -s "${AUDIT}?page=9999&pageSize=10" | jq '.'

echo "-- pageSize=99999 (clamp to 100) --"
curl -s "${AUDIT}?pageSize=99999" | jq '.pageInfo'

# --- Filters ---

echo ""
echo "-- filter: resource_type=MAS, pageSize=3 --"
curl -s "${AUDIT}?resource_type=MAS&page=0&pageSize=3" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type, status: .audit_information.status}]}'

echo "-- filter: audit_type=SHARED_MEMORY_OPERATION, pageSize=3 --"
curl -s "${AUDIT}?audit_type=SHARED_MEMORY_OPERATION&page=0&pageSize=3" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type, status: .audit_information.status}]}'

echo "-- filter: both, pageSize=3 --"
curl -s "${AUDIT}?resource_type=MAS&audit_type=SHARED_MEMORY_OPERATION&page=0&pageSize=3" | jq '{pageInfo, data: [.data[] | {id: .id[:8], resource_type, audit_type, status: .audit_information.status}]}'

echo "-- filter: no results (TASK) --"
curl -s "${AUDIT}?resource_type=TASK" | jq '.'

echo "-- filter: no results (COGNITION_ENGINE) --"
curl -s "${AUDIT}?resource_type=COGNITION_ENGINE" | jq '.'

# --- Get by ID ---

echo ""
echo "-- get single event (full detail) --"
ID=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq -r '.data[0].id')
echo "  ID: ${ID}"
curl -s "${AUDIT}/${ID}" | jq '.'

# --- Error cases ---

echo ""
echo "========================================="
echo " ERROR CASES"
echo "========================================="

echo ""
echo "-- invalid resource_type --"
curl -s "${AUDIT}?resource_type=BOGUS" | jq .

echo "-- invalid audit_type --"
curl -s "${AUDIT}?audit_type=BOGUS" | jq .

echo "-- page=abc --"
curl -s "${AUDIT}?page=abc" | jq .

echo "-- page=-1 --"
curl -s "${AUDIT}?page=-1" | jq .

echo "-- pageSize=0 --"
curl -s "${AUDIT}?pageSize=0" | jq .

echo "-- pageSize=-1 --"
curl -s "${AUDIT}?pageSize=-1" | jq .

echo "-- pageSize=abc --"
curl -s "${AUDIT}?pageSize=abc" | jq .

echo "-- invalid UUID --"
curl -s "${AUDIT}/not-a-uuid" | jq .

echo "-- event not found --"
curl -s "${AUDIT}/00000000-0000-0000-0000-000000000000" | jq .

echo "-- empty string resource_type (ignored, returns all) --"
curl -s "${AUDIT}?resource_type=&page=0&pageSize=1" | jq '.pageInfo'

echo "-- empty string audit_type (ignored, returns all) --"
curl -s "${AUDIT}?audit_type=&page=0&pageSize=1" | jq '.pageInfo'

echo "-- both params invalid --"
curl -s "${AUDIT}?page=abc&pageSize=xyz" | jq .

echo "-- valid filter + invalid page --"
curl -s "${AUDIT}?resource_type=MAS&page=-1" | jq .

echo "-- valid filter + invalid pageSize --"
curl -s "${AUDIT}?resource_type=MAS&pageSize=0" | jq .

# --- Ordering ---

echo ""
echo "========================================="
echo " ORDERING"
echo "========================================="

echo ""
echo "-- verify created_on DESC order (newest first) --"
curl -s "${AUDIT}?page=0&pageSize=5" | jq '[.data[] | {id: .id[:8], created_on}]'

echo "-- compare page 0 last vs page 1 first (no gap) --"
P0_LAST=$(curl -s "${AUDIT}?page=0&pageSize=3" | jq -r '.data[-1].created_on')
P1_FIRST=$(curl -s "${AUDIT}?page=1&pageSize=3" | jq -r '.data[0].created_on')
echo "  page 0 last:  ${P0_LAST}"
echo "  page 1 first: ${P1_FIRST}"
if [[ "$P0_LAST" > "$P1_FIRST" ]] || [[ "$P0_LAST" == "$P1_FIRST" ]]; then
  echo "  OK: page 0 last >= page 1 first (DESC order maintained)"
else
  echo "  FAIL: ordering broken across pages"
fi

# --- No duplicate IDs across pages ---

echo ""
echo "========================================="
echo " NO DUPLICATE IDS"
echo "========================================="

echo ""
echo "-- check first 3 pages (pageSize=5) have unique IDs --"
IDS=""
for p in 0 1 2; do
  PAGE_IDS=$(curl -s "${AUDIT}?page=${p}&pageSize=5" | jq -r '.data[].id')
  IDS="${IDS}
${PAGE_IDS}"
done
UNIQUE=$(echo "$IDS" | sort -u | grep -c .)
RAW=$(echo "$IDS" | grep -c .)
echo "  raw=${RAW} unique=${UNIQUE}"
if [ "$RAW" -eq "$UNIQUE" ]; then
  echo "  PASS: no duplicates across pages"
else
  echo "  FAIL: found duplicates"
fi

# --- All valid resource_type filters ---

echo ""
echo "========================================="
echo " ALL VALID RESOURCE TYPES"
echo "========================================="

for RT in COGNITION_ENGINE POLICY_ENFORCER MEMORY_PROVIDER MAS MAS-AGENT WORKFLOW TASK; do
  echo ""
  echo "-- resource_type=${RT} --"
  curl -s "${AUDIT}?resource_type=${RT}&page=0&pageSize=1" | jq '.pageInfo'
done

# --- All valid audit_type filters ---

echo ""
echo "========================================="
echo " ALL VALID AUDIT TYPES"
echo "========================================="

for AT in RESOURCE_CREATED RESOURCE_UPDATED RESOURCE_DELETED RESOURCE_PURGED RESOURCE_PRUNED KNOWLEDGE_INGESTION KNOWLEDGE_QUERY MEMORY_OPERATION SHARED_MEMORY_OPERATION AGENT_MEMORY_OPERATION; do
  echo ""
  echo "-- audit_type=${AT} --"
  curl -s "${AUDIT}?audit_type=${AT}&page=0&pageSize=1" | jq '.pageInfo'
done

# --- totalElements consistency ---

echo ""
echo "========================================="
echo " CONSISTENCY"
echo "========================================="

echo ""
echo "-- totalElements same across different pages --"
T0=$(curl -s "${AUDIT}?page=0&pageSize=5" | jq '.pageInfo.totalElements')
T1=$(curl -s "${AUDIT}?page=1&pageSize=5" | jq '.pageInfo.totalElements')
T2=$(curl -s "${AUDIT}?page=99&pageSize=5" | jq '.pageInfo.totalElements')
echo "  page 0: ${T0}, page 1: ${T1}, page 99: ${T2}"
if [ "$T0" -eq "$T1" ] && [ "$T1" -eq "$T2" ]; then
  echo "  PASS: totalElements consistent"
else
  echo "  FAIL: totalElements differs across pages"
fi

echo "-- totalElements same across different pageSizes --"
S1=$(curl -s "${AUDIT}?pageSize=1" | jq '.pageInfo.totalElements')
S20=$(curl -s "${AUDIT}?pageSize=20" | jq '.pageInfo.totalElements')
S100=$(curl -s "${AUDIT}?pageSize=100" | jq '.pageInfo.totalElements')
echo "  pageSize=1: ${S1}, pageSize=20: ${S20}, pageSize=100: ${S100}"
if [ "$S1" -eq "$S20" ] && [ "$S20" -eq "$S100" ]; then
  echo "  PASS: totalElements consistent across page sizes"
else
  echo "  FAIL: totalElements differs"
fi

# --- HTTP status codes ---

echo ""
echo "========================================="
echo " HTTP STATUS CODES"
echo "========================================="

echo ""
echo "-- 200: list --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}"

echo "-- 200: list with filter --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}?resource_type=MAS"

echo "-- 200: get by ID --"
ID=$(curl -s "${AUDIT}?page=0&pageSize=1" | jq -r '.data[0].id')
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}/${ID}"

echo "-- 400: invalid resource_type --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}?resource_type=BOGUS"

echo "-- 400: invalid audit_type --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}?audit_type=BOGUS"

echo "-- 400: invalid page --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}?page=abc"

echo "-- 400: invalid pageSize --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}?pageSize=-1"

echo "-- 400: invalid UUID --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}/not-a-uuid"

echo "-- 404: event not found --"
curl -s -o /dev/null -w "  %{http_code}\n" "${AUDIT}/00000000-0000-0000-0000-000000000000"

echo ""
echo "Done."
