# Audit System

The audit system provides an immutable audit trail for tracking operations across resource types in the IOC CFN service. All audit endpoints are internal-only (`/api/internal/`).

## Architecture

```
pkg/audit/audit.go            — Model, enums, validation, DB operations (GORM)
pkg/audit/audit_test.go        — Unit tests (SQLite in-memory)
pkg/app/handlers_audit.go      — HTTP handlers (create, get, list, delete)
pkg/app/handlers_audit_test.go — Handler tests
pkg/app/routes.go              — Route registration
pkg/client/database.go         — Database interface + MockDatabase
pkg/client/database/database.go — Real (Postgres) Database implementation
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

Both enums are validated on create and list operations. Invalid values return an error with the list of valid options.

## Validation

- `ValidateResourceType(rt string) error` — returns error if `rt` is not in the valid set
- `ValidateAuditType(at string) error` — returns error if `at` is not in the valid set
- Validation errors contain `"invalid"` in the message, which handlers use to distinguish 400 vs 500 responses

## API Endpoints

All routes are registered in `pkg/app/routes.go` under `internalPrefix` (`/api/internal`).

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/api/internal/audit-events` | `createAuditEventHandler` | Create an audit event |
| `GET` | `/api/internal/audit-events` | `listAuditEventsHandler` | List events (optional filters) |
| `GET` | `/api/internal/audit-events/{eventId}` | `getAuditEventHandler` | Get single event by UUID |
| `DELETE` | `/api/internal/audit-events/{eventId}` | `deleteAuditEventHandler` | Delete event by UUID |

### Create (`POST`)

- Decodes `CreateAuditEventRequest` from JSON body
- Requires: `resource_type`, `resource_identifier`, `audit_type`, `audit_resource_identifier`
- Validates resource type and audit type enums
- Auto-generates `id`, `created_on`, `last_modified_on`
- Returns `200 OK` with `{"message": "entry created"}`
- Returns `400` for invalid JSON, missing fields, or invalid enum values
- Returns `500` for DB errors

### List (`GET`)

- Optional query params: `resource_type`, `audit_type`
- Validates filter values if provided
- Results ordered by `created_on DESC`
- Returns `200 OK` with JSON array of audit events
- Returns `400` for invalid filter values

### Get (`GET`)

- Path param: `eventId` (UUID)
- Returns `200 OK` with single audit event JSON
- Returns `400` for invalid UUID
- Returns `404` if not found

### Delete (`DELETE`)

- Path param: `eventId` (UUID)
- Returns `204 No Content` on success
- Returns `400` for invalid UUID
- Returns `404` if not found

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

```go
type Database interface {
    CreateAuditEvent(*audit.Audit) error
    GetAuditEventByID(uuid.UUID) (*audit.Audit, error)
    ListAuditEvents(resourceType, auditType string) ([]audit.Audit, error)
    DeleteAuditEventByID(uuid.UUID) error
}
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
- `TestDeleteAuditEventByID`
- `TestEnumConstants` — verifies all enum string values

### Handler Tests (`pkg/app/handlers_audit_test.go`)

Tests HTTP handlers using `MockDatabase`.

## Handler Audit Trails

All audit information is stored as JSON in the `audit_information` field.

### Crete or Update Shared Memories (`createOrUpdateSharedMemoriesHandler`)

Emits two audit rows per operation: a **start** event before the call and an **end** event on completion. (todo: keep as is for now, need to revisit later)

| Phase | Resource Type | Audit Type | Resource Identifier | Audit Resource Identifier | Audit Information |
|-------|--------------|------------|---------------------|--------------------------|-------------------|
| Start | `MAS` | `KNOWLEDGE_INGESTION` | `masId` | `masId` | `{"status":"STARTED"}` |
| Success | `MEMORY_PROVIDER` | `KNOWLEDGE_INGESTION` | `masId` | `masId` | `{"status":"SUCCESS"}` |
| Failure | `MEMORY_PROVIDER` | `KNOWLEDGE_INGESTION` | `masId` | `masId` | `{"status":"FAILED","error":"..."}` |

### Fetch Shared Memories (`fetchSharedMemoriesHandler`)

Emits a **single audit row** per operation (no STARTED entry). The row is created only after the operation completes (SUCCESS or FAILED).

| Resource Type | Audit Type | Resource Identifier | Audit Resource Identifier |
|--------------|------------|---------------------|---------------------------|
| `MEMORY_PROVIDER` | `KNOWLEDGE_QUERY` | `masId` | `masId` |

| Outcome | Audit Information |
|---------|-------------------|
| Success | `{"status":"SUCCESS"}` |
| Failure | `{"status":"FAILED","error":"..."}` |

### Memory Operations (`memoryOperationsHandler`)

Emits a **single audit row** per operation (no STARTED entry). The row is created only after the operation completes (SUCCESS or FAILED) and includes the full request and response in `audit_information`.

| Resource Type | Audit Type | Resource Identifier | Audit Resource Identifier |
|--------------|------------|---------------------|---------------------------|
| `MEMORY_PROVIDER` | `MEMORY_OPERATION` | `masId` | `agentId` |

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

1. **Immutability**: No update endpoint exists. Events are append-only (delete is internal-only).
2. **Enum validation**: Both resource types and audit types are validated server-side with clear error messages.
3. **Error classification**: Handlers check for `"invalid"` in error messages to return 400 vs 500.
4. **UUID primary keys**: All event IDs are UUIDs, auto-generated on creation.
5. **JSONB storage**: `audit_information` uses PostgreSQL JSONB for flexible structured data.
6. **Ordering**: List results are always ordered by `created_on DESC` (newest first).
