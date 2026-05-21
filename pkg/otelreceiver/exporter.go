// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const otelSpansBatchPath = "/api/otel-spans/batch"

var exporterLog = logger.SubPkg("otelwriter")

// SpanBatchRequest is the JSON body sent to POST /api/otel-spans/batch.
type SpanBatchRequest struct {
	Spans []SpanRecord `json:"spans"`
}

// MemorySvcExporter implements consumer.Traces and component.Component.
// It maps ptrace.Traces to SpanRecords and POSTs them to the configured endpoint via
// httpclient (retry + exponential backoff handled by the client). Batching is handled
// upstream by the batchprocessor.
type MemorySvcExporter struct {
	memorySvcURL string
	resolver     AgentResolver
	httpClient   *httpclient.Client
}

// NewMemorySvcExporter creates a MemorySvcExporter that posts to memorySvcURL.
func NewMemorySvcExporter(memorySvcURL string, resolver AgentResolver) *MemorySvcExporter {
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 30 * time.Second
	cfg.MaxRetries = 2
	cfg.RetryWaitMin = 500 * time.Millisecond
	cfg.RetryWaitMax = 2 * time.Second
	return &MemorySvcExporter{
		memorySvcURL: memorySvcURL,
		resolver:     resolver,
		httpClient:   httpclient.NewWithConfig(cfg),
	}
}

// Capabilities returns consumer capabilities (read-only, does not mutate spans).
func (e *MemorySvcExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is a no-op required by component.Component.
func (e *MemorySvcExporter) Start(_ context.Context, _ component.Host) error { return nil }

// Shutdown is a no-op required by component.Component.
func (e *MemorySvcExporter) Shutdown(_ context.Context) error { return nil }

// ConsumeTraces maps spans, drops those missing required fields, and posts the rest to the configured endpoint.
func (e *MemorySvcExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	spans := MapSpans(td, e.resolver)

	valid := spans[:0]
	for _, s := range spans {
		if s.WorkspaceID == "" || s.MasID == "" {
			exporterLog.Warnf("otelwriter: dropping span %s — missing workspace_id or mas_id", s.SpanID)
			continue
		}
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		return nil
	}

	return e.post(ctx, valid)
}

func (e *MemorySvcExporter) post(ctx context.Context, spans []SpanRecord) error {
	body, err := json.Marshal(SpanBatchRequest{Spans: spans})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := e.memorySvcURL + otelSpansBatchPath
	headers := map[string]string{"Content-Type": "application/json"}

	resp, err := e.httpClient.Post(ctx, url, body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("span storage returned %d: %s", resp.StatusCode, string(respBody))
	}

	exporterLog.Infof("otelwriter: posted %d span(s), status=%d", len(spans), resp.StatusCode)
	return nil
}
