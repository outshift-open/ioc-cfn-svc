// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	l9cache "github.com/outshift-open/ioc-cfn-svc/pkg/cache/l9"
	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

// TestL9CacheHandlers_ResponseConsistency verifies that both getConversationHandler
// and getPreviousNHandler return consistent response payloads with session_id and
// all participants from the entire conversation.
func TestL9CacheHandlers_ResponseConsistency(t *testing.T) {
	app := &App{}

	// Create test messages for a conversation
	// Root message (session start)
	msg1 := &l9.L9{
		Header: l9.L9Header{
			Message: &l9.L9HeaderMessage{
				ID:      "msg-1",
				Parents: []string{},
			},
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "user-1", Role: "user"},
					{ID: "agent-1", Role: "assistant"},
				},
			},
		},
	}

	// Second message with additional participant
	msg2 := &l9.L9{
		Header: l9.L9Header{
			Message: &l9.L9HeaderMessage{
				ID:      "msg-2",
				Parents: []string{"msg-1"},
			},
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "user-1", Role: "user"},
					{ID: "agent-1", Role: "assistant"},
					{ID: "agent-2", Role: "assistant"}, // New participant in msg-2
				},
			},
		},
	}

	// Third message
	msg3 := &l9.L9{
		Header: l9.L9Header{
			Message: &l9.L9HeaderMessage{
				ID:      "msg-3",
				Parents: []string{"msg-2"},
			},
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "user-1", Role: "user"},
					{ID: "agent-2", Role: "assistant"},
				},
			},
		},
	}

	// Fourth message
	msg4 := &l9.L9{
		Header: l9.L9Header{
			Message: &l9.L9HeaderMessage{
				ID:      "msg-4",
				Parents: []string{"msg-3"},
			},
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "user-1", Role: "user"},
					{ID: "agent-1", Role: "assistant"},
				},
			},
		},
	}

	// Create cache and add messages
	cache := l9cache.New()
	cache.Add(msg1)
	cache.Add(msg2)
	cache.Add(msg3)
	cache.Add(msg4)

	// Test Case 1: getConversationHandler - should return all 4 messages
	t.Run("getConversationHandler returns full conversation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?msgID=msg-4", nil)
		w := httptest.NewRecorder()

		status, err := app.getConversationHandler(w, req, cache, "test-ws", "test-mas", "msg-4")
		if err != nil {
			t.Fatalf("getConversationHandler failed: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d", status)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify response structure
		if response["workspace_id"] != "test-ws" {
			t.Errorf("expected workspace_id=test-ws, got %v", response["workspace_id"])
		}
		if response["mas_id"] != "test-mas" {
			t.Errorf("expected mas_id=test-mas, got %v", response["mas_id"])
		}
		if response["session_id"] != "msg-1" {
			t.Errorf("expected session_id=msg-1 (root message), got %v", response["session_id"])
		}

		messages := response["messages"].([]interface{})
		if len(messages) != 4 {
			t.Errorf("expected 4 messages, got %d", len(messages))
		}

		participants := response["participants"].([]interface{})
		if len(participants) != 3 { // user-1, agent-1, agent-2
			t.Errorf("expected 3 unique participants, got %d", len(participants))
		}
	})

	// Test Case 2: getPreviousNHandler - should return last N messages BUT all participants
	t.Run("getPreviousNHandler returns consistent session_id and all participants", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?msgID=msg-4&n=2", nil)
		w := httptest.NewRecorder()

		status, err := app.getPreviousNHandler(w, req, cache, "test-ws", "test-mas", "msg-4", 2)
		if err != nil {
			t.Fatalf("getPreviousNHandler failed: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("expected status 200, got %d", status)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify response structure
		if response["workspace_id"] != "test-ws" {
			t.Errorf("expected workspace_id=test-ws, got %v", response["workspace_id"])
		}
		if response["mas_id"] != "test-mas" {
			t.Errorf("expected mas_id=test-mas, got %v", response["mas_id"])
		}
		if response["session_id"] != "msg-1" {
			t.Errorf("expected session_id=msg-1 (root message), got %v", response["session_id"])
		}

		// Should return only 2 messages (msg-3, msg-4)
		messages := response["messages"].([]interface{})
		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}

		// BUT should return ALL 3 participants from the entire conversation
		participants := response["participants"].([]interface{})
		if len(participants) != 3 { // user-1, agent-1, agent-2 from entire conversation
			t.Errorf("expected 3 unique participants from entire conversation, got %d", len(participants))
		}
	})

	// Test Case 3: Both handlers should have same session_id for same message
	t.Run("both handlers return same session_id", func(t *testing.T) {
		// Get full conversation
		req1 := httptest.NewRequest(http.MethodGet, "/test?msgID=msg-2", nil)
		w1 := httptest.NewRecorder()
		app.getConversationHandler(w1, req1, cache, "test-ws", "test-mas", "msg-2")

		var resp1 map[string]interface{}
		json.NewDecoder(w1.Body).Decode(&resp1)

		// Get previous N
		req2 := httptest.NewRequest(http.MethodGet, "/test?msgID=msg-2&n=1", nil)
		w2 := httptest.NewRecorder()
		app.getPreviousNHandler(w2, req2, cache, "test-ws", "test-mas", "msg-2", 1)

		var resp2 map[string]interface{}
		json.NewDecoder(w2.Body).Decode(&resp2)

		// Both should have same session_id
		if resp1["session_id"] != resp2["session_id"] {
			t.Errorf("session_id mismatch: full conversation=%v, previous N=%v",
				resp1["session_id"], resp2["session_id"])
		}

		// Both should have same participants count
		p1 := resp1["participants"].([]interface{})
		p2 := resp2["participants"].([]interface{})
		if len(p1) != len(p2) {
			t.Errorf("participants count mismatch: full conversation=%d, previous N=%d",
				len(p1), len(p2))
		}
	})
}
