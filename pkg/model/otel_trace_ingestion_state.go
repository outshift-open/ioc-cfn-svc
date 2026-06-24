// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// OtelTraceIngestionState tracks the KG ingestion status of individual OTel traces.
// States: pending → ready → running → completed | failed
// The "pending" state indicates spans are still arriving.
// The "ready" state indicates the trace is complete (no spans for N seconds) and ready for KG ingestion.
// The "running" state means cfn-svc has claimed the trace and is pushing its spans to CE.
// Late spans can demote ready/running traces back to pending for another inactivity window.
type OtelTraceIngestionState struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WorkspaceID  string     `gorm:"type:uuid;not null;uniqueIndex:idx_otel_trace_unique"`
	MasID        string     `gorm:"column:mas_id;type:uuid;not null;uniqueIndex:idx_otel_trace_unique"`
	TraceID      string     `gorm:"type:text;not null;uniqueIndex:idx_otel_trace_unique"`
	Status       string     `gorm:"type:text;not null;default:'pending'"`
	LastSpanTime *time.Time `gorm:"type:timestamptz"`
	CreatedAt    time.Time  `gorm:"not null;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"not null;autoUpdateTime"`
}

func (OtelTraceIngestionState) TableName() string {
	return "otel_trace_ingestion_state"
}
