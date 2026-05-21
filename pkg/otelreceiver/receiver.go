// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

const maxBodyBytes = 4 << 20 // 4 MB

var (
	receiverLog  *zap.SugaredLogger
	receiverOnce sync.Once
)

func getLogger() *zap.SugaredLogger {
	receiverOnce.Do(func() {
		receiverLog = logger.SubPkg("otelreceiver")
	})
	return receiverLog
}

// OTLPReceiver listens for OTLP/HTTP trace exports and forwards parsed spans
// to the consumer pipeline (batchprocessor → MemorySvcExporter).
type OTLPReceiver struct {
	server   *http.Server
	consumer consumer.Traces
}

// New returns an OTLPReceiver bound to the given port.
// consumer is normally the batchprocessor wrapping the MemorySvcExporter.
func New(port string, consumer consumer.Traces) *OTLPReceiver {
	r := &OTLPReceiver{consumer: consumer}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.handleTraces)
	r.server = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	return r
}

// Start begins serving OTLP/HTTP requests and starts the consumer pipeline.
func (r *OTLPReceiver) Start() {
	log := getLogger()
	if c, ok := r.consumer.(component.Component); ok {
		if err := c.Start(context.Background(), &minimalHost{}); err != nil {
			log.Errorf("OTLP receiver: failed to start consumer pipeline: %v", err)
		}
	}
	log.Infof("OTLP receiver listening on %s", r.server.Addr)
	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("OTLP receiver stopped unexpectedly: %v", err)
		}
	}()
}

// Stop drains pending spans then gracefully shuts down the HTTP server.
func (r *OTLPReceiver) Stop(ctx context.Context) error {
	if c, ok := r.consumer.(component.Component); ok {
		_ = c.Shutdown(ctx)
	}
	return r.server.Shutdown(ctx)
}

func (r *OTLPReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	log := getLogger()

	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes))
	if err != nil {
		log.Errorf("otelreceiver: failed to read request body: %v", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	exportReq := ptraceotlp.NewExportRequest()
	ct := req.Header.Get("Content-Type")
	switch ct {
	case "application/json":
		err = exportReq.UnmarshalJSON(body)
	default:
		err = exportReq.UnmarshalProto(body)
		if err != nil && ct != "application/x-protobuf" && ct != "application/octet-stream" {
			log.Warnf("otelreceiver: unsupported content-type %q and protobuf fallback failed: %v", ct, err)
			http.Error(w, "unsupported content-type", http.StatusUnsupportedMediaType)
			return
		}
	}

	if err != nil {
		log.Errorf("otelreceiver: parse failed (content-type=%q): %v", ct, err)
		http.Error(w, "failed to parse OTLP request", http.StatusBadRequest)
		return
	}

	if err := r.consumer.ConsumeTraces(req.Context(), exportReq.Traces()); err != nil {
		log.Errorf("otelreceiver: consume failed: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// minimalHost satisfies component.Host for the batchprocessor.
type minimalHost struct{}

func (h *minimalHost) GetExtensions() map[component.ID]component.Component { return nil }
