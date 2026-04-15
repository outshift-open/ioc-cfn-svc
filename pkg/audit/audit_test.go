package audit

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := MigrateUp(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestMigrateUp(t *testing.T) {
	db := setupTestDB(t)
	assert.True(t, db.Migrator().HasTable(&Audit{}))
}

func TestCreateAuditEvent(t *testing.T) {
	db := setupTestDB(t)

	opID := "op-123"
	extra := "SUCCESS"
	a := &Audit{
		OperationID:             &opID,
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-456",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-456",
		AuditInformation:        datatypes.JSON([]byte(`{"foo":"bar"}`)),
		AuditExtraInformation:   &extra,
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}

	err := CreateAuditEvent(db, a)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.False(t, a.CreatedOn.IsZero())
	assert.False(t, a.LastModifiedOn.IsZero())
}

func TestGetAuditEventByID(t *testing.T) {
	db := setupTestDB(t)

	a := &Audit{
		ResourceType:            ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	err := CreateAuditEvent(db, a)
	assert.NoError(t, err)

	found, err := GetAuditEventByID(db, a.ID)
	assert.NoError(t, err)
	assert.Equal(t, a.ID, found.ID)
	assert.Equal(t, ResourceTypeMAS, found.ResourceType)
	assert.Equal(t, AuditTypeKnowledgeQuery, found.AuditType)
}

func TestGetAuditEventByID_NotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := GetAuditEventByID(db, uuid.New())
	assert.Error(t, err)
}

func TestListAuditEvents_NoFilters(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 3; i++ {
		a := &Audit{
			ResourceType:            ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		assert.NoError(t, CreateAuditEvent(db, a))
	}

	resp, err := ListAuditEvents(db, "", "", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 3)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, DefaultPageSize(), resp.PageInfo.PageSize)
	assert.Equal(t, 3, resp.PageInfo.PageCount)
	assert.Equal(t, 3, resp.PageInfo.TotalElements)
}

func TestListAuditEvents_FilterByResourceType(t *testing.T) {
	db := setupTestDB(t)

	a1 := &Audit{
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	a2 := &Audit{
		ResourceType:            ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	assert.NoError(t, CreateAuditEvent(db, a1))
	assert.NoError(t, CreateAuditEvent(db, a2))

	resp, err := ListAuditEvents(db, ResourceTypeMAS, "", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, ResourceTypeMAS, resp.Data[0].ResourceType)
}

func TestListAuditEvents_FilterByAuditType(t *testing.T) {
	db := setupTestDB(t)

	a1 := &Audit{
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	a2 := &Audit{
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               AuditTypeResourceDeleted,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	assert.NoError(t, CreateAuditEvent(db, a1))
	assert.NoError(t, CreateAuditEvent(db, a2))

	resp, err := ListAuditEvents(db, "", AuditTypeResourceDeleted, 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, AuditTypeResourceDeleted, resp.Data[0].AuditType)
}

func TestListAuditEvents_FilterByBoth(t *testing.T) {
	db := setupTestDB(t)

	a1 := &Audit{
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	a2 := &Audit{
		ResourceType:            ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "mas-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	a3 := &Audit{
		ResourceType:            ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	assert.NoError(t, CreateAuditEvent(db, a1))
	assert.NoError(t, CreateAuditEvent(db, a2))
	assert.NoError(t, CreateAuditEvent(db, a3))

	resp, err := ListAuditEvents(db, ResourceTypeMAS, AuditTypeKnowledgeQuery, 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, ResourceTypeMAS, resp.Data[0].ResourceType)
	assert.Equal(t, AuditTypeKnowledgeQuery, resp.Data[0].AuditType)
}

func TestListAuditEvents_WithPagination(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 5; i++ {
		a := &Audit{
			ResourceType:            ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		assert.NoError(t, CreateAuditEvent(db, a))
	}

	// page 0, pageSize 2
	resp, err := ListAuditEvents(db, "", "", 0, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, 2, resp.PageInfo.PageSize)
	assert.Equal(t, 2, resp.PageInfo.PageCount)
	assert.Equal(t, 5, resp.PageInfo.TotalElements)

	// page 1, pageSize 2
	resp, err = ListAuditEvents(db, "", "", 1, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 1, resp.PageInfo.Page)

	// last page — partial results (page 2 of pageSize 2 = items 4..4, so 1 item)
	resp, err = ListAuditEvents(db, "", "", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, 2, resp.PageInfo.Page)
	assert.Equal(t, 2, resp.PageInfo.PageSize)
	assert.Equal(t, 1, resp.PageInfo.PageCount)
	assert.Equal(t, 5, resp.PageInfo.TotalElements)

	// page past all results
	resp, err = ListAuditEvents(db, "", "", 10, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 0)
	assert.Equal(t, 0, resp.PageInfo.PageCount)
	assert.Equal(t, 5, resp.PageInfo.TotalElements)
}

func TestListAuditEvents_DefaultsAndClamping(t *testing.T) {
	db := setupTestDB(t)

	for i := 0; i < 3; i++ {
		a := &Audit{
			ResourceType:            ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		assert.NoError(t, CreateAuditEvent(db, a))
	}

	// pageSize=0 should default to DefaultPageSize()
	resp, err := ListAuditEvents(db, "", "", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, DefaultPageSize(), resp.PageInfo.PageSize)

	// pageSize=-5 should also default
	resp, err = ListAuditEvents(db, "", "", 0, -5)
	assert.NoError(t, err)
	assert.Equal(t, DefaultPageSize(), resp.PageInfo.PageSize)

	// pageSize > MaxPageSize should be clamped
	resp, err = ListAuditEvents(db, "", "", 0, 9999)
	assert.NoError(t, err)
	assert.Equal(t, MaxPageSize(), resp.PageInfo.PageSize)

	// negative page should be treated as 0
	resp, err = ListAuditEvents(db, "", "", -3, 2)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.PageInfo.Page)
}

func TestSetPaginationConfig(t *testing.T) {
	// Save originals and restore after test
	origDefault := DefaultPageSize()
	origMax := MaxPageSize()
	defer SetPaginationConfig(origDefault, origMax)

	// Override both
	SetPaginationConfig(10, 50)
	assert.Equal(t, 10, DefaultPageSize())
	assert.Equal(t, 50, MaxPageSize())

	// default > max => default clamped to max
	SetPaginationConfig(200, 30)
	assert.Equal(t, 30, DefaultPageSize())
	assert.Equal(t, 30, MaxPageSize())

	// zero/negative values ignored (keeps previous)
	SetPaginationConfig(15, 80)
	SetPaginationConfig(0, -1)
	assert.Equal(t, 15, DefaultPageSize())
	assert.Equal(t, 80, MaxPageSize())
}

func TestListAuditEvents_EmptyDB(t *testing.T) {
	db := setupTestDB(t)

	resp, err := ListAuditEvents(db, "", "", 0, 10)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Data)
	assert.Len(t, resp.Data, 0)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, 10, resp.PageInfo.PageSize)
	assert.Equal(t, 0, resp.PageInfo.PageCount)
	assert.Equal(t, 0, resp.PageInfo.TotalElements)
}

func TestDeleteAuditEventByID(t *testing.T) {
	db := setupTestDB(t)

	a := &Audit{
		ResourceType:            ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	assert.NoError(t, CreateAuditEvent(db, a))

	err := DeleteAuditEventByID(db, a.ID)
	assert.NoError(t, err)

	_, err = GetAuditEventByID(db, a.ID)
	assert.Error(t, err)
}

func TestDeleteAuditEventByID_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := DeleteAuditEventByID(db, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEnumConstants(t *testing.T) {
	assert.Equal(t, "COGNITION_ENGINE", ResourceTypeCognitionEngine)
	assert.Equal(t, "POLICY_ENFORCER", ResourceTypePolicyEnforcer)
	assert.Equal(t, "MEMORY_PROVIDER", ResourceTypeMemoryProvider)
	assert.Equal(t, "MAS", ResourceTypeMAS)
	assert.Equal(t, "MAS-AGENT", ResourceTypeMASAgent)
	assert.Equal(t, "WORKFLOW", ResourceTypeWorkflow)
	assert.Equal(t, "TASK", ResourceTypeTask)

	assert.Equal(t, "RESOURCE_CREATED", AuditTypeResourceCreated)
	assert.Equal(t, "RESOURCE_UPDATED", AuditTypeResourceUpdated)
	assert.Equal(t, "RESOURCE_DELETED", AuditTypeResourceDeleted)
	assert.Equal(t, "RESOURCE_PURGED", AuditTypeResourcePurged)
	assert.Equal(t, "RESOURCE_PRUNED", AuditTypeResourcePruned)
	assert.Equal(t, "KNOWLEDGE_INGESTION", AuditTypeKnowledgeIngestion)
	assert.Equal(t, "KNOWLEDGE_QUERY", AuditTypeKnowledgeQuery)
	assert.Equal(t, "MEMORY_OPERATION", AuditTypeMemoryOperation)
	assert.Equal(t, "SHARED_MEMORY_OPERATION", AuditTypeSharedMemoryOperation)
	assert.Equal(t, "AGENT_MEMORY_OPERATION", AuditTypeAgentMemoryOperation)
}
