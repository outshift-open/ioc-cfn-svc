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

// SpanExporter implements consumer.Traces and component.Component.
// It maps ptrace.Traces to SpanRecords and POSTs them to the configured endpoint via
// httpclient (retry + exponential backoff handled by the client). Batching is handled
// upstream by the batchprocessor.
type SpanExporter struct {
	spanIngestURL string
	resolver      AgentResolver
	httpClient    *httpclient.Client
}

// NewSpanExporter creates a SpanExporter that posts to spanIngestURL.
func NewSpanExporter(spanIngestURL string, resolver AgentResolver) *SpanExporter {
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 30 * time.Second
	cfg.MaxRetries = 2
	cfg.RetryWaitMin = 500 * time.Millisecond
	cfg.RetryWaitMax = 2 * time.Second
	return &SpanExporter{
		spanIngestURL: spanIngestURL,
		resolver:      resolver,
		httpClient:    httpclient.NewWithConfig(cfg),
	}
}

// Capabilities returns consumer capabilities (read-only, does not mutate spans).
func (e *SpanExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is a no-op required by component.Component.
func (e *SpanExporter) Start(_ context.Context, _ component.Host) error { return nil }

// Shutdown is a no-op required by component.Component.
func (e *SpanExporter) Shutdown(_ context.Context) error { return nil }

// ConsumeTraces maps spans, drops those missing required fields, and posts the rest to the configured endpoint.
func (e *SpanExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	spans := MapSpans(td, e.resolver)

	valid := spans[:0]
	for _, s := range spans {
		if s.WorkspaceID == "" || s.MasID == "" {
			exporterLog.Warnf("otelwriter: dropping span %s (%s) — missing workspace_id or mas_id", s.SpanID, s.OperationName)
			continue
		}
		if s.OperationName == "openclaw.session.stuck" {
			continue
		}
		if ch, ok := s.Attributes["openclaw.message.channel"].(string); ok && ch == "heartbeat" {
			continue
		}
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		return nil
	}

	return e.post(ctx, valid)
}

func (e *SpanExporter) post(ctx context.Context, spans []SpanRecord) error {
	body, err := json.Marshal(SpanBatchRequest{Spans: spans})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := e.spanIngestURL + otelSpansBatchPath
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
