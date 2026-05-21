// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package otelreceiver

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var otelSpansLog = logger.SubPkg("otelspans")

// OtelSpan is the GORM model for the otel_spans TimescaleDB hypertable.
type OtelSpan struct {
	StartTime     time.Time      `gorm:"primaryKey;not null"`
	TraceID       string         `gorm:"type:text;primaryKey;not null"`
	SpanID        string         `gorm:"type:text;primaryKey;not null"`
	WorkspaceID   uuid.UUID      `gorm:"type:uuid;not null;index"`
	MasID         uuid.UUID      `gorm:"type:uuid;not null;index"`
	AgentID       string         `gorm:"type:text"`
	ParentSpanID  string         `gorm:"type:text"`
	OperationName string         `gorm:"type:text;not null"`
	ServiceName   string         `gorm:"type:text"`
	SpanKind      string         `gorm:"type:text"`
	DurationUs    int64          `gorm:"not null"`
	StatusCode    string         `gorm:"type:text"`
	Attributes    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	Events        datatypes.JSON `gorm:"type:jsonb"`
	Links         datatypes.JSON `gorm:"type:jsonb"`
	Resource      datatypes.JSON `gorm:"type:jsonb"`
}

func (OtelSpan) TableName() string { return "otel_spans" }

// MigrateUp creates the otel_spans table and configures it as a TimescaleDB hypertable.
func MigrateUp(db *gorm.DB) error {
	if err := db.AutoMigrate(&OtelSpan{}); err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_otel_spans_workspace_time
		ON otel_spans (workspace_id, start_time)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_otel_spans_mas_time
		ON otel_spans (mas_id, start_time)
	`).Error; err != nil {
		return err
	}

	otelSpansLog.Info("Attempting to enable TimescaleDB extension...")
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE`).Error; err != nil {
		otelSpansLog.Errorf("Failed to create TimescaleDB extension: %v", err)
		return err
	}

	otelSpansLog.Info("Converting otel_spans to TimescaleDB hypertable...")
	if err := db.Exec(`
		SELECT create_hypertable('otel_spans', 'start_time',
			chunk_time_interval => INTERVAL '1 day',
			if_not_exists => TRUE
		)
	`).Error; err != nil {
		otelSpansLog.Errorf("Failed to create hypertable: %v", err)
		return err
	}
	otelSpansLog.Info("Successfully created hypertable with 1-day chunks")

	if err := db.Exec(`
		ALTER TABLE otel_spans SET (
			timescaledb.compress,
			timescaledb.compress_segmentby = 'workspace_id,mas_id,agent_id',
			timescaledb.compress_orderby = 'start_time DESC'
		)
	`).Error; err != nil {
		otelSpansLog.Warnf("Compression not available (requires TimescaleDB Community edition): %v", err)
	} else {
		if err := db.Exec(`
			SELECT add_compression_policy('otel_spans', INTERVAL '7 days')
		`).Error; err != nil {
			otelSpansLog.Warnf("Compression policy not available: %v", err)
		} else {
			otelSpansLog.Info("Successfully enabled compression (7-day policy)")
		}
	}

	if err := db.Exec(`
		SELECT add_retention_policy('otel_spans', INTERVAL '90 days')
	`).Error; err != nil {
		otelSpansLog.Warnf("Retention policy not available: %v", err)
	} else {
		otelSpansLog.Info("Successfully added retention policy")
	}

	return nil
}

// spanRecordToOtelSpan converts a SpanRecord (transport DTO) to an OtelSpan (DB model).
// Returns an error if WorkspaceID, MasID, or StartTime cannot be parsed.
func spanRecordToOtelSpan(r SpanRecord) (OtelSpan, error) {
	startTime, err := time.Parse(time.RFC3339Nano, r.StartTime)
	if err != nil {
		return OtelSpan{}, fmt.Errorf("parse start_time %q: %w", r.StartTime, err)
	}

	wsID, err := uuid.Parse(r.WorkspaceID)
	if err != nil {
		return OtelSpan{}, fmt.Errorf("parse workspace_id %q: %w", r.WorkspaceID, err)
	}

	masID, err := uuid.Parse(r.MasID)
	if err != nil {
		return OtelSpan{}, fmt.Errorf("parse mas_id %q: %w", r.MasID, err)
	}

	attrsJSON, _ := json.Marshal(r.Attributes)
	eventsJSON, _ := json.Marshal(r.Events)
	linksJSON, _ := json.Marshal(r.Links)
	resourceJSON, _ := json.Marshal(r.Resource)

	return OtelSpan{
		StartTime:     startTime,
		TraceID:       r.TraceID,
		SpanID:        r.SpanID,
		ParentSpanID:  r.ParentSpanID,
		WorkspaceID:   wsID,
		MasID:         masID,
		AgentID:       r.AgentID,
		OperationName: r.OperationName,
		ServiceName:   r.ServiceName,
		SpanKind:      r.SpanKind,
		DurationUs:    r.DurationUs,
		StatusCode:    r.StatusCode,
		Attributes:    datatypes.JSON(attrsJSON),
		Events:        datatypes.JSON(eventsJSON),
		Links:         datatypes.JSON(linksJSON),
		Resource:      datatypes.JSON(resourceJSON),
	}, nil
}
