package app

import (
	"encoding/json"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
)

// otelSpanToRecord converts a persisted OtelSpan (DB schema) to a SpanRecord (API/wire format)
// by unmarshaling JSON fields.
func otelSpanToRecord(s otelreceiver.OtelSpan) otelreceiver.SpanRecord {
	return otelreceiver.SpanRecord{
		StartTime:     s.StartTime.UTC().Format(time.RFC3339Nano),
		TraceID:       s.TraceID,
		SpanID:        s.SpanID,
		ParentSpanID:  s.ParentSpanID,
		WorkspaceID:   s.WorkspaceID.String(),
		MasID:         s.MasID.String(),
		AgentID:       s.AgentID,
		Name:          s.Name,
		ServiceName:   s.ServiceName,
		Kind:          s.Kind,
		DurationNano:  s.DurationNano,
		StatusCode:    s.StatusCode,
		StatusMessage: s.StatusMessage,
		Attributes:    otelJSONMap(s.Attributes),
		Events:        otelJSONMapSlice(s.Events),
		Links:         otelJSONMapSlice(s.Links),
		Resource:      otelJSONMap(s.Resource),
	}
}

// otelJSONMap unmarshals raw JSON bytes into a map. Returns an empty map on parse errors.
func otelJSONMap(raw []byte) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// otelJSONMapSlice unmarshals raw JSON bytes into a slice of maps. Returns nil on parse errors.
func otelJSONMapSlice(raw []byte) []map[string]any {
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
