// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"encoding/hex"
	"strings"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// SpanRecord mirrors the Pydantic SpanRecord expected by memory-svc POST /api/otel-spans/batch.
type SpanRecord struct {
	TraceID       string         `json:"trace_id"`
	SpanID        string         `json:"span_id"`
	ParentSpanID  string         `json:"parent_span_id,omitempty"`
	WorkspaceID   string         `json:"workspace_id"`
	MasID         string         `json:"mas_id"`
	AgentID       string         `json:"agent_id,omitempty"`
	OperationName string         `json:"operation_name"`
	ServiceName   string         `json:"service_name"`
	SpanKind      string         `json:"span_kind,omitempty"`
	DurationUs    int64          `json:"duration_us"`
	StatusCode    string         `json:"status_code,omitempty"`
	StartTime     string         `json:"start_time"` // RFC3339Nano
	Attributes    map[string]any `json:"attributes"`
	Events        any            `json:"events,omitempty"`
	Links         any            `json:"links,omitempty"`
	Resource      map[string]any `json:"resource,omitempty"`
}

// AgentResolver resolves an agent_id UUID from an agent session key
// (e.g. "main::agents::planner") by matching Identity.Identifiers["url"] in CFN config.
type AgentResolver func(sessionKey string) string

// MapSpans converts a slice of OTLP ResourceSpans into a flat slice of SpanRecords.
//
// workspace_id / mas_id lookup order (first non-empty wins):
//  1. Resource attributes — dot notation keys ("workspace.id", "mas.id") set via
//     the plugin's resourceAttributes config. Present on every ResourceSpans block.
//  2. Span attributes — hyphen notation keys ("workspace-id", "mas-id") set via
//     the plugin's customAttributes config. Only present on the root span.
//
// agent_id lookup order per span (first non-empty wins):
//  1. Span attribute "agent.id" — direct UUID, set by any sender that already knows it.
//  2. openclaw.session.key resolver — looks up "main::agents::<name>" against
//     AgentCfg.Identity.Identifiers["url"] in ParsedConfig and returns its UUID.
func MapSpans(resourceSpans []*tracepb.ResourceSpans, resolver AgentResolver) []SpanRecord {
	if len(resourceSpans) == 0 {
		return nil
	}

	workspaceID, masID := extractWorkspaceMAS(resourceSpans)

	var out []SpanRecord
	for _, rs := range resourceSpans {
		resourceAttrs := kvListToMap(rs.GetResource().GetAttributes())
		serviceName := stringFromMap(resourceAttrs, "service.name")

		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				record := mapSpan(span, serviceName, workspaceID, masID, resourceAttrs, resolver)
				out = append(out, record)
			}
		}
	}
	return out
}

// extractWorkspaceMAS returns the workspace and MAS IDs by checking resource attributes
// first (resourceAttributes path, dot notation), then span attributes (customAttributes
// path, hyphen notation on the root span only).
func extractWorkspaceMAS(resourceSpans []*tracepb.ResourceSpans) (workspaceID, masID string) {
	for _, rs := range resourceSpans {
		// resourceAttributes path: set on every ResourceSpans block.
		// Keys use dot notation as emitted by the plugin's resourceAttributes config.
		resourceAttrs := kvListToMap(rs.GetResource().GetAttributes())
		if wid := stringFromMap(resourceAttrs, "workspace.id"); wid != "" {
			workspaceID = wid
		}
		if mid := stringFromMap(resourceAttrs, "mas.id"); mid != "" {
			masID = mid
		}
		if workspaceID != "" && masID != "" {
			return
		}

		// customAttributes path: only the root span has these (hyphen notation).
		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				attrs := kvListToMap(span.GetAttributes())
				if wid := stringFromMap(attrs, "workspace-id"); wid != "" && workspaceID == "" {
					workspaceID = wid
				}
				if mid := stringFromMap(attrs, "mas-id"); mid != "" && masID == "" {
					masID = mid
				}
				if workspaceID != "" && masID != "" {
					return
				}
			}
		}
	}
	return
}

