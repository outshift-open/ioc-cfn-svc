# Metrics Architecture: CE vs MAS Separation

## Overview

This document describes the architectural separation of Cognition Engine (CE) infrastructure metrics from Multi-Agentic System (MAS) business operation metrics.

### Key Principles

1. **CE Metrics** (`ce_metrics` table): Infrastructure/service-level metrics pushed **directly by CE**
2. **MAS Metrics** (`mas_metrics` table): Business operation metrics **extracted from CE API responses**
3. **Single unified query API**: Clients query both via `/api/cognition-engine/metrics` with filtering

---

## Database Schema

### ce_metrics Table

**Purpose**: Track Cognition Engine infrastructure health and performance.

```sql
CREATE TABLE ce_metrics (
    time          TIMESTAMPTZ NOT NULL,
    ce_id         UUID NOT NULL,           -- CE instance identifier
    metric_name   TEXT NOT NULL,           -- e.g., "ce.queue.depth", "ce.inference.latency_ms"
    value         DOUBLE PRECISION NOT NULL,
    attributes    JSONB DEFAULT '{}'::jsonb,
    
    PRIMARY KEY (ce_id, metric_name, time)
);
```

**Characteristics**:
- No workspace/MAS context (CE-level only)
- High-frequency writes (CE pushes every second)
- 90-day retention, 7-day compression
- Space-partitioned by `ce_id` (4 partitions)

**Example metrics**:
- `ce.inference.latency_ms`: Model inference time
- `ce.queue.depth`: Pending request count
- `ce.memory.usage_bytes`: Process memory
- `ce.requests.rate`: Requests per second

### mas_metrics Table

**Purpose**: Track MAS business operations with full workspace context.

```sql
CREATE TABLE mas_metrics (
    time          TIMESTAMPTZ NOT NULL,
    workspace_id  UUID NOT NULL,
    mas_id        UUID NOT NULL,
    agent_id      TEXT NOT NULL,
    metric_name   TEXT NOT NULL,
    value         DOUBLE PRECISION NOT NULL,
    attributes    JSONB DEFAULT '{}'::jsonb,
    
    PRIMARY KEY (time, workspace_id, mas_id, agent_id, metric_name)
);
```

**Characteristics**:
- Full business context (workspace, MAS, agent)
- Extracted from CE API responses (`/decide`, `/start`, etc.)
- 90-day retention, 7-day compression
- Space-partitioned by `workspace_id` and `mas_id`

**Note**: v1 does not include CE correlation (`ce_id` field). This can be added in v2 if needed for debugging "which CE instance handled this request?"

**Example metrics**:
- `llm.tokens.prompt`: Prompt tokens used
- `llm.tokens.completion`: Completion tokens
- `llm.latency_ms`: End-to-end latency
- `llm.cost_usd`: Operation cost
- `sn.negotiation.rounds`: Semantic negotiation rounds

**Future (v2)**: May add optional `ce_id UUID` field for correlation queries.

---

## Data Flow

### Flow 1: CE Infrastructure Metrics (Push)

```
┌───────────────────┐
│ Cognition Engine  │
│ (Python service)  │
└─────────┬─────────┘
          │ POST /api/internal/cognition-engine/metrics
          │ { "ce_id": "uuid", "metrics": [...] }
          ▼
┌───────────────────┐
│ ioc-cfn-svc       │
│ /ingestCEMetrics  │
└─────────┬─────────┘
          │ Validate: ce_id required, workspace_id/mas_id FORBIDDEN
          ▼
┌───────────────────┐
│ ce_metrics table  │
└───────────────────┘
```

