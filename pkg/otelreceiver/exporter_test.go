// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type mockSpanStore struct {
	inserted []OtelSpan
	err      error
}

func (m *mockSpanStore) BulkInsertOtelSpans(spans []OtelSpan) error {
	if m.err != nil {
		return m.err
	}
	m.inserted = append(m.inserted, spans...)
	return nil
}

var (
	spanStart = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	spanEnd   = spanStart.Add(time.Second)
)

func buildSingleSpanTraces(workspaceID, masID, operationName string, extraAttrs map[string]string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if workspaceID != "" {
		rs.Resource().Attributes().PutStr("workspace.id", workspaceID)
	}
	if masID != "" {
		rs.Resource().Attributes().PutStr("mas.id", masID)
	}
	rs.Resource().Attributes().PutStr("service.name", "test-svc")

	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(operationName)
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{1}))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(spanStart))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(spanEnd))
	for k, v := range extraAttrs {
		span.Attributes().PutStr(k, v)
	}
	return td
}

func TestConsumeTraces_ValidSpan(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces(testWorkspaceID, testMasID, "llm.call", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Len(t, store.inserted, 1)
	assert.Equal(t, "llm.call", store.inserted[0].Name)
}

func TestConsumeTraces_DropsSpanMissingWorkspaceID(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces("", testMasID, "llm.call", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_DropsSpanMissingMasID(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces(testWorkspaceID, "", "llm.call", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_DropsSessionStuck(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces(testWorkspaceID, testMasID, "openclaw.session.stuck", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_DropsHeartbeat(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces(testWorkspaceID, testMasID, "some.op", map[string]string{
		"openclaw.message.channel": "heartbeat",
	})
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_DropsSpanWithInvalidWorkspaceUUID(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces("not-a-uuid", testMasID, "llm.call", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_NoSpansCallsNoInsert(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	err := exp.ConsumeTraces(context.Background(), ptrace.NewTraces())
	require.NoError(t, err)
	assert.Empty(t, store.inserted)
}

func TestConsumeTraces_StoreErrorPropagates(t *testing.T) {
	store := &mockSpanStore{err: errors.New("db down")}
	exp := NewSpanExporter(store, nil)

	td := buildSingleSpanTraces(testWorkspaceID, testMasID, "llm.call", nil)
	err := exp.ConsumeTraces(context.Background(), td)
	assert.ErrorContains(t, err, "db down")
}

func TestConsumeTraces_MixedValidAndDropped(t *testing.T) {
	store := &mockSpanStore{}
	exp := NewSpanExporter(store, nil)

	td := ptrace.NewTraces()

	addSpan := func(td ptrace.Traces, wsID, masID, name string, attrs map[string]string) {
		rs := td.ResourceSpans().AppendEmpty()
		if wsID != "" {
			rs.Resource().Attributes().PutStr("workspace.id", wsID)
		}
		if masID != "" {
			rs.Resource().Attributes().PutStr("mas.id", masID)
		}
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetName(name)
		s.SetTraceID(pcommon.TraceID([16]byte{byte(td.ResourceSpans().Len())}))
		s.SetSpanID(pcommon.SpanID([8]byte{byte(td.ResourceSpans().Len())}))
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(spanStart))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(spanEnd))
		for k, v := range attrs {
			s.Attributes().PutStr(k, v)
		}
	}

	addSpan(td, testWorkspaceID, testMasID, "valid.op", nil)
	addSpan(td, testWorkspaceID, testMasID, "msg.op", map[string]string{"openclaw.message.channel": "heartbeat"})
	addSpan(td, testWorkspaceID, testMasID, "openclaw.session.stuck", nil)
	addSpan(td, "", testMasID, "missing.workspace", nil)

	err := exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	assert.Len(t, store.inserted, 1)
	assert.Equal(t, "valid.op", store.inserted[0].Name)
}
