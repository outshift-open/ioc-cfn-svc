// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

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
// to the SpanWriter for batched delivery to the memory service.
type OTLPReceiver struct {
	server   *http.Server
	writer   *SpanWriter
	resolver AgentResolver
}

// New returns an OTLPReceiver bound to the given port.
// resolver is called per-span to map openclaw.session.key → agent_id; may be nil.
func New(port string, writer *SpanWriter, resolver AgentResolver) *OTLPReceiver {
	r := &OTLPReceiver{writer: writer, resolver: resolver}
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

// Start begins serving OTLP/HTTP requests and starts the span flush loop.
func (r *OTLPReceiver) Start() {
	log := getLogger()
	r.writer.Start()
	log.Infof("OTLP receiver listening on %s", r.server.Addr)
	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("OTLP receiver stopped unexpectedly: %v", err)
		}
	}()
}

// Stop drains pending spans, then gracefully shuts down the HTTP server.
func (r *OTLPReceiver) Stop(ctx context.Context) error {
	r.writer.Stop()
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

	var resourceSpans []*tracepb.ResourceSpans

	ct := req.Header.Get("Content-Type")
	switch ct {
	case "application/x-protobuf", "application/octet-stream":
		resourceSpans, err = parseProtobuf(body)
	case "application/json":
		resourceSpans, err = parseJSON(body)
	default:
		// Try binary protobuf for requests without a content-type header.
		resourceSpans, err = parseProtobuf(body)
		if err != nil {
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

	spans := MapSpans(resourceSpans, r.resolver)
	if len(spans) > 0 {
		r.writer.Enqueue(spans)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// parseProtobuf decodes a binary OTLP ExportTraceServiceRequest.
// Field 1 of ExportTraceServiceRequest is repeated ResourceSpans (bytes).
// We parse the outer message manually with protowire to avoid importing the
// collector/trace/v1 package (which transitively pulls in grpc-gateway).
func parseProtobuf(b []byte) ([]*tracepb.ResourceSpans, error) {
	var out []*tracepb.ResourceSpans
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		if num == 1 && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
			rs := &tracepb.ResourceSpans{}
			if err := proto.Unmarshal(v, rs); err != nil {
				return nil, err
			}
			out = append(out, rs)
		} else {
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return out, nil
}

// exportRequestJSON mirrors the JSON encoding of ExportTraceServiceRequest.
// Each element is kept as raw JSON and unmarshalled individually via protojson.
type exportRequestJSON struct {
	ResourceSpans []json.RawMessage `json:"resourceSpans"`
}

// parseJSON decodes an OTLP/JSON ExportTraceServiceRequest.
func parseJSON(b []byte) ([]*tracepb.ResourceSpans, error) {
	var wrapper exportRequestJSON
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, err
	}
	out := make([]*tracepb.ResourceSpans, 0, len(wrapper.ResourceSpans))
	for _, raw := range wrapper.ResourceSpans {
		rs := &tracepb.ResourceSpans{}
		if err := protojson.Unmarshal(raw, rs); err != nil {
			return nil, err
		}
		out = append(out, rs)
	}
	return out, nil
}
