// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/outshift-open/ioc-cfn-svc/pkg/tools/logger"
)

const maxBodyBytes = 4 << 20 // 4 MB

var receiverLog = logger.SubPkg("otelreceiver")

// OTLPReceiver manages the consumer pipeline (batchprocessor → SpanExporter) and
// exposes HandleTraces as an easyHandler to be registered on the main HTTP server.
type OTLPReceiver struct {
	consumer consumer.Traces
}

// New returns an OTLPReceiver wrapping the given consumer.
func New(consumer consumer.Traces) *OTLPReceiver {
	return &OTLPReceiver{consumer: consumer}
}

// Start begins the consumer pipeline.
func (r *OTLPReceiver) Start() error {
	if c, ok := r.consumer.(component.Component); ok {
		if err := c.Start(context.Background(), &minimalHost{}); err != nil {
			return fmt.Errorf("OTLP receiver: failed to start consumer pipeline: %w", err)
		}
	}
	return nil
}

// Stop drains pending spans and shuts down the consumer pipeline.
func (r *OTLPReceiver) Stop(ctx context.Context) error {
	if c, ok := r.consumer.(component.Component); ok {
		return c.Shutdown(ctx)
	}
	return nil
}

// HandleTraces parses an OTLP/HTTP trace export and forwards it to the consumer pipeline.
// Registered as POST /v1/traces on the main HTTP server.
func (r *OTLPReceiver) HandleTraces(w http.ResponseWriter, req *http.Request) (int, error) {
	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes))
	if err != nil {
		receiverLog.Errorf("otelreceiver: failed to read request body: %v", err)
		return http.StatusBadRequest, err
	}

	exportReq := ptraceotlp.NewExportRequest()
	ct := req.Header.Get("Content-Type")
	switch ct {
	case "application/json":
		err = exportReq.UnmarshalJSON(body)
	case "application/x-protobuf", "application/octet-stream", "":
		err = exportReq.UnmarshalProto(body)
	default:
		return http.StatusUnsupportedMediaType, nil
	}

	if err != nil {
		receiverLog.Errorf("otelreceiver: parse failed (content-type=%q): %v", ct, err)
		return http.StatusBadRequest, err
	}

	if err := r.consumer.ConsumeTraces(req.Context(), exportReq.Traces()); err != nil {
		receiverLog.Errorf("otelreceiver: consume failed: %v", err)
		return http.StatusServiceUnavailable, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
	return http.StatusOK, nil
}

// minimalHost satisfies component.Host for the batchprocessor.
type minimalHost struct{}

func (h *minimalHost) GetExtensions() map[component.ID]component.Component { return nil }
