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

	"go.uber.org/zap"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const (
	defaultBatchSize     = 100
	defaultFlushInterval = 5 * time.Second
	// maxPendingMultiplier caps the buffer at this many times the batch size.
	// Once full, incoming spans are dropped (with a warning) rather than
	// blocking the request handler — telemetry should never backpressure
	// the main agent flow.
	maxPendingMultiplier = 10

	otelSpansBatchPath = "/api/otel-spans/batch"

	retryAttempts  = 3
	retryBaseDelay = 500 * time.Millisecond
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

// SpanWriter buffers SpanRecords and flushes them to memory-svc in batches.
// It applies backpressure by dropping spans when the in-memory buffer is full,
// and retries failed flushes with exponential backoff.
type SpanWriter struct {
	memorySvcURL  string
	batchSize     int
	maxPending    int
	flushInterval time.Duration
	httpClient    *http.Client

	mu      sync.Mutex
	pending []SpanRecord
	dropped int64 // total spans dropped due to backpressure

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSpanWriter creates a SpanWriter that posts to memorySvcURL.
func NewSpanWriter(memorySvcURL string, batchSize int, flushInterval time.Duration) *SpanWriter {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}
	return &SpanWriter{
		memorySvcURL:  memorySvcURL,
		batchSize:     batchSize,
		maxPending:    batchSize * maxPendingMultiplier,
		flushInterval: flushInterval,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		stopCh:        make(chan struct{}),
	}
}

// Start launches the background flush loop.
func (w *SpanWriter) Start() {
	w.wg.Add(1)
	go w.flushLoop()
}

// Stop flushes all pending spans then shuts down.
func (w *SpanWriter) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	w.flushNow() // final drain
}

// Enqueue adds spans to the buffer.
// If the buffer is at capacity, incoming spans are dropped (backpressure).
// A flush is triggered immediately when the buffer reaches batchSize.
func (w *SpanWriter) Enqueue(spans []SpanRecord) {
	log := getWriterLogger()

	valid := spans[:0]
	for _, s := range spans {
		if s.WorkspaceID == "" || s.MasID == "" {
			log.Warnf("otelwriter: dropping span %s — missing workspace_id or mas_id", s.SpanID)
			continue
		}
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		return
	}
	spans = valid

	w.mu.Lock()
	available := w.maxPending - len(w.pending)
	if available <= 0 {
		w.dropped += int64(len(spans))
		w.mu.Unlock()
		log.Warnf("otelwriter: buffer full (max=%d), dropped %d span(s) — total dropped=%d",
			w.maxPending, len(spans), w.dropped)
		return
	}
	if len(spans) > available {
		dropped := len(spans) - available
		w.dropped += int64(dropped)
		spans = spans[:available]
		log.Warnf("otelwriter: buffer nearly full, dropped %d span(s) — total dropped=%d",
			dropped, w.dropped)
	}
	w.pending = append(w.pending, spans...)
	ready := len(w.pending) >= w.batchSize
	w.mu.Unlock()

	if ready {
		w.flushNow()
	}
}

func (w *SpanWriter) flushLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.flushNow()
		case <-w.stopCh:
			return
		}
	}
}

func (w *SpanWriter) flushNow() {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.pending
	w.pending = nil
	w.mu.Unlock()

	if err := w.postWithRetry(batch); err != nil {
		getWriterLogger().Errorf("otelwriter: permanently failed to post %d span(s) after %d attempts: %v",
			len(batch), retryAttempts, err)
	}
}

// postWithRetry attempts to POST the batch to memory-svc, retrying up to
// retryAttempts times with exponential backoff on transient failures.
func (w *SpanWriter) postWithRetry(spans []SpanRecord) error {
	log := getWriterLogger()

	payload := SpanBatchRequest{Spans: spans}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		if err := w.postOnce(body, len(spans)); err != nil {
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

func (w *SpanWriter) postOnce(body []byte, spanCount int) error {
	log := getWriterLogger()

	url := w.memorySvcURL + otelSpansBatchPath
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
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
