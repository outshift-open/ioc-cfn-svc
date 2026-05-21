// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var exporterLog = logger.SubPkg("otelwriter")

// SpanStore is the sink interface for persisting OtelSpans.
type SpanStore interface {
	BulkInsertOtelSpans(spans []OtelSpan) error
}

// SpanExporter implements consumer.Traces and component.Component.
// It maps ptrace.Traces to OtelSpans and bulk-inserts them via SpanStore.
// Batching is handled upstream by the batchprocessor.
type SpanExporter struct {
	store    SpanStore
	resolver AgentResolver
}

// NewSpanExporter creates a SpanExporter that writes to store.
func NewSpanExporter(store SpanStore, resolver AgentResolver) *SpanExporter {
	return &SpanExporter{store: store, resolver: resolver}
}

// Capabilities returns consumer capabilities (read-only, does not mutate spans).
func (e *SpanExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is a no-op required by component.Component.
func (e *SpanExporter) Start(_ context.Context, _ component.Host) error { return nil }

// Shutdown is a no-op required by component.Component.
func (e *SpanExporter) Shutdown(_ context.Context) error { return nil }

// ConsumeTraces maps spans, drops those missing required fields, and bulk-inserts the rest.
func (e *SpanExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	records := MapSpans(td, e.resolver)

	var otelSpans []OtelSpan
	for _, r := range records {
		if r.WorkspaceID == "" || r.MasID == "" {
			exporterLog.Warnf("otelwriter: dropping span %s (%s) — missing workspace_id or mas_id", r.SpanID, r.OperationName)
			continue
		}
		if r.OperationName == "openclaw.session.stuck" {
			continue
		}
		if ch, ok := r.Attributes["openclaw.message.channel"].(string); ok && ch == "heartbeat" {
			continue
		}
		span, err := spanRecordToOtelSpan(r)
		if err != nil {
			exporterLog.Warnf("otelwriter: dropping span %s — %v", r.SpanID, err)
			continue
		}
		otelSpans = append(otelSpans, span)
	}

	if len(otelSpans) == 0 {
		return nil
	}

	if err := e.store.BulkInsertOtelSpans(otelSpans); err != nil {
		exporterLog.Errorf("otelwriter: bulk insert failed: %v", err)
		return err
	}

	exporterLog.Infof("otelwriter: inserted %d span(s)", len(otelSpans))
	return nil
}
