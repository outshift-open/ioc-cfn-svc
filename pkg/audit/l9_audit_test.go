// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupL9TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := MigrateL9Up(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestL9MigrateUp(t *testing.T) {
	db := setupL9TestDB(t)
	assert.True(t, db.Migrator().HasTable(&L9AuditEvent{}))
}

func TestCreateL9AuditEvent(t *testing.T) {
	db := setupL9TestDB(t)

	subkind := "query"
	payloadType := "request"

	e := &L9AuditEvent{
		AuditType:          AuditTypeL9Knowledge,
		ResourceType:       ResourceTypeMAS,
		ResourceIdentifier: "mas-1",
		Kind:               "knowledge",
		Subkind:            &subkind,
		Protocol:           "sstp",
		Subprotocol:        "snp",
		MessageID:          "msg-123",
		EpisodeID:          "ep-456",
		ParentIDs:          []byte(`["msg-100"]`),
		Actors:             []byte(`[{"id":"agent-1","role":"sender"}]`),
		Context:            []byte(`{"topic":"semantic-alignment"}`),
		PayloadType:        &payloadType,
		Status:             "success",
		WorkspaceID:        "ws-1",
		MASID:              "mas-1",
	}

	err := CreateL9AuditEvent(db, e)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.False(t, e.CreatedOn.IsZero())
}

func TestGetL9AuditEventByID(t *testing.T) {
	db := setupL9TestDB(t)

	e := &L9AuditEvent{
		AuditType:          AuditTypeL9Intent,
		ResourceType:       ResourceTypeMAS,
		ResourceIdentifier: "mas-1",
		Kind:               "intent",
		Protocol:           "sstp",
		Subprotocol:        "ioc",
		MessageID:          "msg-001",
		EpisodeID:          "ep-001",
		Actors:             []byte(`[]`),
		Status:             "success",
		WorkspaceID:        "ws-1",
		MASID:              "mas-1",
	}
	err := CreateL9AuditEvent(db, e)
	assert.NoError(t, err)

	found, err := GetL9AuditEventByID(db, e.ID)
	assert.NoError(t, err)
	assert.Equal(t, e.ID, found.ID)
	assert.Equal(t, "intent", found.Kind)
	assert.Equal(t, "ioc", found.Subprotocol)
}

func TestGetL9AuditEventByID_NotFound(t *testing.T) {
	db := setupL9TestDB(t)

	_, err := GetL9AuditEventByID(db, uuid.New())
	assert.Error(t, err)
}

func TestListL9AuditEvents_NoFilters(t *testing.T) {
	db := setupL9TestDB(t)

	for i := 0; i < 3; i++ {
		e := &L9AuditEvent{
			Kind:        "exchange",
			Protocol:    "sstp",
			Subprotocol: "tfp",
			MessageID:   "msg-" + string(rune('a'+i)),
			EpisodeID:   "ep-001",
			Actors:      []byte(`[]`),
			WorkspaceID: "ws-1",
			MASID:       "mas-1",
		}
		assert.NoError(t, CreateL9AuditEvent(db, e))
	}

	resp, err := ListL9AuditEvents(db, "", "", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 3)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, DefaultPageSize(), resp.PageInfo.PageSize)
	assert.Equal(t, 3, resp.PageInfo.PageCount)
	assert.Equal(t, 3, resp.PageInfo.TotalElements)
}

