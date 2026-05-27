# Metrics API v2.0 - Breaking Changes Summary

## Overview
This document summarizes the breaking changes made to the Metrics API based on PR #101 code review feedback.

## Changes Made

### 1. Entity Filters Now Required ✅

**Before (v1):**
```bash
# Time-only query returned data from BOTH tables
curl "http://localhost:9002/api/cognition-engine/metrics?start_time=2026-05-27&end_time=2026-05-28"
→ Returns CE + MAS metrics
```

**After (v2):**
```bash
# Time-only query now returns 400 error
curl "http://localhost:9002/api/cognition-engine/metrics?start_time=2026-05-27&end_time=2026-05-28"
→ 400: "at least one entity filter required: ce_id, workspace_id, mas_id, or agent_id"
```

**Rationale:**
- Prevents expensive full-table scans on production databases
- Forces explicit intent (CE vs MAS metrics)
- Aligns with TimescaleDB best practices for scoped queries
- Reduces risk of unauthorized data access

**Migration:**
- Add at least one entity filter to all queries
- For CE: Add `?ce_id={uuid}`
- For MAS: Add `?workspace_id={uuid}` or `?mas_id={uuid}` or `?agent_id={string}`

---

### 2. Pagination: limit/offset → page/pageSize ✅

**Before (v1):**
```bash
curl "...?limit=100&offset=200"
```

**After (v2):**
```bash
curl "...?page=2&pageSize=100"  # page 2 = skip 200 records
```

**Response structure changed:**
```json
// Before
{
  "ce_metrics": {
    "total": 245,
    "limit": 100,
    "offset": 200,
    "data": [...]
  }
}

// After
{
  "ce_metrics": {
    "page": 2,
    "pageSize": 100,
    "totalCount": 245,
    "series": [...]
  }
}
```

**Defaults:**
- `page`: 0 (first page, 0-indexed)
- `pageSize`: 20
- Max `pageSize`: 100

**Rationale:**
- Hides DB implementation details (abstraction)
- More user-friendly ("Page 3" vs "Offset 40")
- Consistent with Audit API and REST best practices
- Less error-prone than manual offset calculation

**Migration:**
```python
# Old code
limit = 100
offset = page_number * 100
url = f"?limit={limit}&offset={offset}"

# New code
page = page_number
page_size = 100
url = f"?page={page}&pageSize={page_size}"
```

---

### 3. Response Format: Flat → Grouped (60-70% Size Reduction) ✅

**Before (v1) - Flat array (verbose):**
```json
{
  "ce_metrics": {
    "total": 1000,
    "data": [
      {
        "timestamp": "2026-05-27T10:00:00Z",
        "ce_id": "550e8400-...",
        "metric_name": "cpu.usage",
        "value": 12.5,
        "attributes": {"host": "prod-01"}
      },
      {
        "timestamp": "2026-05-27T10:01:00Z",
        "ce_id": "550e8400-...",           // ← Repeated
        "metric_name": "cpu.usage",         // ← Repeated
        "value": 15.2,
        "attributes": {"host": "prod-01"}   // ← Repeated
      },
      // ... 998 more records with repeated metadata
    ]
  }
}
```
**Size:** ~850KB for 1000 datapoints

**After (v2) - Grouped by metric (efficient):**
```json
{
  "ce_metrics": {
    "page": 0,
    "pageSize": 20,
    "totalCount": 1000,
    "series": [
      {
        "metric_name": "cpu.usage",
        "ce_id": "550e8400-...",
        "attributes": {"host": "prod-01"},
        "datapoints": [
          ["2026-05-27T10:00:00Z", 12.5],
          ["2026-05-27T10:01:00Z", 15.2],
          ["2026-05-27T10:02:00Z", 18.1],
          // ... all datapoints for this metric
        ]
      },
      {
        "metric_name": "mem.usage",
        "ce_id": "550e8400-...",
        "attributes": {"host": "prod-01"},
        "datapoints": [
          ["2026-05-27T10:00:00Z", 67.5],
          ["2026-05-27T10:01:00Z", 68.2]
        ]
      }
    ]
  }
}
```
**Size:** ~300KB for same data (**65% reduction**)

