// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/outshift-open/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/outshift-open/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/outshift-open/ioc-cfn-svc/pkg/common"
	"github.com/outshift-open/ioc-cfn-svc/pkg/model"
)

// sendOtelTaskExecution builds a shared-memory request from a pre-built OTel payload
// and delegates extraction + persistence to createOrUpdateSharedMemoriesCore.
//
// Precondition: payload is non-nil and contains at least one trace with at least one
// span; the caller (dispatchTask) is responsible for that gate.
func (a *App) sendOtelTaskExecution(t model.Task, historyID string, payload *OtelTaskPayload) {
	log := getLogger()

	result := map[string]interface{}{
		"format":      payload.Format,
		"trace_count": payload.TraceCount,
		"span_count":  payload.SpanCount,
		"trace_ids":   otelTaskTraceIDs(payload),
	}

	payloadData, err := json.Marshal(payload.Traces)
	if err != nil {
		log.Errorf("failed to marshal OTel extraction payload | workspace=%s mas=%s task=%s: %s", t.WorkspaceID, t.MASID, t.ID, err)
		a.updateOtelTraceStatuses(t.WorkspaceID, t.MASID, payload.Traces, "failed")
		a.completeTaskExecution(t, historyID, "failed", result, err)
		return
	}

	req := sharedmemory.CreateOrUpdateRequest{
		RequestId: &historyID,
		Payload: cognitionagentclient.ExtractionPayload{
			Metadata: cognitionagentclient.ExtractionPayloadMetadata{
				Format: common.FormatOTelTrace,
			},
			Data: json.RawMessage(payloadData),
		},
	}

	resp, err := a.createOrUpdateSharedMemoriesCore(context.Background(), t.WorkspaceID, t.MASID, req)
	if err != nil {
		log.Errorf("OTel extraction failed | workspace=%s mas=%s task=%s: %s", t.WorkspaceID, t.MASID, t.ID, err)
		a.updateOtelTraceStatuses(t.WorkspaceID, t.MASID, payload.Traces, "failed")
		a.completeTaskExecution(t, historyID, "failed", result, err)
		return
	}

	if resp != nil {
		result["graph_status"] = resp.Status
		result["graph_store_message"] = resp.GraphStoreMessage
		result["vector_store_message"] = resp.VectorStoreMessage
		if resp.ResponseID != nil {
			result["extraction_response_id"] = *resp.ResponseID
		}
	}

	a.updateOtelTraceStatuses(t.WorkspaceID, t.MASID, payload.Traces, "completed")
	a.completeTaskExecution(t, historyID, "success", result, nil)
}

// otelTaskTraceIDs extracts a list of trace IDs from an OTel task payload for logging/result tracking.
func otelTaskTraceIDs(payload *OtelTaskPayload) []string {
	if payload == nil || len(payload.Traces) == 0 {
		return nil
	}

	traceIDs := make([]string, 0, len(payload.Traces))
	for _, trace := range payload.Traces {
		traceIDs = append(traceIDs, trace.TraceID)
	}
	return traceIDs
}

// updateOtelTraceStatuses marks each trace in the payload with the given ingestion status
// (e.g., "completed", "failed"). Used after KG extraction succeeds or fails.
func (a *App) updateOtelTraceStatuses(workspaceID, masID string, traces []OtelTraceTaskPayload, status string) {
	log := getLogger()
	for _, trace := range traces {
		if trace.TraceID == "" {
			continue
		}
		if err := a.db.UpdateOtelTraceStatus(workspaceID, masID, trace.TraceID, status); err != nil {
			log.Errorf("failed to update OTel trace state | workspace=%s mas=%s trace=%s status=%s err=%s", workspaceID, masID, trace.TraceID, status, err)
		}
	}
}
