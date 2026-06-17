// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"time"

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

// TraceTracker records ingestion state for trace discovery and completion detection.
type TraceTracker interface {
	UpsertPendingOtelTrace(workspaceID, masID, traceID string, lastSpanTime time.Time) error
}

// SpanFilter returns true if the span record should be kept for storage.
type SpanFilter func(SpanRecord) bool

// DropNoise drops internal/noise spans (session.stuck, heartbeat).
func DropNoise(r SpanRecord) bool {
	if r.Name == "openclaw.session.stuck" {
		return false
	}
	if ch, ok := r.Attributes["openclaw.message.channel"].(string); ok && ch == "heartbeat" {
		return false
	}
	return true
}

// IngressValidator adapts a func(workspaceID, masID string) bool into a SpanFilter.
func IngressValidator(check func(workspaceID, masID string) bool) SpanFilter {
	return func(r SpanRecord) bool {
		if !check(r.WorkspaceID, r.MasID) {
			exporterLog.Warnf("otelwriter: dropping span %s (%s) — failed ingress validation", r.SpanID, r.Name)
			return false
		}
		return true
	}
}

// SpanExporter implements consumer.Traces and component.Component.
// It maps ptrace.Traces to OtelSpans and bulk-inserts them via SpanStore.
// Batching is handled upstream by the batchprocessor.
type SpanExporter struct {
	store    SpanStore    // persists validated spans to the otel_spans table
	tracker  TraceTracker // records ingestion state for trace completion detection
	resolver AgentResolver
	filters  []SpanFilter
}

// NewSpanExporter creates a SpanExporter that writes to store.
func NewSpanExporter(store SpanStore, tracker TraceTracker, resolver AgentResolver, filters ...SpanFilter) *SpanExporter {
	var ff []SpanFilter
	for _, f := range filters {
		if f != nil {
			ff = append(ff, f)
		}
	}
	return &SpanExporter{store: store, tracker: tracker, resolver: resolver, filters: ff}
}

// Capabilities returns consumer capabilities (read-only, does not mutate spans).
func (e *SpanExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is a no-op required by component.Component.
func (e *SpanExporter) Start(_ context.Context, _ component.Host) error { return nil }

// Shutdown is a no-op required by component.Component.
func (e *SpanExporter) Shutdown(_ context.Context) error { return nil }

// accept returns true if the span passes all registered filters.
func (e *SpanExporter) accept(r SpanRecord) bool {
	for _, f := range e.filters {
		if !f(r) {
			return false
		}
	}
	return true
}

// ConsumeTraces maps spans, applies filters, and bulk-inserts the rest.
func (e *SpanExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	records := MapSpans(td, e.resolver)

	var otelSpans []OtelSpan
	for _, r := range records {
		if !e.accept(r) {
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

	// Track distinct traces for pending KG ingestion and update last_span_time for completion detection.
	if e.tracker != nil {
		type traceKey struct {
			ws, mas, trace string
			lastSpanTime   time.Time
		}
		traces := make(map[string]traceKey) // key: ws|mas|trace
		for _, s := range otelSpans {
			key := s.WorkspaceID.String() + "|" + s.MasID.String() + "|" + s.TraceID
			existing, ok := traces[key]
			if !ok || s.StartTime.After(existing.lastSpanTime) {
				traces[key] = traceKey{
					ws:           s.WorkspaceID.String(),
					mas:          s.MasID.String(),
					trace:        s.TraceID,
					lastSpanTime: s.StartTime,
				}
			}
		}
		for _, k := range traces {
			if err := e.tracker.UpsertPendingOtelTrace(k.ws, k.mas, k.trace, k.lastSpanTime); err != nil {
				exporterLog.Warnf("otelwriter: failed to track pending trace %s: %v", k.trace, err)
			}
		}
	}

	return nil
}
