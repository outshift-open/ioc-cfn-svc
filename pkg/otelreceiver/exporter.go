// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const (
	otelSpansBatchPath = "/api/otel-spans/batch"
	retryAttempts      = 3
	retryBaseDelay     = 500 * time.Millisecond
)

var (
	writerLog  *zap.SugaredLogger
	writerOnce sync.Once
)

func getWriterLogger() *zap.SugaredLogger {
	writerOnce.Do(func() {
		writerLog = logger.SubPkg("otelwriter")
	})
	return writerLog
}

// SpanBatchRequest is the JSON body sent to memory-svc POST /api/otel-spans/batch.
type SpanBatchRequest struct {
	Spans []SpanRecord `json:"spans"`
}

// MemorySvcExporter implements consumer.Traces and component.Component.
// It maps ptrace.Traces to SpanRecords and POSTs them to memory-svc with retry.
// Batching and queueing are handled upstream by the batchprocessor.
type MemorySvcExporter struct {
	memorySvcURL string
	resolver     AgentResolver
	httpClient   *http.Client
}

// NewMemorySvcExporter creates a MemorySvcExporter that posts to memorySvcURL.
func NewMemorySvcExporter(memorySvcURL string, resolver AgentResolver) *MemorySvcExporter {
	return &MemorySvcExporter{
		memorySvcURL: memorySvcURL,
		resolver:     resolver,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
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

// ConsumeTraces maps spans, drops those missing required fields, and posts the rest to memory-svc.
func (e *MemorySvcExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	log := getWriterLogger()

	spans := MapSpans(td, e.resolver)

	valid := spans[:0]
	for _, s := range spans {
		if s.WorkspaceID == "" || s.MasID == "" {
			log.Warnf("otelwriter: dropping span %s — missing workspace_id or mas_id", s.SpanID)
			continue
		}
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		return nil
	}

	return e.postWithRetry(valid)
}

func (e *MemorySvcExporter) postWithRetry(spans []SpanRecord) error {
	log := getWriterLogger()

	payload := SpanBatchRequest{Spans: spans}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		if err := e.postOnce(body, len(spans)); err != nil {
			lastErr = err
			if attempt < retryAttempts {
				delay := retryBaseDelay * (1 << (attempt - 1)) // 500ms, 1s, 2s
				log.Warnf("otelwriter: attempt %d/%d failed (%v), retrying in %s",
					attempt, retryAttempts, err, delay)
				time.Sleep(delay)
				continue
			}
		} else {
			return nil
		}
	}
	return lastErr
}

func (e *MemorySvcExporter) postOnce(body []byte, spanCount int) error {
	log := getWriterLogger()

	url := e.memorySvcURL + otelSpansBatchPath
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("memory-svc returned %d: %s", resp.StatusCode, string(respBody))
	}

	log.Infof("otelwriter: posted %d span(s), status=%d", spanCount, resp.StatusCode)
	return nil
}
