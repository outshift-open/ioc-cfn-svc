# Audit System

The audit system provides an immutable audit trail for tracking operations across resource types in the IOC CFN service. All audit endpoints are internal-only (`/api/internal/`).

## Architecture

```
pkg/audit/audit.go              — Model, enums, validation, DB operations (GORM)
pkg/audit/audit_test.go          — Unit tests (SQLite in-memory)
pkg/app/audit_resource_ids.go    — Fetches shared_memory.id & agentic_memory.id from summary API
pkg/app/handlers_audit.go        — HTTP handlers (get, list)
pkg/app/handlers_audit_test.go   — Handler tests
pkg/app/routes.go                — Route registration
pkg/client/database.go           — Database interface + MockDatabase
pkg/client/database/database.go  — Real (Postgres) Database implementation
```

## Data Model

### `Audit` struct (`pkg/audit/audit.go`)

| Field | Type | DB Constraint | JSON Key | Required             |
|-------|------|---------------|----------|----------------------|
| `ID` | `uuid.UUID` | `uuid; primaryKey` | `id` | Auto-generated       |
| `OperationID` | `*string` | `size:128` | `operation_id` | No(TBD- will change) |
| `ResourceType` | `string` | `size:64; not null` | `resource_type` | Yes                  |
| `ResourceIdentifier` | `string` | `size:128; not null` | `resource_identifier` | Yes                  |
| `AuditType` | `string` | `size:64; not null` | `audit_type` | Yes                  |
| `AuditResourceIdentifier` | `string` | `size:128; not null` | `audit_resource_identifier` | Yes                  |
| `AuditInformation` | `datatypes.JSON` | `type:jsonb` | `audit_information` | No                   |
| `AuditExtraInformation` | `*string` | — | `audit_extra_information` | No                   |
| `CreatedBy` | `uuid.UUID` | `uuid; not null` | `created_by` | Yes                  |
| `CreatedOn` | `time.Time` | `not null` | `created_on` | Auto-set             |
| `LastModifiedBy` | `uuid.UUID` | `uuid; not null` | `last_modified_by` | Yes                  |
| `LastModifiedOn` | `time.Time` | `not null` | `last_modified_on` | Auto-set             |

## Enums

### Resource Types

| Constant | Value |
|----------|-------|
| `ResourceTypeCognitionEngine` | `COGNITION_ENGINE` |
| `ResourceTypePolicyEnforcer` | `POLICY_ENFORCER` |
| `ResourceTypeMemoryProvider` | `MEMORY_PROVIDER` |
| `ResourceTypeMAS` | `MAS` |
| `ResourceTypeMASAgent` | `MAS-AGENT` |
| `ResourceTypeWorkflow` | `WORKFLOW` |
| `ResourceTypeTask` | `TASK` |

### Audit Types

| Constant | Value |
|----------|-------|
| `AuditTypeResourceCreated` | `RESOURCE_CREATED` |
| `AuditTypeResourceUpdated` | `RESOURCE_UPDATED` |
| `AuditTypeResourceDeleted` | `RESOURCE_DELETED` |
| `AuditTypeResourcePurged` | `RESOURCE_PURGED` |
| `AuditTypeResourcePruned` | `RESOURCE_PRUNED` |
| `AuditTypeKnowledgeIngestion` | `KNOWLEDGE_INGESTION` |
| `AuditTypeKnowledgeQuery` | `KNOWLEDGE_QUERY` |
| `AuditTypeMemoryOperation` | `MEMORY_OPERATION` |
| `AuditTypeSharedMemoryOperation` | `SHARED_MEMORY_OPERATION` |
| `AuditTypeAgentMemoryOperation` | `AGENT_MEMORY_OPERATION` |
| `AuditTypeSemanticNegotiation` | `SEMANTIC_NEGOTIATION` |

Both enums are validated on create and list operations. Invalid values return an error with the list of valid options.

## Validation

- `ValidateResourceType(rt string) error` — returns error if `rt` is not in the valid set
- `ValidateAuditType(at string) error` — returns error if `at` is not in the valid set
- Validation errors contain `"invalid"` in the message, which handlers use to distinguish 400 vs 500 responses

