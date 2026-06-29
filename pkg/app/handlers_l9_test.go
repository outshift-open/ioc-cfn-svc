package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

func TestL9Handler(t *testing.T) {
	// Create app instance
	app := &App{}

	tests := []struct {
		name           string
		workspaceID    string
		masID          string
		ceID           string
		l9Message      l9.L9
		expectedStatus int
	}{
		{
			name:        "valid knowledge message",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			ceID:        "test-ce",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindKnowledge,
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
							{ID: "ce-knowledge", Role: "receiver"},
						},
						Groups: &l9.ParticipantSetGroups{},
					},
				},
				Payload: l9.L9Payload{
					Type: "query",
					Data: l9.L9PayloadData{
						"question": "What is AI?",
						"context":  "technical",
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "intent message",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			ceID:        "test-ce",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindIntent,
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-main", Role: "sender"},
						},
						Groups: &l9.ParticipantSetGroups{},
					},
				},
				Payload: l9.L9Payload{
					Type: "goal",
					Data: l9.L9PayloadData{
						"goal": "Find documents about quantum computing",
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "exchange message with subkind",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			ceID:        "test-ce",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindExchange,
					Subkind:     "data-transfer",
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
							{ID: "ce-processor", Role: "receiver"},
						},
						Groups: &l9.ParticipantSetGroups{},
					},
				},
				Payload: l9.L9Payload{
					Type: "data",
					Data: l9.L9PayloadData{
						"content": "Sample data payload",
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal L9 message to JSON
			body, err := json.Marshal(tt.l9Message)
			if err != nil {
				t.Fatalf("failed to marshal L9 message: %v", err)
			}

			// Create request
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/workspaces/"+tt.workspaceID+"/multi-agentic-systems/"+tt.masID+"/cognition-engines/"+tt.ceID+"/l9",
				bytes.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler directly
			status, err := app.l9Handler(w, req)

			// Check status
			if status != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, status)
			}

			// Check error
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Parse response
			var response l9.L9
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			// Verify response echoes back the message
			if response.Header.Kind != tt.l9Message.Header.Kind {
				t.Errorf("expected kind %s, got %s", tt.l9Message.Header.Kind, response.Header.Kind)
			}

			if response.Header.Protocol != tt.l9Message.Header.Protocol {
				t.Errorf("expected protocol %s, got %s", tt.l9Message.Header.Protocol, response.Header.Protocol)
			}

			t.Logf("✓ Response echoed back correctly - kind=%s, protocol=%s, version=%s",
				response.Header.Kind,
				response.Header.Protocol,
				response.Header.Version)
		})
	}
}

func TestL9Handler_InvalidJSON(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/test-ws/multi-agentic-systems/test-mas/cognition-engines/test-ce/l9",
		bytes.NewReader([]byte("invalid json")),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	status, _ := app.l9Handler(w, req)

	if status != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, status)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if _, ok := response["error"]; !ok {
		t.Error("expected error field in response")
	}

	t.Logf("✓ Invalid JSON correctly rejected with error: %s", response["error"])
}

func TestL9Handler_MissingFields(t *testing.T) {
	app := &App{}

	// L9 message missing required fields
	invalidMsg := map[string]interface{}{
		"header": map[string]interface{}{
			"protocol": "sstp",
			// Missing required fields: version, subprotocol, kind, participants
		},
	}

	body, _ := json.Marshal(invalidMsg)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/test-ws/multi-agentic-systems/test-mas/cognition-engines/test-ce/l9",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	status, _ := app.l9Handler(w, req)

	if status != http.StatusBadRequest {
		t.Errorf("expected status %d for missing fields, got %d", http.StatusBadRequest, status)
	}

	t.Log("✓ Missing required fields correctly rejected")
}
