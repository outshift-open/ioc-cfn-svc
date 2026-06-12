// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"net/http"
	"strconv"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// getOtelSpansHandler godoc
// @Summary		Get OTel spans by trace ID
// @Description	Returns raw OTel spans from TimescaleDB for a given trace_id. Used by cognition-engine for KG ingestion.
// @Tags			otel-spans
// @Produce		json
// @Param		workspaceId	path	string	true	"Workspace ID"
// @Param		masId		path	string	true	"MAS ID"
// @Param		trace_id	query	string	true	"Trace ID to fetch spans for"
// @Success		200		{array}		otelreceiver.SpanRecord
// @Failure		400		{object}	map[string]string
// @Failure		500		{object}	map[string]string
// @Router		/api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/otel-spans [get]
func (a *App) getOtelSpansHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	traceID := r.URL.Query().Get("trace_id")
	if traceID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "trace_id query parameter is required",
		})
	}

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	spans, err := a.db.GetOtelSpansForTrace(workspaceID, masID, traceID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to query spans: " + err.Error(),
		})
	}

	result := make([]otelreceiver.SpanRecord, 0, len(spans))
	for _, s := range spans {
		result = append(result, otelSpanToRecord(s))
	}

	return eh.RespondWithJSON(w, http.StatusOK, result)
}

// getPendingOtelSpansHandler returns trace IDs pending KG ingestion.
// GET /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/otel-spans/pending?limit=10
func (a *App) getPendingOtelSpansHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	traceIDs, err := a.db.GetPendingOtelTraces(workspaceID, masID, limit, a.Cfg.TraceCompletion.InactivityTimeout)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to query pending traces: " + err.Error(),
		})
	}

	if traceIDs == nil {
		traceIDs = []string{}
	}
	return eh.RespondWithJSON(w, http.StatusOK, traceIDs)
}
