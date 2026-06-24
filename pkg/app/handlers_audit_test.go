// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outshift-open/ioc-cfn-svc/pkg/audit"
	"github.com/outshift-open/ioc-cfn-svc/pkg/client"
)

func newTestApp() *App {
	return &App{
		db: client.NewMockDatabase(),
	}
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

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit/"+event.ID.String(), nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit/not-a-uuid", nil)
	req.SetPathValue("eventId", "not-a-uuid")
	rr := httptest.NewRecorder()

	code, _ := app.getAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestGetAuditEventHandler_NotFound(t *testing.T) {
	app := newTestApp()

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit/"+id.String(), nil)
	req.SetPathValue("eventId", id.String())
	rr := httptest.NewRecorder()

	code, _ := app.getAuditEventHandler(rr, req)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestListAuditEventsHandler(t *testing.T) {
	app := newTestApp()

	for i := 0; i < 3; i++ {
		e := &audit.Audit{
			ResourceType:            audit.ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               audit.AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		require.NoError(t, app.db.CreateAuditEvent(e))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.AuditListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp.Data, 3)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, audit.DefaultPageSize(), resp.PageInfo.PageSize)
	assert.Equal(t, 3, resp.PageInfo.PageCount)
	assert.Equal(t, 3, resp.PageInfo.TotalElements)
}

func TestListAuditEventsHandler_WithFilters(t *testing.T) {
	app := newTestApp()

	e1 := &audit.Audit{
		ResourceType:            audit.ResourceTypeCognitionEngine,
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

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?resource_type=MAS", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.AuditListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, audit.ResourceTypeMAS, resp.Data[0].ResourceType)
}

func TestListAuditEventsHandler_WithPagination(t *testing.T) {
	app := newTestApp()

	for i := 0; i < 5; i++ {
		e := &audit.Audit{
			ResourceType:            audit.ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               audit.AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		require.NoError(t, app.db.CreateAuditEvent(e))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?page=0&pageSize=2", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.AuditListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, 2, resp.PageInfo.PageSize)
	assert.Equal(t, 2, resp.PageInfo.PageCount)
	assert.Equal(t, 5, resp.PageInfo.TotalElements)
}

func TestListAuditEventsHandler_WithFiltersAndPagination(t *testing.T) {
	app := newTestApp()

	for i := 0; i < 3; i++ {
		e := &audit.Audit{
			ResourceType:            audit.ResourceTypeMAS,
			ResourceIdentifier:      "mas-1",
			AuditType:               audit.AuditTypeResourceCreated,
			AuditResourceIdentifier: "mas-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		require.NoError(t, app.db.CreateAuditEvent(e))
	}
	e := &audit.Audit{
		ResourceType:            audit.ResourceTypeCognitionEngine,
		ResourceIdentifier:      "ce-1",
		AuditType:               audit.AuditTypeResourceDeleted,
		AuditResourceIdentifier: "ce-1",
		CreatedBy:               uuid.New(),
		LastModifiedBy:          uuid.New(),
	}
	require.NoError(t, app.db.CreateAuditEvent(e))

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?resource_type=MAS&audit_type=RESOURCE_CREATED&page=0&pageSize=2", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.AuditListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 3, resp.PageInfo.TotalElements)
	for _, r := range resp.Data {
		assert.Equal(t, audit.ResourceTypeMAS, r.ResourceType)
		assert.Equal(t, audit.AuditTypeResourceCreated, r.AuditType)
	}
}

func TestListAuditEventsHandler_PageSizeExceedsMax(t *testing.T) {
	app := newTestApp()

	for i := 0; i < 3; i++ {
		e := &audit.Audit{
			ResourceType:            audit.ResourceTypeCognitionEngine,
			ResourceIdentifier:      "ce-1",
			AuditType:               audit.AuditTypeResourceCreated,
			AuditResourceIdentifier: "ce-1",
			CreatedBy:               uuid.New(),
			LastModifiedBy:          uuid.New(),
		}
		require.NoError(t, app.db.CreateAuditEvent(e))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?pageSize=99999", nil)
	rr := httptest.NewRecorder()

	code, err := app.listAuditEventsHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp audit.AuditListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, audit.MaxPageSize(), resp.PageInfo.PageSize)
}

func TestListAuditEventsHandler_InvalidPage(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?page=abc", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid page parameter")
}

func TestListAuditEventsHandler_InvalidPageSize(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?pageSize=-1", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid pageSize parameter")
}

func TestListAuditEventsHandler_InvalidResourceTypeFilter(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?resource_type=BOGUS", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid resource_type")
}

func TestListAuditEventsHandler_InvalidAuditTypeFilter(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/internal/mgmt/audit?audit_type=BOGUS", nil)
	rr := httptest.NewRecorder()

	code, _ := app.listAuditEventsHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid audit_type")
}

