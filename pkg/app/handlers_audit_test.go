package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
)

func newTestApp() *App {
	return &App{
		db: client.NewMockDatabase(),
	}
}

func TestCreateAuditEventHandler(t *testing.T) {
	app := newTestApp()

	body := audit.CreateAuditEventRequest{
		ResourceType:            audit.ResourceTypeCognitiveEngine,
		ResourceIdentifier:      "ce-123",
		AuditType:               audit.AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-123",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, err := app.createAuditEventHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "entry created", resp["message"])
}

func TestCreateAuditEventHandler_InvalidJSON(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, _ := app.createAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestCreateAuditEventHandler_MissingRequiredFields(t *testing.T) {
	app := newTestApp()

	body := audit.CreateAuditEventRequest{
		ResourceType: audit.ResourceTypeCognitiveEngine,
		// missing resource_identifier, audit_type, audit_resource_identifier
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, _ := app.createAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestGetAuditEventHandler(t *testing.T) {
	app := newTestApp()

	// Create an event first
	event := &audit.Audit{
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               audit.AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	require.NoError(t, app.db.CreateAuditEvent(event))

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events/"+event.ID.String(), nil)
	req.SetPathValue("eventId", event.ID.String())
	rr := httptest.NewRecorder()

	code, err := app.getAuditEventHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.Audit
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, event.ID, resp.ID)
}

func TestGetAuditEventHandler_InvalidUUID(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events/not-a-uuid", nil)
	req.SetPathValue("eventId", "not-a-uuid")
	rr := httptest.NewRecorder()

	code, _ := app.getAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestGetAuditEventHandler_NotFound(t *testing.T) {
	app := newTestApp()

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events/"+id.String(), nil)
	req.SetPathValue("eventId", id.String())
	rr := httptest.NewRecorder()

	code, _ := app.getAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestListAuditEventsHandler(t *testing.T) {
	app := newTestApp()

	for i := 0; i < 3; i++ {
		e := &audit.Audit{
			ResourceType:            audit.ResourceTypeCognitiveEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               audit.AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		require.NoError(t, app.db.CreateAuditEvent(e))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp []audit.Audit
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 3)
}

func TestListAuditEventsHandler_WithFilters(t *testing.T) {
	app := newTestApp()

	e1 := &audit.Audit{
		ResourceType:            audit.ResourceTypeCognitiveEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               audit.AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	e2 := &audit.Audit{
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               audit.AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	require.NoError(t, app.db.CreateAuditEvent(e1))
	require.NoError(t, app.db.CreateAuditEvent(e2))

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events?resource_type=MAS", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp []audit.Audit
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, audit.ResourceTypeMAS, resp[0].ResourceType)
}

func TestDeleteAuditEventHandler(t *testing.T) {
	app := newTestApp()

	event := &audit.Audit{
		ResourceType:            audit.ResourceTypeCognitiveEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               audit.AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	require.NoError(t, app.db.CreateAuditEvent(event))

	req := httptest.NewRequest(http.MethodDelete, "/api/internal/audit-events/"+event.ID.String(), nil)
	req.SetPathValue("eventId", event.ID.String())
	rr := httptest.NewRecorder()

	code, err := app.deleteAuditEventHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, code)

	// Verify it's gone
	req2 := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events/"+event.ID.String(), nil)
	req2.SetPathValue("eventId", event.ID.String())
	rr2 := httptest.NewRecorder()

	code2, _ := app.getAuditEventHandler(rr2, req2)
	assert.Equal(t, http.StatusNotFound, code2)
}

func TestDeleteAuditEventHandler_InvalidUUID(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodDelete, "/api/internal/audit-events/bad-id", nil)
	req.SetPathValue("eventId", "bad-id")
	rr := httptest.NewRecorder()

	code, _ := app.deleteAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestDeleteAuditEventHandler_NotFound(t *testing.T) {
	app := newTestApp()

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/internal/audit-events/"+id.String(), nil)
	req.SetPathValue("eventId", id.String())
	rr := httptest.NewRecorder()

	code, _ := app.deleteAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestCreateAuditEventHandler_InvalidResourceType(t *testing.T) {
	app := newTestApp()

	body := audit.CreateAuditEventRequest{
		ResourceType:            "INVALID_TYPE",
		ResourceIdentifier:      "ce-123",
		AuditType:               audit.AuditTypeResourceCreated,
		AuditResourceIdentifier: "ce-123",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, _ := app.createAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid resource_type")
}

func TestCreateAuditEventHandler_InvalidAuditType(t *testing.T) {
	app := newTestApp()

	body := audit.CreateAuditEventRequest{
		ResourceType:            audit.ResourceTypeCognitiveEngine,
		ResourceIdentifier:      "ce-123",
		AuditType:               "INVALID_AUDIT",
		AuditResourceIdentifier: "ce-123",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, _ := app.createAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid audit_type")
}

func TestListAuditEventsHandler_InvalidResourceTypeFilter(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events?resource_type=BOGUS", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid resource_type")
}

func TestListAuditEventsHandler_InvalidAuditTypeFilter(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/audit-events?audit_type=BOGUS", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid audit_type")
}

func TestCreateAuditEventHandler_WithOptionalFields(t *testing.T) {
	app := newTestApp()

	opID := "op-456"
	extra := "some extra info"
	body := audit.CreateAuditEventRequest{
		OperationID:             &opID,
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      "mas-1",
		AuditType:               audit.AuditTypeKnowledgeQuery,
		AuditResourceIdentifier: "agent-1",
		AuditInformation:        datatypes.JSON(`{"query":"test"}`),
		AuditExtraInformation:   &extra,
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/audit-events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, err := app.createAuditEventHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "entry created", resp["message"])
}
