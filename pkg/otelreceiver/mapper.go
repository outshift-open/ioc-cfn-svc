// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"encoding/hex"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SpanRecord struct {
	TraceID       string           `json:"trace_id"`
	SpanID        string           `json:"span_id"`
	ParentSpanID  string           `json:"parent_span_id,omitempty"`
	WorkspaceID   string           `json:"workspace_id"`
	MasID         string           `json:"mas_id"`
	AgentID       string           `json:"agent_id,omitempty"`
	Name          string           `json:"name"`
	ServiceName   string           `json:"service_name"`
	Kind          int              `json:"kind,omitempty"`
	DurationNano  int64            `json:"duration_nano"`
	StatusCode    int              `json:"status_code,omitempty"`
	StatusMessage string           `json:"status_message,omitempty"`
	StartTime     string           `json:"start_time"` // RFC3339Nano
	Attributes    map[string]any   `json:"attributes"`
	Events        []map[string]any `json:"events,omitempty"`
	Links         []map[string]any `json:"links,omitempty"`
	Resource      map[string]any   `json:"resource,omitempty"`
}

// AgentResolver resolves an agent_id UUID from an agent session key by matching
// against Identity.Identifiers values in CFN config.
type AgentResolver func(sessionKey string) string

// MapSpans converts a ptrace.Traces into a flat slice of SpanRecords.
//
// workspace_id / mas_id lookup order (first non-empty wins):
//  1. Resource attributes — dot notation keys ("workspace.id", "mas.id") set via
//     the plugin's resourceAttributes config. Present on every ResourceSpans block.
//  2. Span attributes — hyphen notation keys ("workspace-id", "mas-id") set via
//     the plugin's customAttributes config. Only present on the root span.
//
// agent_id lookup order per span (first non-empty wins):
//  1. Span attribute "agent.id" — direct UUID, set by any sender that already knows it.
//  2. openclaw.session.key resolver — prefix-matched against all Identity.Identifiers
//     values in ParsedConfig and returns the matching agent's UUID.
func MapSpans(td ptrace.Traces, resolver AgentResolver) []SpanRecord {
	rss := td.ResourceSpans()
	if rss.Len() == 0 {
		return nil
	}

	var out []SpanRecord
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		workspaceID, masID := extractWorkspaceMASFromRS(rs)
		resourceAttrs := rs.Resource().Attributes().AsRaw()
		serviceName := ""
		if v, ok := rs.Resource().Attributes().Get("service.name"); ok {
			serviceName = v.Str()
		}

		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				out = append(out, mapSpan(spans.At(k), serviceName, workspaceID, masID, resourceAttrs, resolver))
			}
		}
	}
	return out
}

// extractWorkspaceMASFromRS returns workspace and MAS IDs for a single ResourceSpans block.
// It checks resource attributes first (resourceAttributes path, dot notation), then span
// attributes within the same block (customAttributes path, hyphen notation on root span).
func extractWorkspaceMASFromRS(rs ptrace.ResourceSpans) (workspaceID, masID string) {
	attrs := rs.Resource().Attributes()
	if v, ok := attrs.Get("workspace.id"); ok {
		workspaceID = v.Str()
	}
	if v, ok := attrs.Get("mas.id"); ok {
		masID = v.Str()
	}
	if workspaceID != "" && masID != "" {
		return
	}

	// customAttributes path: only the root span has these (hyphen notation).
	sss := rs.ScopeSpans()
	for j := 0; j < sss.Len(); j++ {
		spans := sss.At(j).Spans()
		for k := 0; k < spans.Len(); k++ {
			a := spans.At(k).Attributes()
			if workspaceID == "" {
				if v, ok := a.Get("workspace-id"); ok {
					workspaceID = v.Str()
				}
			}
			if masID == "" {
				if v, ok := a.Get("mas-id"); ok {
					masID = v.Str()
				}
			}
			if workspaceID != "" && masID != "" {
				return
			}
		}
	}
	return
}

func mapSpan(
	span ptrace.Span,
	serviceName, workspaceID, masID string,
	resourceAttrs map[string]any,
	resolver AgentResolver,
) SpanRecord {
	attrs := span.Attributes()

	// agent_id priority: direct attribute → session key resolver → empty
	agentID := ""
	if v, ok := attrs.Get("agent.id"); ok {
		agentID = v.Str()
	}
	if agentID == "" && resolver != nil {
		if v, ok := attrs.Get("openclaw.session.key"); ok {
			agentID = resolver(v.Str())
		}
	}

	startNs := span.StartTimestamp()
	endNs := span.EndTimestamp()
	durationNano := int64(0)
	if endNs > startNs {
		durationNano = int64(endNs - startNs)
	}

	traceID := span.TraceID()
	spanID := span.SpanID()

	record := SpanRecord{
		TraceID:      hex.EncodeToString(traceID[:]),
		SpanID:       hex.EncodeToString(spanID[:]),
		WorkspaceID:  workspaceID,
		MasID:        masID,
		AgentID:      agentID,
		Name:         span.Name(),
		ServiceName:  serviceName,
		Kind:         int(span.Kind()),
		DurationNano: durationNano,
		StartTime:    startNs.AsTime().UTC().Format(time.RFC3339Nano),
		Attributes:   attrs.AsRaw(),
	}

	if parentID := span.ParentSpanID(); !parentID.IsEmpty() {
		record.ParentSpanID = hex.EncodeToString(parentID[:])
	}

	if status := span.Status(); status.Code() != ptrace.StatusCodeUnset {
		record.StatusCode = int(status.Code())
		record.StatusMessage = status.Message()
	}

	if events := span.Events(); events.Len() > 0 {
		record.Events = mapEvents(events)
	}

	if links := span.Links(); links.Len() > 0 {
		record.Links = mapLinks(links)
	}

	if len(resourceAttrs) > 0 {
		record.Resource = resourceAttrs
	}

	return record
}

func mapEvents(events ptrace.SpanEventSlice) []map[string]any {
	out := make([]map[string]any, events.Len())
	for i := 0; i < events.Len(); i++ {
		e := events.At(i)
		out[i] = map[string]any{
			"name":       e.Name(),
			"timestamp":  e.Timestamp().AsTime().UTC().Format(time.RFC3339Nano),
			"attributes": e.Attributes().AsRaw(),
		}
	}
	return out
}

func mapLinks(links ptrace.SpanLinkSlice) []map[string]any {
	out := make([]map[string]any, links.Len())
	for i := 0; i < links.Len(); i++ {
		l := links.At(i)
		tid := l.TraceID()
		sid := l.SpanID()
		out[i] = map[string]any{
			"trace_id":   hex.EncodeToString(tid[:]),
			"span_id":    hex.EncodeToString(sid[:]),
			"attributes": l.Attributes().AsRaw(),
		}
	}
	return out
}