func mapSpan(
	span *tracepb.Span,
	serviceName, workspaceID, masID string,
	resourceAttrs map[string]any,
	resolver AgentResolver,
) SpanRecord {
	attrs := kvListToMap(span.GetAttributes())

	// agent_id priority: direct attribute → session key resolver → empty
	agentID := stringFromMap(attrs, "agent.id")
	if agentID == "" && resolver != nil {
		if sessionKey := stringFromMap(attrs, "openclaw.session.key"); sessionKey != "" {
			agentID = resolver(sessionKey)
		}
	}

	startNs := span.GetStartTimeUnixNano()
	endNs := span.GetEndTimeUnixNano()
	durationUs := int64(0)
	if endNs > startNs {
		durationUs = int64((endNs - startNs) / 1000)
	}

	startTime := time.Unix(0, int64(startNs)).UTC().Format(time.RFC3339Nano)

	record := SpanRecord{
		TraceID:       hex.EncodeToString(span.GetTraceId()),
		SpanID:        hex.EncodeToString(span.GetSpanId()),
		WorkspaceID:   workspaceID,
		MasID:         masID,
		AgentID:       agentID,
		OperationName: span.GetName(),
		ServiceName:   serviceName,
		SpanKind:      spanKindString(span.GetKind()),
		DurationUs:    durationUs,
		StartTime:     startTime,
		Attributes:    attrs,
	}

	if parentID := hex.EncodeToString(span.GetParentSpanId()); parentID != "" && !isZeroID(parentID) {
		record.ParentSpanID = parentID
	}

	if status := span.GetStatus(); status != nil {
		record.StatusCode = statusCodeString(status.GetCode())
	}

	if events := span.GetEvents(); len(events) > 0 {
		record.Events = mapEvents(events)
	}

	if links := span.GetLinks(); len(links) > 0 {
		record.Links = mapLinks(links)
	}

	if len(resourceAttrs) > 0 {
		record.Resource = resourceAttrs
	}

	return record
}

// isZeroID returns true if the hex-encoded ID is all zeros (absent parent span).
func isZeroID(hexID string) bool {
	return strings.TrimLeft(hexID, "0") == ""
}

func kvListToMap(kvs []*commonpb.KeyValue) map[string]any {
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = anyValueToGo(kv.GetValue())
	}
	return m
}

func anyValueToGo(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return val.BoolValue
	case *commonpb.AnyValue_IntValue:
		return val.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return val.DoubleValue
	case *commonpb.AnyValue_ArrayValue:
		if val.ArrayValue == nil {
			return nil
		}
		arr := make([]any, len(val.ArrayValue.GetValues()))
		for i, item := range val.ArrayValue.GetValues() {
			arr[i] = anyValueToGo(item)
		}
		return arr
	case *commonpb.AnyValue_KvlistValue:
		if val.KvlistValue == nil {
			return nil
		}
		return kvListToMap(val.KvlistValue.GetValues())
	default:
		return nil
	}
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func spanKindString(k tracepb.Span_SpanKind) string {
	switch k {
	case tracepb.Span_SPAN_KIND_INTERNAL:
		return "INTERNAL"
	case tracepb.Span_SPAN_KIND_SERVER:
		return "SERVER"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "CLIENT"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "PRODUCER"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "CONSUMER"
	default:
		return ""
	}
}

func statusCodeString(c tracepb.Status_StatusCode) string {
	switch c {
	case tracepb.Status_STATUS_CODE_OK:
		return "OK"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "ERROR"
	default:
		return "UNSET"
	}
}

func mapEvents(events []*tracepb.Span_Event) []map[string]any {
	out := make([]map[string]any, len(events))
	for i, e := range events {
		out[i] = map[string]any{
			"name":       e.GetName(),
			"timestamp":  time.Unix(0, int64(e.GetTimeUnixNano())).UTC().Format(time.RFC3339Nano),
			"attributes": kvListToMap(e.GetAttributes()),
		}
	}
	return out
}

func mapLinks(links []*tracepb.Span_Link) []map[string]any {
	out := make([]map[string]any, len(links))
	for i, l := range links {
		out[i] = map[string]any{
			"trace_id":   hex.EncodeToString(l.GetTraceId()),
			"span_id":    hex.EncodeToString(l.GetSpanId()),
			"attributes": kvListToMap(l.GetAttributes()),
		}
	}
	return out
}
