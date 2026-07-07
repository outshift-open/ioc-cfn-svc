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

	// Audit fields (matching the standard Audit table for unified querying)
	AuditType          string `gorm:"size:64;not null;index" json:"audit_type"`
	ResourceType       string `gorm:"size:64;not null;index" json:"resource_type"`
	ResourceIdentifier string `gorm:"size:128;not null;index" json:"resource_identifier"`

	// L9 Header fields
	Kind        string  `gorm:"size:32;not null;index" json:"kind"`
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
	Status   string  `gorm:"size:32;not null" json:"status"` // "success", "failed"
	ErrorMsg *string `gorm:"size:512" json:"error_msg,omitempty"`

	// Tenant scoping
	WorkspaceID string `gorm:"size:128;not null;index" json:"workspace_id"`
	MASID       string `gorm:"size:128;not null;index" json:"mas_id"`

	// Timestamps
	CreatedOn time.Time `gorm:"not null;index" json:"created_on"`
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
		ID:                 uuid.New(),
		AuditType:          L9KindToAuditType(msg.Header.Kind),
		ResourceType:       L9ResourceType(),
		ResourceIdentifier: masID,
		Kind:               string(msg.Header.Kind),
		Protocol:           msg.Header.Protocol,
		Subprotocol:        msg.Header.Subprotocol,
		MessageID:          msg.Header.Message.ID,
		EpisodeID:          msg.Header.Message.Episode,
		WorkspaceID:        workspaceID,
		MASID:              masID,
		CreatedOn:          time.Now(),
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
	if page > MaxPage() {
		page = MaxPage()
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

// L9KindToAuditType maps L9 header.kind to the corresponding AuditType constant.
func L9KindToAuditType(kind l9.Kind) string {
	switch kind {
	case l9.KindIntent:
		return AuditTypeL9Intent
	case l9.KindExchange:
		return AuditTypeL9Exchange
	case l9.KindContingency:
		return AuditTypeL9Contingency
	case l9.KindCommit:
		return AuditTypeL9Commit
	case l9.KindKnowledge:
		return AuditTypeL9Knowledge
	default:
		return "L9_" + string(kind)
	}
}

// L9ResourceType returns the resource type for L9 audit events.
// Currently all L9 events are attributed to the MAS.
// TODO: This will be refined as the L9 protocol evolves.
func L9ResourceType() string {
	return ResourceTypeMAS
}

// L9AuditInformation contains structured L9 metadata stored in audit_information JSONB.
type L9AuditInformation struct {
	Kind        string              `json:"kind"`
	Protocol    string              `json:"protocol"`
	Subprotocol string              `json:"subprotocol"`
	Subkind     string              `json:"subkind,omitempty"`
	EpisodeID   string              `json:"episode_id,omitempty"`
	ParentIDs   []string            `json:"parent_ids,omitempty"`
	Actors      []L9Actor           `json:"actors"`
	Context     *l9.L9HeaderContext `json:"context,omitempty"`
	PayloadType string              `json:"payload_type,omitempty"`
	Status      string              `json:"status"`
	ErrorMsg    string              `json:"error_msg,omitempty"`
}

// NewAuditFromL9Message creates an Audit event from an L9 message.
// Returns nil if the message lacks required fields (message.id or message.episode).
// The mapping is:
//   - audit_type <- L9 header.kind (prefixed with L9_)
//   - resource_type <- MAS or MAS-AGENT (based on whether agentID is present)
//   - resource_identifier <- mas_id or agent_id (the sender)
//   - audit_resource_identifier <- message_id (unique message ID)
//   - operation_id <- workspace_id|mas_id (tenant scope)
//   - audit_information <- L9 metadata (actors, context, parents, episode, etc.)
//   - audit_extra_information <- subkind (converged, rejected, etc.)
func NewAuditFromL9Message(msg *l9.L9, workspaceID, masID, agentID, status, errMsg string) *Audit {
	// message_id and episode_id are required
	if msg.Header.Message == nil || msg.Header.Message.ID == "" || msg.Header.Message.Episode == "" {
		return nil
	}

	auditType := L9KindToAuditType(msg.Header.Kind)

	// Resource type is always MAS, identifier is the MAS ID.
	// TODO: This will be refined as the L9 protocol evolves.
	resourceType := L9ResourceType()
	resourceIdentifier := masID

	// Build operation_id as composite tenant key
	operationID := workspaceID + "|" + masID

	// Build audit_information with L9 metadata
	info := L9AuditInformation{
		Kind:        string(msg.Header.Kind),
		Protocol:    msg.Header.Protocol,
		Subprotocol: msg.Header.Subprotocol,
		EpisodeID:   msg.Header.Message.Episode,
		Status:      status,
		ErrorMsg:    errMsg,
	}

	// Subkind
	if msg.Header.Subkind != nil {
		if s, ok := msg.Header.Subkind.(string); ok && s != "" {
			info.Subkind = s
		}
	}

	// Parent IDs
	if len(msg.Header.Message.Parents) > 0 {
		info.ParentIDs = msg.Header.Message.Parents
	}

	// Actors
	for _, actor := range msg.Header.Participants.Actors {
		info.Actors = append(info.Actors, L9Actor{
			ID:   actor.ID,
			Role: actor.Role,
		})
	}

	// Context
	if msg.Header.Context != nil {
		info.Context = msg.Header.Context
	}

	// Payload type
	if msg.Payload.Type != "" {
		info.PayloadType = msg.Payload.Type
	}

	auditInfoJSON, _ := json.Marshal(info)

	// Build audit_extra_information with subkind if present
	var extraInfo *string
	if info.Subkind != "" {
		extraInfo = &info.Subkind
	}

	// Use a system UUID for L9 automated events
	systemUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	return &Audit{
		OperationID:             &operationID,
		ResourceType:            resourceType,
		ResourceIdentifier:      resourceIdentifier,
		AuditType:               auditType,
		AuditResourceIdentifier: msg.Header.Message.ID,
		AuditInformation:        auditInfoJSON,
		AuditExtraInformation:   extraInfo,
		CreatedBy:               systemUUID,
		LastModifiedBy:          systemUUID,
	}
}

// L9AuditEventToAudit converts an L9AuditEvent (from DB) to an Audit struct for API responses.
// This allows L9 events stored in the dedicated audit_l9 table to be returned via the existing audit API.
// The mapping is:
//   - audit_type <- L9 kind (prefixed with L9_)
//   - resource_type <- MAS (consistent with existing audit pattern)
//   - resource_identifier <- mas_id
//   - audit_resource_identifier <- message_id
//   - operation_id <- workspace_id|mas_id
//   - audit_information <- L9 metadata (protocol, subprotocol, episode, actors, context, parents, status, error)
//   - audit_extra_information <- subkind
func (e *L9AuditEvent) ToAudit() Audit {
	// Build operation_id as composite tenant key
	operationID := e.WorkspaceID + "|" + e.MASID

	// Build audit_information from L9 fields
	var parentIDs []string
	if len(e.ParentIDs) > 0 {
		_ = json.Unmarshal(e.ParentIDs, &parentIDs)
	}

	var actors []L9Actor
	if len(e.Actors) > 0 {
		_ = json.Unmarshal(e.Actors, &actors)
	}

	info := L9AuditInformation{
		Kind:        e.Kind,
		Protocol:    e.Protocol,
		Subprotocol: e.Subprotocol,
		EpisodeID:   e.EpisodeID,
		ParentIDs:   parentIDs,
		Actors:      actors,
		Status:      e.Status,
	}

	if e.Subkind != nil {
		info.Subkind = *e.Subkind
	}
	if e.PayloadType != nil {
		info.PayloadType = *e.PayloadType
	}
	if e.ErrorMsg != nil {
		info.ErrorMsg = *e.ErrorMsg
	}

	// Context is already JSONB, include it in audit_information
	if len(e.Context) > 0 {
		var ctx l9.L9HeaderContext
		if err := json.Unmarshal(e.Context, &ctx); err == nil {
			info.Context = &ctx
		}
	}

	auditInfoJSON, _ := json.Marshal(info)

	// Use a system UUID for L9 automated events
	systemUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	return Audit{
		ID:                      e.ID,
		OperationID:             &operationID,
		ResourceType:            e.ResourceType,
		ResourceIdentifier:      e.ResourceIdentifier,
		AuditType:               e.AuditType,
		AuditResourceIdentifier: e.MessageID,
		AuditInformation:        auditInfoJSON,
		AuditExtraInformation:   e.Subkind,
		CreatedBy:               systemUUID,
		CreatedOn:               e.CreatedOn,
		LastModifiedBy:          systemUUID,
		LastModifiedOn:          e.CreatedOn,
	}
}

// AuditTypeToL9Kind converts an L9 audit type (e.g., "L9_COMMIT") to L9 kind (e.g., "commit").
// Returns empty string if not an L9 audit type.
func AuditTypeToL9Kind(auditType string) string {
	switch auditType {
	case AuditTypeL9Intent:
		return "intent"
	case AuditTypeL9Exchange:
		return "exchange"
	case AuditTypeL9Contingency:
		return "contingency"
	case AuditTypeL9Commit:
		return "commit"
	case AuditTypeL9Knowledge:
		return "knowledge"
	default:
		return ""
	}
}

// IsL9AuditType returns true if the audit type is an L9 protocol type.
func IsL9AuditType(auditType string) bool {
	return AuditTypeToL9Kind(auditType) != ""
}
