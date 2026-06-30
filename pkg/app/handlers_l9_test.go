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
	// Create app instance with mock config
	app := &App{}

	// Setup mock CFN config with CEs registered for different kinds
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID:            "test-workspace",
				WorkspaceName: "Test Workspace",
				MultiAgenticSystems: []MASCfg{
					{
						ID:          "test-mas",
						WorkspaceID: "test-workspace",
						Name:        "Test MAS",
						CognitionEngines: []MASEngineCfg{
							{ID: "ce-knowledge", Name: "Knowledge CE"},
							{ID: "ce-intent", Name: "Intent CE"},
						},
					},
				},
			},
		},
		CognitionEngines: []EngineCfg{
			{
				ID:      "ce-knowledge",
				Name:    "Knowledge Management CE",
				Kind:    "knowledge",
				Subkind: "query",
				Enabled: true,
				URL:     "http://ce-knowledge:9004",
			},
			{
				ID:      "ce-intent",
				Name:    "Intent Processing CE",
				Kind:    "intent",
				Enabled: true,
				URL:     "http://ce-intent:9005",
			},
		},
	}

	tests := []struct {
		name           string
		workspaceID    string
		masID          string
		l9Message      l9.L9
		expectedStatus int
		expectedCE     string // Expected CE ID to be routed to
	}{
		{
			name:        "valid knowledge/query message",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindKnowledge,
					Subkind:     "query",
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
						},
						Groups: &l9.ParticipantSetGroups{
							"workspace_id": "test-workspace",
							"mas_id":       "test-mas",
						},
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
			expectedCE:     "ce-knowledge",
		},
		{
			name:        "intent message routes to intent CE",
			workspaceID: "test-workspace",
			masID:       "test-mas",
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
						Groups: &l9.ParticipantSetGroups{
							"workspace_id": "test-workspace",
							"mas_id":       "test-mas",
						},
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
			expectedCE:     "ce-intent",
		},
		{
			name:        "missing kind returns 400",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
						},
						Groups: &l9.ParticipantSetGroups{
							"workspace_id": "test-workspace",
							"mas_id":       "test-mas",
						},
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "missing groups returns 400",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindKnowledge,
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
						},
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "workspace ID mismatch returns 400",
			workspaceID: "test-workspace",
			masID:       "test-mas",
			l9Message: l9.L9{
				Header: l9.L9Header{
					Protocol:    "sstp",
					Version:     "1.0",
					Subprotocol: "ioc",
					Kind:        l9.KindKnowledge,
					Participants: l9.ParticipantSet{
						Actors: []l9.Actor{
							{ID: "agent-1", Role: "sender"},
						},
						Groups: &l9.ParticipantSetGroups{
							"workspace_id": "wrong-workspace",
							"mas_id":       "test-mas",
						},
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal L9 message to JSON
			body, err := json.Marshal(tt.l9Message)
			if err != nil {
				t.Fatalf("failed to marshal L9 message: %v", err)
			}

			// Create request - workspace/MAS extracted from L9 message
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/l9/messages",
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
			if err != nil && tt.expectedStatus == http.StatusOK {
				t.Errorf("unexpected error: %v", err)
			}

			// For successful routing, verify the CE ID
			if tt.expectedStatus == http.StatusOK && tt.expectedCE != "" {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				routedCE, ok := response["routed_to_ce"].(string)
				if !ok {
					t.Error("response missing routed_to_ce field")
				} else if routedCE != tt.expectedCE {
					t.Errorf("expected routing to CE %s, got %s", tt.expectedCE, routedCE)
				}

				t.Logf("✓ Message routed to CE %s (%s)", routedCE, response["ce_name"])
			}
		})
	}
}

func TestL9Handler_InvalidJSON(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/l9/messages",
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

func TestL9Handler_NoMatchingCE(t *testing.T) {
	app := &App{}

	// Setup config with no CEs for "exchange" kind
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID: "test-workspace",
				MultiAgenticSystems: []MASCfg{
					{
						ID:               "test-mas",
						CognitionEngines: []MASEngineCfg{}, // Empty - no CEs
					},
				},
			},
		},
	}

	l9Msg := l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "tfp",
			Kind:        l9.KindExchange,
			Subkind:     "team-formation",
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "agent-1", Role: "sender"},
				},
				Groups: &l9.ParticipantSetGroups{
					"workspace_id": "test-workspace",
					"mas_id":       "test-mas",
				},
			},
		},
	}

	body, _ := json.Marshal(l9Msg)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/l9/messages",
		bytes.NewReader(body),
	)

	w := httptest.NewRecorder()
	status, _ := app.l9Handler(w, req)

	if status != http.StatusNotFound {
		t.Errorf("expected status %d when no CE matches, got %d", http.StatusNotFound, status)
	}

	t.Log("✓ No matching CE correctly returns 404")
}

func TestL9Handler_MultipleCEs(t *testing.T) {
	app := &App{}

	// Setup config with MULTIPLE CEs for the same kind
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID: "test-workspace",
				MultiAgenticSystems: []MASCfg{
					{
						ID: "test-mas",
						CognitionEngines: []MASEngineCfg{
							{ID: "ce-knowledge-1"},
							{ID: "ce-knowledge-2"},
						},
					},
				},
			},
		},
		CognitionEngines: []EngineCfg{
			{ID: "ce-knowledge-1", Kind: "knowledge", Enabled: true},
			{ID: "ce-knowledge-2", Kind: "knowledge", Enabled: true},
		},
	}

	l9Msg := l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Version:     "1.0",
			Subprotocol: "ioc",
			Kind:        l9.KindKnowledge,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: "agent-1", Role: "sender"},
				},
				Groups: &l9.ParticipantSetGroups{
					"workspace_id": "test-workspace",
					"mas_id":       "test-mas",
				},
			},
		},
	}

	body, _ := json.Marshal(l9Msg)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/l9/messages",
		bytes.NewReader(body),
	)

	w := httptest.NewRecorder()
	status, _ := app.l9Handler(w, req)

	if status != http.StatusInternalServerError {
		t.Errorf("expected status %d when multiple CEs match, got %d", http.StatusInternalServerError, status)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] == "" || response["error"] == "{}" {
		t.Error("expected error message about multiple CEs")
	}

	t.Logf("✓ Multiple matching CEs correctly returns 500: %s", response["error"])
}
