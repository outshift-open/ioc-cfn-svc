// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/outshift-open/ioc-cfn-svc/pkg/common"
	"github.com/outshift-open/ioc-cfn-svc/pkg/otelreceiver"
)

const defaultReadyOtelTraceLimit = 10

// OtelTaskPayload is the payload cfn-svc pushes to CE for OTel KG ingestion.
type OtelTaskPayload struct {
	Format     string                 `json:"format"`
	TraceCount int                    `json:"trace_count"`
	SpanCount  int                    `json:"span_count"`
	Traces     []OtelTraceTaskPayload `json:"traces"`
}

// OtelTraceTaskPayload groups all persisted spans for one trace.
type OtelTraceTaskPayload struct {
	TraceID   string                    `json:"trace_id"`
	SpanCount int                       `json:"span_count"`
	Spans     []otelreceiver.SpanRecord `json:"spans"`
}

// BuildReadyOtelTaskPayload claims ready traces that passed the inactivity delay
// and builds the payload that the task framework can attach to the CE trigger call.
func (a *App) BuildReadyOtelTaskPayload(workspaceID, masID string, limit int) (*OtelTaskPayload, error) {
	if limit <= 0 {
		limit = defaultReadyOtelTraceLimit
	}

	traceIDs, err := a.db.ClaimReadyOtelTraces(
		workspaceID,
		masID,
		limit,
		a.Cfg.TraceCompletion.InactivityTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("claim ready otel traces: %w", err)
	}

	payload := &OtelTaskPayload{
		Format: common.FormatOTelTrace,
		Traces: make([]OtelTraceTaskPayload, 0, len(traceIDs)),
	}

	for _, traceID := range traceIDs {
		spans, err := a.db.GetOtelSpansForTrace(workspaceID, masID, traceID)
		if err != nil {
			return nil, fmt.Errorf("get otel spans for trace %s: %w", traceID, err)
		}

		tracePayload := OtelTraceTaskPayload{
			TraceID:   traceID,
			SpanCount: len(spans),
			Spans:     make([]otelreceiver.SpanRecord, 0, len(spans)),
		}
		for _, span := range spans {
			tracePayload.Spans = append(tracePayload.Spans, otelSpanToRecord(span))
		}

		payload.TraceCount++
		payload.SpanCount += tracePayload.SpanCount
		payload.Traces = append(payload.Traces, tracePayload)
	}

	return payload, nil
}
