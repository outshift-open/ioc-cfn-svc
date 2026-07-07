// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/outshift-open/ioc-cfn-svc/pkg/audit"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
)

// getAuditEventHandler retrieves a single audit event by ID.
// Internal API - not exposed in public Swagger documentation.
// Searches both the standard audit table and L9 audit table.
func (a *App) getAuditEventHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	idStr := eh.PathParam(r, "eventId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid event ID: must be a valid UUID",
		})
	}

	// Try standard audit table first
	event, err := a.db.GetAuditEventByID(id)
	if err == nil {
		return eh.RespondWithJSON(w, http.StatusOK, event)
	}

	// Try L9 audit table and convert to Audit format
	l9Event, err := a.db.GetL9AuditEventByID(id)
	if err == nil {
		auditEvent := l9Event.ToAudit()
		return eh.RespondWithJSON(w, http.StatusOK, &auditEvent)
	}

	return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
		"error": "audit event not found",
	})
}

// listAuditEventsHandler lists audit events with optional filters.
// Internal API - not exposed in public Swagger documentation.
// L9 protocol events are stored in a dedicated table but merged into this API's response.
// When no filters are specified, merges results from both tables sorted by created_on.
func (a *App) listAuditEventsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	resourceType := r.URL.Query().Get("resource_type")
	auditType := r.URL.Query().Get("audit_type")

	page, pageSize, err := parsePagination(r)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// If no audit_type filter, merge results from both tables
	if auditType == "" {
		return a.listMergedAuditEvents(w, resourceType, page, pageSize)
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

// listMergedAuditEvents fetches from both audit tables and merges results sorted by created_on DESC.
// This provides a unified view when no audit_type filter is specified.
func (a *App) listMergedAuditEvents(w http.ResponseWriter, resourceType string, page, pageSize int) (int, error) {
	// Fetch from standard audit table
	standardResult, err := a.db.ListAuditEvents(resourceType, "", page, pageSize)
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

	// Fetch from L9 audit table (no kind filter = all L9 events)
	l9Result, err := a.db.ListL9AuditEvents("", "", page, pageSize)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list L9 audit events",
		})
	}

	// Convert L9 events to Audit format
	l9Audits := make([]audit.Audit, 0, len(l9Result.Data))
	for _, l9Event := range l9Result.Data {
		l9Audits = append(l9Audits, l9Event.ToAudit())
	}

	// Merge and sort by CreatedOn DESC
	merged := append(standardResult.Data, l9Audits...)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedOn.After(merged[j].CreatedOn)
	})

	// Apply pagination to merged results
	totalElements := standardResult.PageInfo.TotalElements + l9Result.PageInfo.TotalElements
	start := page * pageSize
	end := start + pageSize
	if start > len(merged) {
		start = len(merged)
	}
	if end > len(merged) {
		end = len(merged)
	}
	paged := merged[start:end]

	result := &audit.AuditListResponse{
		Data: paged,
		PageInfo: audit.PageInfo{
			Page:          page,
			PageSize:      pageSize,
			PageCount:     len(paged),
			TotalElements: totalElements,
		},
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
