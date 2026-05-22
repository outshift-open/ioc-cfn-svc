// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	testWorkspaceID = "7430ce19-8858-4c96-b4fc-563bdc93fdfe"
	testMasID       = "2a5bd0f8-2e94-4b77-8332-7b0826e61c88"
	testAgentID     = "d0770cb5-d30b-4962-a7a9-2c939f9cf83c"
)

// buildTraces constructs a minimal ptrace.Traces with one span.
func buildTraces(resourceAttrs map[string]string, spanAttrs map[string]string, spanName string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range resourceAttrs {
		rs.Resource().Attributes().PutStr(k, v)
	}
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName(spanName)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)

	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(500 * time.Millisecond)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))

	for k, v := range spanAttrs {
		span.Attributes().PutStr(k, v)
	}

	return td
}

func TestMapSpans_ResourceAttributesPath(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, nil, "test.op")

	records := MapSpans(td, nil)
	require.Len(t, records, 1)
	assert.Equal(t, testWorkspaceID, records[0].WorkspaceID)
	assert.Equal(t, testMasID, records[0].MasID)
	assert.Equal(t, "test-service", records[0].ServiceName)
	assert.Equal(t, "test.op", records[0].Name)
}

func TestMapSpans_SpanAttributesFallback(t *testing.T) {
	td := buildTraces(nil, map[string]string{
		"workspace-id": testWorkspaceID,
		"mas-id":       testMasID,
	}, "test.op")

	records := MapSpans(td, nil)
	require.Len(t, records, 1)
	assert.Equal(t, testWorkspaceID, records[0].WorkspaceID)
	assert.Equal(t, testMasID, records[0].MasID)
}

func TestMapSpans_ResourceAttributesTakePrecedence(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, map[string]string{
		"workspace-id": "should-not-be-used",
		"mas-id":       "should-not-be-used",
	}, "test.op")

	records := MapSpans(td, nil)
	require.Len(t, records, 1)
	assert.Equal(t, testWorkspaceID, records[0].WorkspaceID)
	assert.Equal(t, testMasID, records[0].MasID)
}

func TestMapSpans_AgentIDFromDirectAttribute(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, map[string]string{
		"agent.id": testAgentID,
	}, "test.op")

	records := MapSpans(td, nil)
	require.Len(t, records, 1)
	assert.Equal(t, testAgentID, records[0].AgentID)
}

func TestMapSpans_AgentIDFromResolver(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, map[string]string{
		"openclaw.session.key": "agent:sre:telegram:123",
	}, "test.op")

	resolver := func(sessionKey string) string {
		if sessionKey == "agent:sre:telegram:123" {
			return testAgentID
		}
		return ""
	}

	records := MapSpans(td, resolver)
	require.Len(t, records, 1)
	assert.Equal(t, testAgentID, records[0].AgentID)
}

func TestMapSpans_AgentIDDirectTakesPrecedenceOverResolver(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, map[string]string{
		"agent.id":             testAgentID,
		"openclaw.session.key": "agent:sre:telegram:123",
	}, "test.op")

	resolver := func(_ string) string { return "should-not-be-used" }

	records := MapSpans(td, resolver)
	require.Len(t, records, 1)
	assert.Equal(t, testAgentID, records[0].AgentID)
}

func TestMapSpans_DurationAndIDs(t *testing.T) {
	td := buildTraces(map[string]string{
		"workspace.id": testWorkspaceID,
		"mas.id":       testMasID,
	}, nil, "test.op")

	records := MapSpans(td, nil)
	require.Len(t, records, 1)

	assert.Equal(t, int64(500000000), records[0].DurationNano) // 500ms = 500_000_000 ns
	assert.Equal(t, hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), records[0].TraceID)
	assert.Equal(t, hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8}), records[0].SpanID)
	assert.Equal(t, "2026-01-01T12:00:00Z", records[0].StartTime)
}

func TestMapSpans_ParentSpanID(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("workspace.id", testWorkspaceID)
	rs.Resource().Attributes().PutStr("mas.id", testMasID)

	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("child.op")
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{2}))
	span.SetParentSpanID(pcommon.SpanID([8]byte{1}))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	records := MapSpans(td, nil)
	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0].ParentSpanID)
}

func TestMapSpans_EmptyTraces(t *testing.T) {
	records := MapSpans(ptrace.NewTraces(), nil)
	assert.Nil(t, records)
}

func TestSpanRecordToOtelSpan_HappyPath(t *testing.T) {
	r := SpanRecord{
		TraceID:      "0102030405060708090a0b0c0d0e0f10",
		SpanID:       "0102030405060708",
		WorkspaceID:  testWorkspaceID,
		MasID:        testMasID,
		AgentID:      testAgentID,
		Name:         "test.op",
		ServiceName:  "svc",
		Kind:         int(ptrace.SpanKindClient),
		DurationNano: 500000000,
		StatusCode:   int(ptrace.StatusCodeOk),
		StartTime:    "2026-01-01T12:00:00Z",
		Attributes:   map[string]any{"key": "val"},
	}

	span, err := spanRecordToOtelSpan(r)
	require.NoError(t, err)

	assert.Equal(t, "2026-01-01 12:00:00 +0000 UTC", span.StartTime.UTC().String())
	assert.Equal(t, testWorkspaceID, span.WorkspaceID.String())
	assert.Equal(t, testMasID, span.MasID.String())
	assert.Equal(t, testAgentID, span.AgentID)
	assert.Equal(t, "test.op", span.Name)
	assert.Equal(t, int64(500000000), span.DurationNano)
	assert.NotEmpty(t, span.Attributes)
}

func TestSpanRecordToOtelSpan_InvalidWorkspaceID(t *testing.T) {
	r := SpanRecord{
		WorkspaceID: "not-a-uuid",
		MasID:       testMasID,
		StartTime:   "2026-01-01T12:00:00Z",
	}
	_, err := spanRecordToOtelSpan(r)
	assert.ErrorContains(t, err, "workspace_id")
}

func TestSpanRecordToOtelSpan_InvalidMasID(t *testing.T) {
	r := SpanRecord{
		WorkspaceID: testWorkspaceID,
		MasID:       "not-a-uuid",
		StartTime:   "2026-01-01T12:00:00Z",
	}
	_, err := spanRecordToOtelSpan(r)
	assert.ErrorContains(t, err, "mas_id")
}

func TestSpanRecordToOtelSpan_InvalidStartTime(t *testing.T) {
	r := SpanRecord{
		WorkspaceID: testWorkspaceID,
		MasID:       testMasID,
		StartTime:   "not-a-time",
	}
	_, err := spanRecordToOtelSpan(r)
	assert.ErrorContains(t, err, "start_time")
}