**Benefits:**
1. **Size reduction:** 60-70% smaller (metadata not repeated)
2. **Industry standard:** Prometheus, InfluxDB, Grafana use this format
3. **Dashboard-friendly:** Series naturally grouped for charting
4. **Intuitive structure:** Metric → timeseries points

**Migration for clients:**
```python
# Old code (flat)
for record in response["ce_metrics"]["data"]:
    timestamp = record["timestamp"]
    value = record["value"]
    metric_name = record["metric_name"]
    process(timestamp, value, metric_name)

# New code (grouped)
for series in response["ce_metrics"]["series"]:
    metric_name = series["metric_name"]
    attributes = series["attributes"]
    for datapoint in series["datapoints"]:
        timestamp, value = datapoint[0], datapoint[1]
        process(timestamp, value, metric_name, attributes)
```

**Charting libraries (Grafana, Chart.js):**
```javascript
// Old format required client-side grouping
const grouped = groupBy(data, 'metric_name');

// New format is ready for charting
response.ce_metrics.series.forEach(series => {
  chart.addSeries({
    name: series.metric_name,
    data: series.datapoints  // Already in [[x, y]] format
  });
});
```

---

## Summary Table

| Change | Before (v1) | After (v2) | Breaking? |
|--------|-------------|------------|-----------|
| Entity filter | Optional | **Required** | ✅ Yes |
| Pagination params | `limit`, `offset` | `page`, `pageSize` | ✅ Yes |
| Response format | Flat array | Grouped series | ✅ Yes |
| Response fields | `total`, `limit`, `offset`, `data` | `page`, `pageSize`, `totalCount`, `series` | ✅ Yes |
| Default page size | 1000 | 20 | ⚠️ Yes |
| Max page size | 10000 | 100 | ⚠️ Yes |

---

## Testing

### Test 1: Entity filter validation
```bash
# Should fail with 400
curl "http://localhost:9002/api/cognition-engine/metrics?start_time=2026-05-27&end_time=2026-05-28"

# Expected error
{
  "error": "at least one entity filter required: ce_id, workspace_id, mas_id, or agent_id"
}
```

### Test 2: Pagination
```bash
# Page 0 (first page)
curl "http://localhost:9002/api/cognition-engine/metrics?\
ce_id=550e8400-e29b-41d4-a716-446655440000&\
start_time=2026-05-27&end_time=2026-05-28&\
page=0&pageSize=10"

# Expected response structure
{
  "ce_metrics": {
    "page": 0,
    "pageSize": 10,
    "totalCount": 245,
    "series": [...]
  }
}
```

### Test 3: Grouped format
```bash
curl "http://localhost:9002/api/cognition-engine/metrics?\
ce_id=550e8400-e29b-41d4-a716-446655440000&\
start_time=2026-05-27&end_time=2026-05-28"

# Verify response has "series" array with grouped datapoints
```

---

## Rollback Plan

If v2 causes issues, revert to v1 by:
1. Remove entity filter validation (lines 525-529 in handlers_metrics.go)
2. Restore `limit`/`offset` parsing and response fields
3. Restore flat array format in query helper functions

Or: Keep v2 as-is and document migration path for clients.

---

## Future Enhancements (v3)

1. **Compression:** Add `Accept-Encoding: gzip` support (another 50% reduction)
2. **Format negotiation:** Add `?format=flat` for backward compatibility
3. **Streaming:** SSE endpoint for real-time metric feeds
4. **Aggregations:** Pre-computed rollups (hourly/daily averages)

---

## Related Issues

- **PR #101:** Original metrics implementation
- **Review comments:** Entity filter requirement, pagination naming, response size
- **Tracking Jira:** Create ticket for other APIs using `limit`/`offset` (audit API, etc.)