## API Endpoints

All routes are registered in `pkg/app/routes.go` under `internalPrefix` (`/api/internal`).

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/api/internal/mgmt/audit` | `listAuditEventsHandler` | List events (optional filters) |
| `GET` | `/api/internal/mgmt/audit/{eventId}` | `getAuditEventHandler` | Get single event by UUID |

### List (`GET`)

- Optional query params: `resource_type`, `audit_type`, `page`, `pageSize`
- `page`: 0-based page number (default `0`, must be `>= 0`)
- `pageSize`: number of results per page (default `20`, max `100`, must be `>= 1`)
- Validates filter values if provided
- Results ordered by `created_on DESC`
- Returns `200 OK` with JSON object containing `data` array and `pageInfo`
- Returns `400` for invalid filter values or invalid pagination parameters

#### Response Shape

```json
{
  "data": [ ... ],
  "pageInfo": {
    "page": 0,
    "pageSize": 20,
    "pageCount": 20,
    "totalElements": 157
  }
}
```

| `pageInfo` Field | Description |
|------------------|-------------|
| `page` | Current 0-based page number |
| `pageSize` | Requested page size (clamped to `[1, MaxPageSize]`) |
| `pageCount` | Number of elements returned in this page |
| `totalElements` | Total number of matching records across all pages |

### Get (`GET`)

- Path param: `eventId` (UUID)
- Returns `200 OK` with single audit event JSON
- Returns `400` for invalid UUID
- Returns `404` if not found

### Query Parameters

| Param | Default | Constraints | Description |
|-------|---------|-------------|-------------|
| `page` | `0` | `>= 0` | 0-based page number |
| `pageSize` | `20` | `1` – configured `MaxPageSize` | Number of records per page (configurable via `DEFAULT_PAGE_SIZE` / `MAX_PAGE_SIZE` env vars; fallback defaults: `20` / `100`) |
| `resource_type` | *(none)* | Must be a valid Resource Type | Filter by resource type |
| `audit_type` | *(none)* | Must be a valid Audit Type | Filter by audit type |

### `curl` Examples

#### List — no filters
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit"
```

#### List — with pagination
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?page=0&pageSize=50"
```

#### List — filter by resource_type
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=COGNITION_ENGINE"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=POLICY_ENFORCER"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MEMORY_PROVIDER"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS-AGENT"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=WORKFLOW"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=TASK"
```

#### List — filter by audit_type
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=RESOURCE_CREATED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=RESOURCE_UPDATED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=RESOURCE_DELETED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=RESOURCE_PURGED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=RESOURCE_PRUNED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=KNOWLEDGE_INGESTION"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=KNOWLEDGE_QUERY"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=MEMORY_OPERATION"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=SHARED_MEMORY_OPERATION"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=AGENT_MEMORY_OPERATION"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?audit_type=SEMANTIC_NEGOTIATION"
```

#### List — both filters
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS&audit_type=RESOURCE_CREATED"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS-AGENT&audit_type=AGENT_MEMORY_OPERATION"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=COGNITION_ENGINE&audit_type=RESOURCE_DELETED"
```

