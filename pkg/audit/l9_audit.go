// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// L9AuditEvent represents an audit trail event for L9 protocol messages.
type L9AuditEvent struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	// L9 Header fields
	Kind        string  `gorm:"size:32;not null" json:"kind"`
	Subkind     *string `gorm:"size:32" json:"subkind,omitempty"`
	Protocol    string  `gorm:"size:16" json:"protocol"`
	Subprotocol string  `gorm:"size:16" json:"subprotocol"`

	// Message identity
	MessageID string `gorm:"size:64;not null" json:"message_id"`
	// Episode identity
	EpisodeID string `gorm:"size:128;not null;index" json:"episode_id"`
	// Parents identity
	ParentIDs datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"parent_ids"`

	// Participants (stored as JSONB array of {id, role} objects)
	Actors datatypes.JSON `gorm:"type:jsonb;not null" json:"actors"`

	// Context (full header.context as JSONB)
	Context datatypes.JSON `gorm:"type:jsonb" json:"context,omitempty"`

	// Payload type
	PayloadType *string `gorm:"size:64" json:"payload_type,omitempty"`

	// Processing status
	Status   string  `gorm:"size:32;not null" json:"status"`        // "success", "failed"
	ErrorMsg *string `gorm:"size:512" json:"error_msg,omitempty"`

	// Tenant scoping
	WorkspaceID string `gorm:"size:128;not null" json:"workspace_id"`
	MASID       string `gorm:"size:128;not null" json:"mas_id"`

	// Timestamps
	CreatedOn time.Time `gorm:"not null" json:"created_on"`
}

// TableName overrides the default table name.
func (L9AuditEvent) TableName() string {
	return "audit_l9"
}

// L9Actor represents a participant actor in an L9 message for JSON storage.
type L9Actor struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// L9AuditListResponse wraps a page of L9 audit events with pagination metadata.
type L9AuditListResponse struct {
	Data     []L9AuditEvent `json:"data"`
	PageInfo PageInfo       `json:"pageInfo"`
}

// MigrateL9Up runs GORM AutoMigrate for the L9 audit table.
func MigrateL9Up(db *gorm.DB) error {
	return db.AutoMigrate(&L9AuditEvent{})
}

// NewL9AuditEventFromMessage creates an L9AuditEvent from an L9 message and CFN context.
// Returns nil if the message lacks required fields (message.id or message.episode).
func NewL9AuditEventFromMessage(msg *l9.L9, workspaceID, masID string) *L9AuditEvent {
	// message_id and episode_id are required
	if msg.Header.Message == nil || msg.Header.Message.ID == "" || msg.Header.Message.Episode == "" {
		return nil
	}

	event := &L9AuditEvent{
		ID:          uuid.New(),
		Kind:        string(msg.Header.Kind),
		Protocol:    msg.Header.Protocol,
		Subprotocol: msg.Header.Subprotocol,
		MessageID:   msg.Header.Message.ID,
		EpisodeID:   msg.Header.Message.Episode,
		WorkspaceID: workspaceID,
		MASID:       masID,
		CreatedOn:   time.Now(),
	}

	// Subkind (interface{} in L9, convert to string pointer)
	if msg.Header.Subkind != nil {
		if s, ok := msg.Header.Subkind.(string); ok && s != "" {
			event.Subkind = &s
		}
	}

	// Parent IDs
	if len(msg.Header.Message.Parents) > 0 {
		parentsJSON, _ := json.Marshal(msg.Header.Message.Parents)
		event.ParentIDs = parentsJSON
	} else {
		event.ParentIDs = []byte("[]")
	}

	// Actors from participants
	actors := make([]L9Actor, 0, len(msg.Header.Participants.Actors))
	for _, actor := range msg.Header.Participants.Actors {
		actors = append(actors, L9Actor{
			ID:   actor.ID,
			Role: actor.Role,
		})
	}
	actorsJSON, _ := json.Marshal(actors)
	event.Actors = actorsJSON

	// Context (store entire context as JSONB)
	if msg.Header.Context != nil {
		contextJSON, _ := json.Marshal(msg.Header.Context)
		event.Context = contextJSON
	}

	// Payload type
	if msg.Payload.Type != "" {
		event.PayloadType = &msg.Payload.Type
	}

	return event
}

// CreateL9AuditEvent inserts a new L9 audit event.
func CreateL9AuditEvent(db *gorm.DB, e *L9AuditEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedOn.IsZero() {
		e.CreatedOn = time.Now()
	}
	return db.Create(e).Error
}

// GetL9AuditEventByID retrieves a single L9 audit event by its UUID.
func GetL9AuditEventByID(db *gorm.DB, id uuid.UUID) (*L9AuditEvent, error) {
	var e L9AuditEvent
	if err := db.Where("id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListL9AuditEvents returns a page of L9 audit events with optional kind and episode filters.
// page is 0-based. pageSize is clamped to [1, MaxPageSize()] and defaults to DefaultPageSize().
func ListL9AuditEvents(db *gorm.DB, kind, episodeID string, page, pageSize int) (*L9AuditListResponse, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize()
	}
	if pageSize > MaxPageSize() {
		pageSize = MaxPageSize()
	}
	if page < 0 {
		page = 0
	}

	query := db.Model(&L9AuditEvent{})
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if episodeID != "" {
		query = query.Where("episode_id = ?", episodeID)
	}

	var totalElements int64
	if err := query.Session(&gorm.Session{}).Count(&totalElements).Error; err != nil {
		return nil, err
	}

	events := make([]L9AuditEvent, 0)
	offset := page * pageSize
	if err := query.Session(&gorm.Session{}).Offset(offset).Limit(pageSize).Order("created_on DESC").Find(&events).Error; err != nil {
		return nil, err
	}

	return &L9AuditListResponse{
		Data: events,
		PageInfo: PageInfo{
			Page:          page,
			PageSize:      pageSize,
			PageCount:     len(events),
			TotalElements: int(totalElements),
		},
	}, nil
}

// GetL9AuditEventsByEpisode returns all L9 audit events for a given episode.
func GetL9AuditEventsByEpisode(db *gorm.DB, episodeID string) ([]L9AuditEvent, error) {
	var events []L9AuditEvent
	if err := db.Where("episode_id = ?", episodeID).Order("created_on ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
