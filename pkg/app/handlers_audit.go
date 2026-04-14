package app

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
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