#### List — both filters + pagination
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS&audit_type=RESOURCE_CREATED&page=0&pageSize=50"
curl -X GET "http://localhost:8080/api/internal/mgmt/audit?resource_type=MAS-AGENT&audit_type=AGENT_MEMORY_OPERATION&page=2&pageSize=50"
```

#### Get by ID
```bash
curl -X GET "http://localhost:8080/api/internal/mgmt/audit/{audit_event_id}"
```

## Database & Schema

- **Database engine**: PostgreSQL
- **Database name**: `cfn_cp` (configured via `DB_NAME` env var)
- **Table name**: `audits` (GORM auto-pluralizes `Audit` struct)
- **Schema**: `public` (default)

### Table: `audits`

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | `uuid` | `PRIMARY KEY` |
| `operation_id` | `varchar(128)` | nullable |
| `resource_type` | `varchar(64)` | `NOT NULL` |
| `resource_identifier` | `varchar(128)` | `NOT NULL` |
| `audit_type` | `varchar(64)` | `NOT NULL` |
| `audit_resource_identifier` | `varchar(128)` | `NOT NULL` |
| `audit_information` | `jsonb` | nullable |
| `audit_extra_information` | `text` | nullable |
| `created_by` | `uuid` | `NOT NULL` |
| `created_on` | `timestamp` | `NOT NULL` |
| `last_modified_by` | `uuid` | `NOT NULL` |
| `last_modified_on` | `timestamp` | `NOT NULL` |

Migration is handled via `audit.MigrateUp(db)` which calls `db.AutoMigrate(&Audit{})`.

## Database Layer

### Interface (`pkg/client/database.go`)

Audit-related methods on the `Database` interface:

```go
CreateAuditEvent(*audit.Audit) error
GetAuditEventByID(uuid.UUID) (*audit.Audit, error)
ListAuditEvents(resourceType, auditType string, page, pageSize int) (*audit.AuditListResponse, error)
DeleteAuditEventByID(uuid.UUID) error
```

### Real Implementation (`pkg/client/database/database.go`)

Delegates to package-level functions in `pkg/audit/audit.go` which operate on `*gorm.DB` directly. Uses PostgreSQL with GORM. Migration is handled via `audit.MigrateUp(db)` called from `Database.MigrateUp()`.

### Mock Implementation (`pkg/client/database.go`)

In-memory `map[uuid.UUID]*audit.Audit` with `sync.Mutex` for thread safety. Performs the same enum validation as the real implementation. Used in handler tests.

## Tests

### Unit Tests (`pkg/audit/audit_test.go`)

Uses SQLite in-memory DB via GORM. Covers:
- `TestMigrateUp` — table creation
- `TestCreateAuditEvent` — insert with all fields, verifies auto-generated ID/timestamps
- `TestGetAuditEventByID` / `TestGetAuditEventByID_NotFound`
- `TestListAuditEvents_NoFilters` — returns all events
- `TestListAuditEvents_FilterByResourceType`
- `TestListAuditEvents_FilterByAuditType`
- `TestListAuditEvents_FilterByBoth`
- `TestListAuditEvents_WithPagination` — page/pageSize, last page partial, page past end
- `TestListAuditEvents_DefaultsAndClamping` — pageSize=0/negative defaults, pageSize>max clamped, negative page
- `TestSetPaginationConfig` — override, default>max clamp, zero/negative ignored
- `TestListAuditEvents_EmptyDB` — empty result set, Data non-nil, all zeros
- `TestDeleteAuditEventByID` / `TestDeleteAuditEventByID_NotFound`
- `TestEnumConstants` — verifies all enum string values

### Handler Tests (`pkg/app/handlers_audit_test.go`)

Tests HTTP handlers using `MockDatabase`. Covers:
- `TestGetAuditEventHandler` / `TestGetAuditEventHandler_InvalidUUID` / `TestGetAuditEventHandler_NotFound`
- `TestListAuditEventsHandler` — no filters (default pagination)
- `TestListAuditEventsHandler_WithFilters`
- `TestListAuditEventsHandler_WithPagination`
- `TestListAuditEventsHandler_WithFiltersAndPagination`
- `TestListAuditEventsHandler_PageSizeExceedsMax` — pageSize=99999 clamped to MaxPageSize
- `TestListAuditEventsHandler_InvalidPage`
- `TestListAuditEventsHandler_InvalidPageSize`
- `TestListAuditEventsHandler_InvalidResourceTypeFilter`
- `TestListAuditEventsHandler_InvalidAuditTypeFilter`

## Handler Audit Trails

All audit information is stored as JSON in the `audit_information` field.

### Create or Update Shared Memories (`createOrUpdateSharedMemoriesHandler`)

Emits a **single audit row** per operation (no STARTED entry). The row is created only after the operation completes (SUCCESS or FAILED).

| Resource Type | Audit Type              | Resource Identifier | Audit Resource Identifier |
|---------------|-------------------------|---------------------|---------------------------|
| `MAS`         | `KNOWLEDGE_INGESTION`   | `masId`             | `shared_memory.id` from summary API (falls back to `masId` if not yet fetched) |

| Outcome | Audit Information |
|---------|-------------------|
| Success | `{"status":"SUCCESS"}` |
| Failure (extraction error) | `{"status":"FAILED","error":"<upstream error>"}` |
| Failure (upsert error) | `{"status":"FAILED","error":"<upstream error>"}` |

### Fetch Shared Memories (`fetchSharedMemoriesHandler`)

Emits a **single audit row** per operation (no STARTED entry). The row is created only after the operation completes (SUCCESS or FAILED).

| Resource Type | Audit Type                 | Resource Identifier | Audit Resource Identifier |
|---------------|----------------------------|---------------------|---------------------------|
| `MAS`         | `SHARED_MEMORY_OPERATION`  | `masId`             | `shared_memory.id` from summary API (falls back to `masId` if not yet fetched) |

| Outcome | Audit Information |
|---------|-------------------|
| Success | `{"status":"SUCCESS"}` |
| Failure (reasoning error) | `{"status":"FAILED","error":"<upstream error>"}` |
| Failure (insufficient evidence) | `{"status":"FAILED","error":"Insufficient evidence to answer provided user intent"}` |

### Memory Operations (`memoryOperationsHandler`)

Emits a **single audit row** per operation (no STARTED entry). The row is created only after the operation completes (SUCCESS or FAILED) and includes the full request and response in `audit_information`.

| Resource Type | Audit Type                | Resource Identifier | <br/>|
|---------------|---------------------------|---------------------|---------------------------|
| `MAS-AGENT`   | `AGENT_MEMORY_OPERATION`  | `agentId`           | `agentic_memory.id` from summary API (falls back to `agentId` if not yet fetched) |

#### `audit_information` structure

**Success:**
```json
{
  "status": "SUCCESS",
  "http_status": 200,
  "request": {
    "http_method": "POST",
    "http_url": "https://provider.example.com/v1/memories/add",
    "http_request_body": { ... }
  },
  "response": { ... }
}
```

**Failure:**
```json
{
  "status": "FAILED",
  "error": "...",
  "request": {
    "http_method": "POST",
    "http_url": "https://provider.example.com/v1/memories/add",
    "http_request_body": { ... }
  }
}
```

### Common Fields

- **`OperationID`**: Random UUID identifying the operation (TBD — will be replaced with trace/correlation ID)
- **`CreatedBy` / `LastModifiedBy`**: Currently `uuid.Nil` (placeholder)
- **`AuditExtraInformation`**: Set to the error message string on failure events; absent on success

## Key Design Decisions

1. **Append-only audit log**: No create, update, or delete HTTP endpoints are exposed. Audit events are created internally by handlers (e.g. `fetchSharedMemoriesHandler`, `memoryOperationsHandler`) via `a.db.CreateAuditEvent(...)`. Once written, entries are never modified or removed.
2. **Enum validation**: Both resource types and audit types are validated server-side with clear error messages.
3. **Error classification**: Handlers check for `"invalid"` in error messages to return 400 vs 500.
4. **UUID primary keys**: All event IDs are UUIDs, auto-generated on creation.
5. **JSONB storage**: `audit_information` uses PostgreSQL JSONB for flexible structured data.
6. **Ordering**: List results are always ordered by `created_on DESC` (newest first).
7. **Pagination**: 0-based page numbering. `DefaultPageSize=20`, `MaxPageSize=100` (fallback defaults). Both are configurable via environment variables `DEFAULT_PAGE_SIZE` and `MAX_PAGE_SIZE` (or equivalent CLI flags `--default_page_size`, `--max_page_size`). At startup, `audit.SetPaginationConfig()` is called with the configured values. Response includes `pageInfo` with `page`, `pageSize`, `pageCount`, and `totalElements`. The pagination model is database-agnostic (no SQL terms like skip/limit/offset leak into the API).
