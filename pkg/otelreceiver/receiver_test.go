// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type mockConsumer struct {
	received []ptrace.Traces
	err      error
}

func (m *mockConsumer) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	if m.err != nil {
		return m.err
	}
	m.received = append(m.received, td)
	return nil
}

func (m *mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func buildOTLPProtoBody(t *testing.T) []byte {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("workspace.id", testWorkspaceID)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("test.op")
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{1}))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(spanStart))

	req := ptraceotlp.NewExportRequestFromTraces(td)
	body, err := req.MarshalProto()
	require.NoError(t, err)
	return body
}

func buildOTLPJSONBody(t *testing.T) []byte {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("workspace.id", testWorkspaceID)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("test.op")
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{1}))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(spanStart))

	req := ptraceotlp.NewExportRequestFromTraces(td)
	body, err := req.MarshalJSON()
	require.NoError(t, err)
	return body
}

func TestHandleTraces_ProtoBody(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	body := buildOTLPProtoBody(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	rr := httptest.NewRecorder()

	code, err := rcvr.HandleTraces(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, mc.received, 1)
}

func TestHandleTraces_JSONBody(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	body := buildOTLPJSONBody(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, err := rcvr.HandleTraces(rr, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, mc.received, 1)
}

func TestHandleTraces_UnsupportedContentType(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte("body")))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	code, err := rcvr.HandleTraces(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnsupportedMediaType, code)
	assert.Empty(t, mc.received)
}

func TestHandleTraces_InvalidProtoBody(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte("not-proto")))
	req.Header.Set("Content-Type", "application/x-protobuf")
	rr := httptest.NewRecorder()

	code, err := rcvr.HandleTraces(rr, req)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Empty(t, mc.received)
}

func TestHandleTraces_InvalidJSONBody(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, err := rcvr.HandleTraces(rr, req)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestHandleTraces_EmptyBodyDefaultsToProto(t *testing.T) {
	mc := &mockConsumer{}
	rcvr := New(mc)

	// empty content-type falls through to proto path — valid empty proto is accepted
	req := ptraceotlp.NewExportRequestFromTraces(ptrace.NewTraces())
	body, err := req.MarshalProto()
	require.NoError(t, err)

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	code, handlerErr := rcvr.HandleTraces(rr, httpReq)
	require.NoError(t, handlerErr)
	assert.Equal(t, http.StatusOK, code)
}