func TestListL9AuditEvents_FilterByKind(t *testing.T) {
	db := setupL9TestDB(t)

	e1 := &L9AuditEvent{
		Kind:        "knowledge",
		Protocol:    "sstp",
		Subprotocol: "ioc",
		MessageID:   "msg-001",
		EpisodeID:   "ep-001",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e2 := &L9AuditEvent{
		Kind:        "intent",
		Protocol:    "sstp",
		Subprotocol: "ioc",
		MessageID:   "msg-002",
		EpisodeID:   "ep-001",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	assert.NoError(t, CreateL9AuditEvent(db, e1))
	assert.NoError(t, CreateL9AuditEvent(db, e2))

	resp, err := ListL9AuditEvents(db, "intent", "", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "intent", resp.Data[0].Kind)
}

func TestListL9AuditEvents_FilterByEpisode(t *testing.T) {
	db := setupL9TestDB(t)

	e1 := &L9AuditEvent{
		Kind:        "exchange",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-001",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e2 := &L9AuditEvent{
		Kind:        "commit",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-002",
		EpisodeID:   "episode-2",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	assert.NoError(t, CreateL9AuditEvent(db, e1))
	assert.NoError(t, CreateL9AuditEvent(db, e2))

	resp, err := ListL9AuditEvents(db, "", "episode-1", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "episode-1", resp.Data[0].EpisodeID)
}

func TestListL9AuditEvents_FilterByBoth(t *testing.T) {
	db := setupL9TestDB(t)

	e1 := &L9AuditEvent{
		Kind:        "exchange",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-001",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e2 := &L9AuditEvent{
		Kind:        "commit",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-002",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e3 := &L9AuditEvent{
		Kind:        "exchange",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-003",
		EpisodeID:   "episode-2",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	assert.NoError(t, CreateL9AuditEvent(db, e1))
	assert.NoError(t, CreateL9AuditEvent(db, e2))
	assert.NoError(t, CreateL9AuditEvent(db, e3))

	resp, err := ListL9AuditEvents(db, "exchange", "episode-1", 0, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "exchange", resp.Data[0].Kind)
	assert.Equal(t, "episode-1", resp.Data[0].EpisodeID)
}

func TestListL9AuditEvents_WithPagination(t *testing.T) {
	db := setupL9TestDB(t)

	for i := 0; i < 5; i++ {
		e := &L9AuditEvent{
			Kind:        "knowledge",
			Protocol:    "sstp",
			Subprotocol: "ioc",
			MessageID:   "msg-" + string(rune('a'+i)),
			EpisodeID:   "ep-001",
			Actors:      []byte(`[]`),
			WorkspaceID: "ws-1",
			MASID:       "mas-1",
		}
		assert.NoError(t, CreateL9AuditEvent(db, e))
	}

	// page 0, pageSize 2
	resp, err := ListL9AuditEvents(db, "", "", 0, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 0, resp.PageInfo.Page)
	assert.Equal(t, 2, resp.PageInfo.PageSize)
	assert.Equal(t, 5, resp.PageInfo.TotalElements)

	// page 1, pageSize 2
	resp, err = ListL9AuditEvents(db, "", "", 1, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 1, resp.PageInfo.Page)

	// last page
	resp, err = ListL9AuditEvents(db, "", "", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, 2, resp.PageInfo.Page)
}

func TestGetL9AuditEventsByEpisode(t *testing.T) {
	db := setupL9TestDB(t)

	e1 := &L9AuditEvent{
		Kind:        "intent",
		Protocol:    "sstp",
		Subprotocol: "ioc",
		MessageID:   "msg-001",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e2 := &L9AuditEvent{
		Kind:        "exchange",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-002",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e3 := &L9AuditEvent{
		Kind:        "commit",
		Protocol:    "sstp",
		Subprotocol: "snp",
		MessageID:   "msg-003",
		EpisodeID:   "episode-1",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}
	e4 := &L9AuditEvent{
		Kind:        "intent",
		Protocol:    "sstp",
		Subprotocol: "ioc",
		MessageID:   "msg-004",
		EpisodeID:   "episode-2",
		Actors:      []byte(`[]`),
		Status:      "success",
		WorkspaceID: "ws-1",
		MASID:       "mas-1",
	}

	assert.NoError(t, CreateL9AuditEvent(db, e1))
	assert.NoError(t, CreateL9AuditEvent(db, e2))
	assert.NoError(t, CreateL9AuditEvent(db, e3))
	assert.NoError(t, CreateL9AuditEvent(db, e4))

	events, err := GetL9AuditEventsByEpisode(db, "episode-1")
	assert.NoError(t, err)
	assert.Len(t, events, 3)
	for _, e := range events {
		assert.Equal(t, "episode-1", e.EpisodeID)
	}
}

func TestNewL9AuditEventFromMessage(t *testing.T) {
	subkind := "query"
	msg := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "ioc",
			Kind:        l9.KindKnowledge,
			Subkind:     subkind,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "agent-1", Role: "sender"},
					{ID: "agent-2", Role: "receiver"},
				},
			},
			Message: &l9.L9HeaderMessage{
				ID:      "msg-123",
				Parents: []string{"msg-100", "msg-101"},
				Episode: "ep-456",
			},
			Context: &l9.L9HeaderContext{
				Topic: "semantic-alignment",
			},
		},
		Payload: l9.L9Payload{
			Type: "request",
		},
	}

	event := NewL9AuditEventFromMessage(msg, "ws-1", "mas-1")

	assert.NotNil(t, event)
	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, "ws-1", event.WorkspaceID)
	assert.Equal(t, "mas-1", event.MASID)
	assert.Equal(t, "knowledge", event.Kind)
	assert.Equal(t, "query", *event.Subkind)
	assert.Equal(t, "sstp", event.Protocol)
	assert.Equal(t, "ioc", event.Subprotocol)
	assert.Equal(t, "msg-123", event.MessageID)
	assert.Equal(t, "ep-456", event.EpisodeID)
	assert.Equal(t, "request", *event.PayloadType)
	assert.False(t, event.CreatedOn.IsZero())

	// Verify actors JSON
	var actors []L9Actor
	err := json.Unmarshal(event.Actors, &actors)
	assert.NoError(t, err)
	assert.Len(t, actors, 2)
	assert.Equal(t, "agent-1", actors[0].ID)
	assert.Equal(t, "sender", actors[0].Role)

	// Verify parents JSON
	var parents []string
	err = json.Unmarshal(event.ParentIDs, &parents)
	assert.NoError(t, err)
	assert.Equal(t, []string{"msg-100", "msg-101"}, parents)

	// Verify context JSON
	var context map[string]interface{}
	err = json.Unmarshal(event.Context, &context)
	assert.NoError(t, err)
	assert.Equal(t, "semantic-alignment", context["topic"])
}

