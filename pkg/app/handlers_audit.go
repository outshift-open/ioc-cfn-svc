package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// createAuditEventHandler handles POST /api/internal/audit-events.
// Creates a new audit event after validating required fields and enum values.
func (a *App) createAuditEventHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var req audit.CreateAuditEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	if req.ResourceType == "" || req.ResourceIdentifier == "" ||
		req.AuditType == "" || req.AuditResourceIdentifier == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "resource_type, resource_identifier, audit_type, and audit_resource_identifier are required",
		})
	}

	event := &audit.Audit{
		OperationID:             req.OperationID,
		ResourceType:            req.ResourceType,
		ResourceIdentifier:      req.ResourceIdentifier,
		AuditType:               req.AuditType,
		AuditResourceIdentifier: req.AuditResourceIdentifier,
		AuditInformation:        req.AuditInformation,
		AuditExtraInformation:   req.AuditExtraInformation,
		CreatedBy:               req.CreatedBy,
		LastModifiedBy:          req.LastModifiedBy,
	}

	if err := a.db.CreateAuditEvent(event); err != nil {
		if strings.Contains(err.Error(), "invalid") {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create audit event",
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "entry created",
	})
}

// getAuditEventHandler handles GET /api/internal/audit-events/{eventId}.
// Returns a single audit event by its UUID.
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

// listAuditEventsHandler handles GET /api/internal/audit-events.
// Returns audit events with optional resource_type and audit_type query filters.
func (a *App) listAuditEventsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	resourceType := r.URL.Query().Get("resource_type")
	auditType := r.URL.Query().Get("audit_type")

	events, err := a.db.ListAuditEvents(resourceType, auditType)
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

	return eh.RespondWithJSON(w, http.StatusOK, events)
}

// deleteAuditEventHandler handles DELETE /api/internal/audit-events/{eventId}.
// Deletes a single audit event by its UUID. Internal API only.
func (a *App) deleteAuditEventHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	idStr := eh.PathParam(r, "eventId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid event ID: must be a valid UUID",
		})
	}

	if err := a.db.DeleteAuditEventByID(id); err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": "audit event not found",
		})
	}

	return http.StatusNoContent, nil
}