**Key points**:
- CE authenticates with service credentials
- No workspace/MAS context (CE doesn't know which MAS is which)
- Metrics like queue depth, memory, inference latency
- Fire-and-forget async write (202 Accepted)

### Flow 2: MAS Operation Metrics (Extracted)

```
┌───────────────────┐
│ MAS Client        │
└─────────┬─────────┘
          │ POST /api/workspaces/{ws}/mas/{mas}/semantic-negotiation/decide
          │ { "session_id": "...", "content_text": "..." }
          ▼
┌───────────────────┐
│ ioc-cfn-svc       │
│ decideHandler     │
└─────────┬─────────┘
          │ Forward to CE
          ▼
┌───────────────────┐
│ Cognition Engine  │
│ POST /decide      │
└─────────┬─────────┘
          │ Response: { "status": "agreed", "meta": { "tokens": {...}, "latency_ms": 123 } }
          ▼
┌───────────────────┐
│ ioc-cfn-svc       │
│ decideHandler     │
└─────────┬─────────┘
          │ Extract meta.tokens, meta.latency_ms
          │ Build MASMetric records
          ▼
┌───────────────────┐
│ mas_metrics table │
│ (workspace, mas,  │
│  agent context)   │
└───────────────────┘
```

**Key points**:
- MAS context already known (from request path)
- Extract `meta` field from CE response
- Convert to standard metric names: `llm.tokens.prompt`, `llm.latency_ms`
- Fire-and-forget async write

---

## API Endpoints

### POST /api/internal/cognition-engine/metrics (CE Push)

**Purpose**: CE pushes its own infrastructure metrics.

**Request**:
```json
{
  "ce_id": "550e8400-e29b-41d4-a716-446655440000",
  "attributes": {
    "hostname": "ce-prod-01",
    "region": "us-west-2"
  },
  "metrics": [
    {
      "timestamp": "2026-05-27T10:30:00Z",
      "name": "ce.inference.latency_ms",
      "value": 45.2,
      "attributes": { "model": "claude-4-6" }
    },
    {
      "name": "ce.queue.depth",
      "value": 12
    }
  ]
}
```

**Validation**:
- ✅ `ce_id` (UUID) **required**
- ❌ `workspace_id`, `mas_id`, `agent_id` **forbidden** (return 400)
- ✅ `metrics` array: 1-10,000 items
- ✅ Finite values (no NaN/Infinity)

**Response**: `202 Accepted` (fire-and-forget)

**Authentication**: Service credentials (not workspace-scoped)

### GET /api/cognition-engine/metrics (Unified Query)

**Purpose**: Query both CE and MAS metrics with unified interface.

**Query Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `start_time` | string | ✅ | Unix timestamp, RFC3339, or date |
| `end_time` | string | ✅ | Unix timestamp, RFC3339, or date |
| `entity_type` | string | ❌ | `"ce"` or `"mas"` (default: both via UNION) |
| `workspace_id` | UUID | ❌ | Filter MAS metrics by workspace |
| `mas_id` | UUID | ❌ | Filter MAS metrics by MAS |
| `agent_id` | string | ❌ | Filter MAS metrics by agent |
| `ce_id` | UUID | ❌ | Filter CE metrics by CE instance |
| `metric_name` | string | ❌ | Exact match or wildcard (`llm.token.*`) |
| `limit` | int | ❌ | Max results (default 1000, max 10000) |
| `offset` | int | ❌ | Pagination offset |

**Response**:
```json
{
  "period": {
    "start": "2026-05-27T00:00:00Z",
    "end": "2026-05-27T23:59:59Z"
  },
  "filters": {
    "entity_type": "mas",
    "workspace_id": "...",
    "metric_name": "llm.tokens.*"
  },
  "pagination": {
    "limit": 1000,
    "offset": 0,
    "total": 4523
  },
  "metrics": [
    {
      "timestamp": "2026-05-27T10:30:00.123Z",
      "entity_type": "mas",
      "workspace_id": "...",
      "mas_id": "...",
      "agent_id": "negotiator-1",
      "metric_name": "llm.tokens.prompt",
      "value": 1234,
      "attributes": { "model": "claude-sonnet-4-6", "service": "semantic_negotiation" }
    },
    {
      "timestamp": "2026-05-27T10:30:05.456Z",
      "entity_type": "ce",
      "ce_id": "550e8400-...",
      "metric_name": "ce.inference.latency_ms",
      "value": 45.2,
      "attributes": { "model": "claude-4-6", "hostname": "ce-prod-01" }
    }
  ]
}
```

**Query Strategy**:
1. If `entity_type=ce`: Query `ce_metrics` only
2. If `entity_type=mas`: Query `mas_metrics` only
3. If no `entity_type` (default): `UNION ALL` from both tables, sorted by `time DESC`

**Filtering Logic**:
- `workspace_id`, `mas_id`, `agent_id`: Only apply to MAS metrics (ignored for CE)
- `ce_id`: Only applies to CE metrics (`ce_metrics.ce_id`)
- `metric_name`: Apply to both with wildcard support

---

## Implementation Changes

### 1. Update Push Endpoint Handler

**File**: `pkg/app/handlers_metrics.go`

**Current**: `ingestMetricsHandler` accepts `workspace_id`, `mas_id`, `agent_id`

**New**: Create separate handler `ingestCEMetricsHandler`

```go
// IngestCEMetricsRequest for CE infrastructure metrics
type IngestCEMetricsRequest struct {
    CEID       string                 `json:"ce_id"`      // Required
    Attributes map[string]interface{} `json:"attributes"` // Batch-level
    Metrics    []MetricDataPoint      `json:"metrics"`
}

func (a *App) ingestCEMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
    var req IngestCEMetricsRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
            "error": "invalid_payload",
        })
    }

    // Validate: ce_id required
    if req.CEID == "" {
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
            "error": "ce_id is required",
        })
    }

    ceID, err := uuid.Parse(req.CEID)
    if err != nil {
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
            "error": "ce_id must be valid UUID",
        })
    }

    // Validate metrics array
    if len(req.Metrics) == 0 || len(req.Metrics) > 10000 {
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
            "error": "metrics array must contain 1-10000 items",
        })
    }

    // Store asynchronously
    go a.storeCEMetricsBatch(req, ceID)

    return eh.RespondWithJSON(w, http.StatusAccepted, map[string]interface{}{
        "status":   "accepted",
        "received": len(req.Metrics),
    })
}

func (a *App) storeCEMetricsBatch(req IngestCEMetricsRequest, ceID uuid.UUID) {
    log := getLogger()

    db, ok := a.db.(*database.Database)
    if !ok {
        log.Errorf("Failed to type-assert database")
        return
    }

    var records []metric.CEMetric
    now := time.Now()

    for i, m := range req.Metrics {
        if m.Name == "" {
            log.Warnf("Skipping metric %d: empty name", i)
            continue
        }

        if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
            log.Warnf("Skipping metric %s: non-finite value", m.Name)
            continue
        }

        timestamp := now
        if m.Timestamp != nil {
            timestamp = *m.Timestamp
        }

        // Merge batch + metric attributes
        finalAttributes := make(map[string]interface{})
        if req.Attributes != nil {
            for k, v := range req.Attributes {
                finalAttributes[k] = v
            }
        }
        if m.Attributes != nil {
            for k, v := range m.Attributes {
                finalAttributes[k] = v
            }
        }

        attributesJSON := datatypes.JSON([]byte("{}"))
        if len(finalAttributes) > 0 {
            if attrBytes, err := json.Marshal(finalAttributes); err == nil {
                attributesJSON = datatypes.JSON(attrBytes)
            }
        }

        records = append(records, metric.CEMetric{
            Time:       timestamp,
            CEID:       ceID,
            MetricName: m.Name,
            Value:      m.Value,
            Attributes: attributesJSON,
        })
    }

    if len(records) > 0 {
        if err := db.Create(&records).Error; err != nil {
            log.Errorf("Failed to store %d CE metrics: %v", len(records), err)
        } else {
            log.Infof("Stored %d CE metrics for ce_id=%s", len(records), ceID)
        }
    }
}
```

**Deprecate old endpoint**: Keep `ingestMetricsHandler` for backward compatibility but log deprecation warning. Eventually rename to `ingestMASMetricsHandler` for explicit clarity (though extraction is preferred).

### 2. Extract MAS Metrics from CE Responses

**Pattern**: Already implemented in `storeTokenMetricsAsync()`.

**Extend to all CE endpoints**:
- `/semantic-negotiation/start` → extract `meta`
- `/semantic-negotiation/decide` → extract `meta`
- `/knowledge/extract` → extract `meta`
- `/shared-memory/*` → extract `meta`

### 3. Refactor Query Endpoint

**File**: `pkg/app/handlers_metrics.go`

**Current**: `getMetricsHandler` queries only `mas_metrics`

**New**: Support `entity_type` parameter and UNION query

```go
func (a *App) getMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
    log := getLogger()

    // Parse time range
    startTime, endTime, err := parseTimeRange(r)
    if err != nil {
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Parse entity_type filter
    entityType := r.URL.Query().Get("entity_type") // "ce", "mas", or empty (both)

    // Parse other filters
    workspaceIDStr := r.URL.Query().Get("workspace_id")
    masIDStr := r.URL.Query().Get("mas_id")
    agentID := r.URL.Query().Get("agent_id")
    ceIDStr := r.URL.Query().Get("ce_id")
    metricName := r.URL.Query().Get("metric_name")

    // Pagination
    limit, offset := parsePagination(r)

    db, ok := a.db.(*database.Database)
    if !ok {
        return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "database_error"})
    }

    var records []MetricRecord
    var total int64

    switch entityType {
    case "ce":
        records, total, err = a.queryCEMetrics(db, startTime, endTime, ceIDStr, metricName, limit, offset)
    case "mas":
        records, total, err = a.queryMASMetrics(db, startTime, endTime, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName, limit, offset)
    case "":
        // Query both with UNION ALL
        records, total, err = a.queryUnifiedMetrics(db, startTime, endTime, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName, limit, offset)
    default:
        return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "entity_type must be 'ce' or 'mas'"})
    }

    if err != nil {
        log.Errorf("Query failed: %v", err)
        return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "query_failed"})
    }

    // Build response
    response := MetricsQueryResponse{
        Period:     Period{Start: startTime.Format(time.RFC3339), End: endTime.Format(time.RFC3339)},
        Filters:    buildFilters(entityType, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName),
        Pagination: Pagination{Limit: limit, Offset: offset, Total: int(total)},
        Metrics:    records,
    }

    return eh.RespondWithJSON(w, http.StatusOK, response)
}

func (a *App) queryCEMetrics(db *gorm.DB, startTime, endTime time.Time, ceIDStr, metricName string, limit, offset int) ([]MetricRecord, int64, error) {
    query := db.Model(&metric.CEMetric{}).
        Where("time >= ? AND time < ?", startTime, endTime)

    if ceIDStr != "" {
        ceID, err := uuid.Parse(ceIDStr)
        if err != nil {
            return nil, 0, fmt.Errorf("ce_id must be valid UUID")
        }
        query = query.Where("ce_id = ?", ceID)
    }

    if metricName != "" {
        query = applyMetricNameFilter(query, metricName)
    }

    var total int64
    query.Count(&total)

    var ceRecords []metric.CEMetric
    if err := query.Order("time DESC").Limit(limit).Offset(offset).Find(&ceRecords).Error; err != nil {
        return nil, 0, err
    }

    records := make([]MetricRecord, len(ceRecords))
    for i, r := range ceRecords {
        var attrs map[string]interface{}
        json.Unmarshal([]byte(r.Attributes), &attrs)

        records[i] = MetricRecord{
            Timestamp:  r.Time.Format(time.RFC3339Nano),
            EntityType: "ce",
            CEID:       r.CEID.String(),
            MetricName: r.MetricName,
            Value:      r.Value,
            Attributes: attrs,
        }
    }

    return records, total, nil
}

func (a *App) queryMASMetrics(db *gorm.DB, startTime, endTime time.Time, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName string, limit, offset int) ([]MetricRecord, int64, error) {
    query := db.Model(&metric.MASMetric{}).
        Where("time >= ? AND time < ?", startTime, endTime)

    if workspaceIDStr != "" {
        workspaceID, err := uuid.Parse(workspaceIDStr)
        if err != nil {
            return nil, 0, fmt.Errorf("workspace_id must be valid UUID")
        }
        query = query.Where("workspace_id = ?", workspaceID)
    }

    if masIDStr != "" {
        masID, err := uuid.Parse(masIDStr)
        if err != nil {
            return nil, 0, fmt.Errorf("mas_id must be valid UUID")
        }
        query = query.Where("mas_id = ?", masID)
    }

    if agentID != "" {
        query = query.Where("agent_id = ?", agentID)
    }

    if metricName != "" {
        query = applyMetricNameFilter(query, metricName)
    }

    var total int64
    query.Count(&total)

    var masRecords []metric.MASMetric
    if err := query.Order("time DESC").Limit(limit).Offset(offset).Find(&masRecords).Error; err != nil {
        return nil, 0, err
    }

    records := make([]MetricRecord, len(masRecords))
    for i, r := range masRecords {
        var attrs map[string]interface{}
        json.Unmarshal([]byte(r.Attributes), &attrs)

        records[i] = MetricRecord{
            Timestamp:   r.Time.Format(time.RFC3339Nano),
            EntityType:  "mas",
            WorkspaceID: r.WorkspaceID.String(),
            MASID:       r.MASID.String(),
            AgentID:     r.AgentID,
            MetricName:  r.MetricName,
            Value:       r.Value,
            Attributes:  attrs,
        }
    }

    return records, total, nil
}

func (a *App) queryUnifiedMetrics(db *gorm.DB, startTime, endTime time.Time, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName string, limit, offset int) ([]MetricRecord, int64, error) {
    // Build UNION ALL query
    // Note: This is simplified - actual implementation needs careful column mapping
    
    // For now, query separately and merge
    ceRecords, ceTotal, err1 := a.queryCEMetrics(db, startTime, endTime, ceIDStr, metricName, limit/2, offset)
    masRecords, masTotal, err2 := a.queryMASMetrics(db, startTime, endTime, workspaceIDStr, masIDStr, agentID, ceIDStr, metricName, limit/2, offset)
    
    if err1 != nil || err2 != nil {
        return nil, 0, fmt.Errorf("unified query failed")
    }
    
    // Merge and sort by timestamp
    allRecords := append(ceRecords, masRecords...)
    sort.Slice(allRecords, func(i, j int) bool {
        return allRecords[i].Timestamp > allRecords[j].Timestamp
    })
    
    return allRecords, ceTotal + masTotal, nil
}

func applyMetricNameFilter(query *gorm.DB, metricName string) *gorm.DB {
    if strings.Contains(metricName, "*") {
        pattern := strings.ReplaceAll(metricName, "*", "%")
        return query.Where("metric_name LIKE ?", pattern)
    }
    return query.Where("metric_name = ?", metricName)
}
```

### 4. Update MetricRecord DTO

```go
type MetricRecord struct {
    Timestamp   string                 `json:"timestamp"`
    EntityType  string                 `json:"entity_type"`  // "ce" or "mas"
    
    // CE fields (present when entity_type="ce")
    CEID        string                 `json:"ce_id,omitempty"`
    
    // MAS fields (present when entity_type="mas")
    WorkspaceID string                 `json:"workspace_id,omitempty"`
    MASID       string                 `json:"mas_id,omitempty"`
    AgentID     string                 `json:"agent_id,omitempty"`
    
    // Common fields
    MetricName  string                 `json:"metric_name"`
    Value       float64                `json:"value"`
    Attributes  map[string]interface{} `json:"attributes"`
}
```

### 5. Update Routes

**File**: `pkg/app/routes.go`

```go
// Add new CE metrics push endpoint
r.With(middlewares...).Post("/api/internal/cognition-engine/metrics", a.Handle(a.ingestCEMetricsHandler))

// Keep existing endpoint for backward compat (deprecated)
r.With(middlewares...).Post("/api/internal/cognition-engine/metrics/mas", a.Handle(a.ingestMetricsHandler))

// Query endpoint remains unchanged (path)
r.With(middlewares...).Get("/api/cognition-engine/metrics", a.Handle(a.getMetricsHandler))
```

---

## Migration Plan

### Phase 1: Schema (Non-Breaking)
- ✅ Add `CEMetric` model and migration
- ✅ Add optional `ce_id` field to `MASMetric`
- ✅ Run migrations on dev/staging

### Phase 2: Write Path (Non-Breaking)
- ✅ Implement `ingestCEMetricsHandler`
- ✅ Update CE service to push to new endpoint
- ⚠️ Keep old push endpoint for backward compatibility

### Phase 3: Read Path (Non-Breaking)
- ✅ Add `entity_type` parameter to query endpoint
- ✅ Implement CE/MAS/unified query logic
- ✅ Update MetricRecord DTO with `entity_type`
- ⚠️ Existing queries still work (default to MAS for compatibility)

### Phase 4: Deprecation (Breaking)
- Deprecate old push endpoint (return warning header)
- Update all CE clients to use new endpoint
- After 30 days, remove old endpoint
- Make `entity_type` required (or default to unified)

---

## Query Examples

### Example 1: CE Infrastructure Metrics
```bash
curl "http://localhost:8080/api/cognition-engine/metrics?\
entity_type=ce&\
ce_id=550e8400-e29b-41d4-a716-446655440000&\
metric_name=ce.inference.latency_ms&\
start_time=2026-05-27T00:00:00Z&\
end_time=2026-05-27T23:59:59Z"
```

### Example 2: MAS Operation Metrics
```bash
curl "http://localhost:8080/api/cognition-engine/metrics?\
entity_type=mas&\
workspace_id=123e4567-e89b-12d3-a456-426614174000&\
mas_id=987fcdeb-51a2-43d7-b890-123456789abc&\
metric_name=llm.tokens.*&\
start_time=2026-05-27&\
end_time=2026-05-28"
```

### Example 3: Unified (CE + MAS)
```bash
curl "http://localhost:8080/api/cognition-engine/metrics?\
start_time=1716768000&\
end_time=1716854400&\
limit=5000"
```

### Example 4: CE Instance Query
```bash
curl "http://localhost:8080/api/cognition-engine/metrics?\
entity_type=ce&\
ce_id=550e8400-e29b-41d4-a716-446655440000&\
start_time=2026-05-27T10:00:00Z&\
end_time=2026-05-27T11:00:00Z"
```
Returns all CE infrastructure metrics for instance `550e8400...` in that hour.

---

## Testing

### Unit Tests

1. **CE metrics push**: Valid payload, invalid payload, forbidden fields
2. **MAS metrics extraction**: Parse `meta` field, handle missing fields
3. **Query endpoint**: Entity type filtering, UNION logic, wildcard matching

### Integration Tests

1. **End-to-end CE push**: Write CE metrics, query them back
2. **End-to-end MAS extraction**: Call `/decide`, verify metrics extracted
3. **Correlation**: Push CE metric, trigger MAS op with `ce_id`, query correlation

### Performance Tests

1. **CE write throughput**: 10k metrics/sec sustained
2. **Query performance**: Sub-100ms for time-range queries with filters
3. **UNION query overhead**: Compare separate vs unified query latency

---

## Monitoring

### Metrics to Track

- `metrics_api.writes.ce`: CE metrics write rate
- `metrics_api.writes.mas`: MAS metrics write rate (extracted)
- `metrics_api.query.latency_ms`: Query endpoint p50/p95/p99
- `timescaledb.compression.ratio`: Compression effectiveness
- `timescaledb.chunk.count`: Chunk growth rate

### Alerts

- CE metrics write failure rate >1%
- MAS metrics extraction failure rate >0.1%
- Query latency p99 >500ms
- Chunk retention policy failures

---

## Open Questions

1. **CE authentication**: Service credentials or workspace-scoped JWT?
2. **CE identifier**: Does CE have a stable UUID, or generated per deployment?
3. **Response header**: Does CE include `X-CE-Instance-ID` or similar?
4. **Retention**: Different retention for CE (30d) vs MAS (90d)?
5. **Dashboards**: Grafana panels need updating for entity_type filter?

---

## References

- TimescaleDB hypertable docs: https://docs.timescale.com/use-timescale/latest/hypertables/
- GORM migrations: https://gorm.io/docs/migration.html
- Existing metrics PRs: #98 (LLM token tracking)