func TestNewL9AuditEventFromMessage_MissingMessageID(t *testing.T) {
	// Message without message.id should return nil
	msg := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "tfp",
			Kind:        l9.KindExchange,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{},
			},
			Message: &l9.L9HeaderMessage{
				Episode: "ep-001",
				// ID is empty
			},
		},
		Payload: l9.L9Payload{},
	}

	event := NewL9AuditEventFromMessage(msg, "ws-1", "mas-1")
	assert.Nil(t, event)
}

func TestNewL9AuditEventFromMessage_MissingEpisode(t *testing.T) {
	// Message without message.episode should return nil
	msg := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "tfp",
			Kind:        l9.KindExchange,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{},
			},
			Message: &l9.L9HeaderMessage{
				ID: "msg-001",
				// Episode is empty
			},
		},
		Payload: l9.L9Payload{},
	}

	event := NewL9AuditEventFromMessage(msg, "ws-1", "mas-1")
	assert.Nil(t, event)
}

func TestNewL9AuditEventFromMessage_NoMessageHeader(t *testing.T) {
	// Message without message header should return nil
	msg := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "tfp",
			Kind:        l9.KindExchange,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{},
			},
			// Message is nil
		},
		Payload: l9.L9Payload{},
	}

	event := NewL9AuditEventFromMessage(msg, "ws-1", "mas-1")
	assert.Nil(t, event)
}

func TestL9AuditEvent_TableName(t *testing.T) {
	e := L9AuditEvent{}
	assert.Equal(t, "audit_l9", e.TableName())
}
