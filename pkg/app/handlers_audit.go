// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/outshift-open/ioc-cfn-svc/pkg/audit"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
)

// getAuditEventHandler retrieves a single audit event by ID.
// Internal API - not exposed in public Swagger documentation.
func (a *App) getAuditEventHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	idStr := eh.PathParam(r, "eventId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid event ID: must be a valid UUID",
		})
	}

	event, err := a.db.GetAuditEventByID(id)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": "audit event not found",
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, event)
}

// listAuditEventsHandler lists audit events with optional filters.
// Internal API - not exposed in public Swagger documentation.
func (a *App) listAuditEventsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	resourceType := r.URL.Query().Get("resource_type")
	auditType := r.URL.Query().Get("audit_type")

	page, pageSize, err := parsePagination(r)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	result, err := a.db.ListAuditEvents(resourceType, auditType, page, pageSize)
	if err != nil {
		if strings.Contains(err.Error(), "invalid") {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list audit events",
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, result)
}

// parsePagination extracts page and pageSize query parameters from the request.
// page is 0-based (default 0). pageSize defaults to DefaultPageSize and is capped at MaxPageSize.
func parsePagination(r *http.Request) (int, int, error) {
	var err error
	page := 0
	pageSize := audit.DefaultPageSize()

	if p := r.URL.Query().Get("page"); p != "" {
		page, err = strconv.Atoi(p)
		if err != nil || page < 0 {
			return 0, 0, fmt.Errorf("invalid page parameter: must be a non-negative integer")
		}
	}
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		pageSize, err = strconv.Atoi(ps)
		if err != nil || pageSize < 1 {
			return 0, 0, fmt.Errorf("invalid pageSize parameter: must be a positive integer")
		}
		if pageSize > audit.MaxPageSize() {
			pageSize = audit.MaxPageSize()
		}
	}

	return page, pageSize, nil
}

// getL9AuditEventHandler retrieves a single L9 audit event by ID.
// @Summary Get L9 audit event by ID
// @Description Retrieves a single L9 protocol audit event by its UUID
// @Tags l9-audit
// @Produce json
// @Param eventId path string true "Event UUID"
// @Success 200 {object} audit.L9AuditEvent "L9 audit event"
// @Failure 400 {object} map[string]string "Invalid event ID"
// @Failure 404 {object} map[string]string "Event not found"
// @Router /api/internal/l9/audit/{eventId} [get]
func (a *App) getL9AuditEventHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	idStr := eh.PathParam(r, "eventId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid event ID: must be a valid UUID",
		})
	}

	event, err := a.db.GetL9AuditEventByID(id)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": "L9 audit event not found",
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, event)
}

// listL9AuditEventsHandler lists L9 audit events with optional filters.
// @Summary List L9 audit events
// @Description Lists L9 protocol audit events with optional kind and episode_id filters
// @Tags l9-audit
// @Produce json
// @Param kind query string false "Filter by L9 kind (intent, exchange, contingency, commit, knowledge)"
// @Param episode_id query string false "Filter by episode ID"
// @Param page query int false "Page number (0-based)" default(0)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} audit.L9AuditListResponse "Paginated list of L9 audit events"
// @Failure 400 {object} map[string]string "Invalid parameters"
// @Failure 500 {object} map[string]string "Internal error"
// @Router /api/internal/l9/audit [get]
func (a *App) listL9AuditEventsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	kind := r.URL.Query().Get("kind")
	episodeID := r.URL.Query().Get("episode_id")

	page, pageSize, err := parsePagination(r)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	result, err := a.db.ListL9AuditEvents(kind, episodeID, page, pageSize)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list L9 audit events",
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, result)
}
