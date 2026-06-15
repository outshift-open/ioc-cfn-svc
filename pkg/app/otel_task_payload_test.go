package app

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
)

type otelTaskPayloadDB struct {
	*client.MockDatabase

	claimedTraceIDs []string
	spansByTraceID  map[string][]otelreceiver.OtelSpan

	claimWorkspaceID string
	claimMasID       string
	claimLimit       int
	claimDelay       time.Duration
}

func (db *otelTaskPayloadDB) ClaimReadyOtelTraces(workspaceID, masID string, limit int, inactivityThreshold time.Duration) ([]string, error) {
	db.claimWorkspaceID = workspaceID
	db.claimMasID = masID
	db.claimLimit = limit
	db.claimDelay = inactivityThreshold
	return db.claimedTraceIDs, nil
}

func (db *otelTaskPayloadDB) GetOtelSpansForTrace(_, _, traceID string) ([]otelreceiver.OtelSpan, error) {
	return db.spansByTraceID[traceID], nil
}

func TestBuildReadyOtelTaskPayload(t *testing.T) {
	workspaceID := uuid.NewString()
	masID := uuid.NewString()
	startTime := time.Date(2026, 6, 3, 12, 0, 0, 123, time.FixedZone("PDT", -7*60*60))

	db := &otelTaskPayloadDB{
		MockDatabase:    client.NewMockDatabase(),
		claimedTraceIDs: []string{"trace-1"},
		spansByTraceID: map[string][]otelreceiver.OtelSpan{
			"trace-1": {
				{
					StartTime:     startTime,
					TraceID:       "trace-1",
					SpanID:        "span-1",
					ParentSpanID:  "parent-1",
					WorkspaceID:   uuid.MustParse(workspaceID),
					MasID:         uuid.MustParse(masID),
					Name:          "agent.reply",
					ServiceName:   "openclaw",
					Kind:          1,
					DurationNano:  2500,
					StatusCode:    1,
					StatusMessage: "ok",
					Attributes:    datatypes.JSON([]byte(`{"openclaw.message.channel":"final"}`)),
					Events:        datatypes.JSON([]byte(`[]`)),
					Links:         datatypes.JSON([]byte(`[]`)),
					Resource:      datatypes.JSON([]byte(`{"service.name":"openclaw","openclaw.plugin":"openclaw-deep-observability"}`)),
				},
			},
		},
	}
	app := &App{
		db: db,
		Cfg: config.Config{
			TraceCompletion: config.TraceCompletionConfig{
				InactivityTimeout: 45 * time.Second,
			},
		},
	}

	payload, err := app.BuildReadyOtelTaskPayload(workspaceID, masID, 0)
	require.NoError(t, err)

	assert.Equal(t, workspaceID, db.claimWorkspaceID)
	assert.Equal(t, masID, db.claimMasID)
	assert.Equal(t, defaultReadyOtelTraceLimit, db.claimLimit)
	assert.Equal(t, 45*time.Second, db.claimDelay)

	require.NotNil(t, payload)
	assert.Equal(t, common.FormatOTelTrace, payload.Format)
	assert.Equal(t, 1, payload.TraceCount)
	assert.Equal(t, 1, payload.SpanCount)
	require.Len(t, payload.Traces, 1)
	assert.Equal(t, "trace-1", payload.Traces[0].TraceID)
	assert.Equal(t, 1, payload.Traces[0].SpanCount)
	require.Len(t, payload.Traces[0].Spans, 1)

	span := payload.Traces[0].Spans[0]
	assert.Equal(t, "2026-06-03T19:00:00.000000123Z", span.StartTime)
	assert.Equal(t, "span-1", span.SpanID)
	assert.Equal(t, "final", span.Attributes["openclaw.message.channel"])
	assert.Equal(t, "openclaw", span.Resource["service.name"])
	assert.Equal(t, "openclaw-deep-observability", span.Resource["openclaw.plugin"])
}


func TestBuildReadyOtelTaskPayloadNoReadyTraces(t *testing.T) {
	db := &otelTaskPayloadDB{
		MockDatabase:   client.NewMockDatabase(),
		spansByTraceID: map[string][]otelreceiver.OtelSpan{},
	}
	app := &App{db: db}

	payload, err := app.BuildReadyOtelTaskPayload(uuid.NewString(), uuid.NewString(), 25)
	require.NoError(t, err)

	require.NotNil(t, payload)
	assert.Equal(t, common.FormatOTelTrace, payload.Format)
	assert.Equal(t, 0, payload.TraceCount)
	assert.Equal(t, 0, payload.SpanCount)
	assert.Empty(t, payload.Traces)
	assert.Equal(t, 25, db.claimLimit)
}
