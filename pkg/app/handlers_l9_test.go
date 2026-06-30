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

	// Create mock HTTP servers for CE endpoints
	mockKnowledgeCE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is properly formatted
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse the incoming L9 message
		var msg l9.L9
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("failed to decode L9 message: %v", err)
			http.Error(w, "invalid L9 message", http.StatusBadRequest)
			return
		}

		// Return a mock L9 response
		response := l9.L9{
			Header: l9.L9Header{
				Protocol:    "sstp",
				Version:     "1.0",
				Subprotocol: "ioc",
				Kind:        msg.Header.Kind,
				Subkind:     msg.Header.Subkind,
				Participants: msg.Header.Participants,
			},
			Payload: l9.L9Payload{
				Type: "response",
				Data: l9.L9PayloadData{
					"status":  "processed",
					"ce_name": "Knowledge CE",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockKnowledgeCE.Close()

	mockIntentCE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg l9.L9
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "invalid L9 message", http.StatusBadRequest)
			return
		}

		response := l9.L9{
			Header: l9.L9Header{
				Protocol:    "sstp",
				Version:     "1.0",
				Subprotocol: "ioc",
				Kind:        msg.Header.Kind,
				Participants: msg.Header.Participants,
			},
			Payload: l9.L9Payload{
				Type: "response",
				Data: l9.L9PayloadData{
					"status":  "processed",
					"ce_name": "Intent CE",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockIntentCE.Close()

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
				URL:     mockKnowledgeCE.URL,
			},
			{
				ID:      "ce-intent",
				Name:    "Intent Processing CE",
				Kind:    "intent",
				Enabled: true,
				URL:     mockIntentCE.URL,
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

			// For successful routing, verify we got a valid L9 response
			if tt.expectedStatus == http.StatusOK && tt.expectedCE != "" {
				var response l9.L9
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode L9 response: %v", err)
				}

				// Verify it's a valid L9 response
				if response.Header.Protocol != "sstp" {
					t.Errorf("expected L9 response with protocol 'sstp', got %s", response.Header.Protocol)
				}

				// Verify the response kind matches the request
				if response.Header.Kind != tt.l9Message.Header.Kind {
					t.Errorf("expected response kind %s, got %s", tt.l9Message.Header.Kind, response.Header.Kind)
				}

				t.Logf("✓ Message successfully routed to CE %s and received valid L9 response", tt.expectedCE)
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

	// Create a mock fallback CE server that will respond
	mockFallbackCE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock L9 response from fallback CE
		var msg l9.L9
		json.NewDecoder(r.Body).Decode(&msg)

		response := l9.L9{
			Header: l9.L9Header{
				Protocol:    "sstp",
				Version:     "1.0",
				Subprotocol: msg.Header.Subprotocol,
				Kind:        msg.Header.Kind,
				Subkind:     msg.Header.Subkind,
				Participants: msg.Header.Participants,
			},
			Payload: l9.L9Payload{
				Type: "response",
				Data: l9.L9PayloadData{
					"status":  "processed_by_fallback",
					"message": "handled by fallback CE",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockFallbackCE.Close()

	// Temporarily override defaultCEURL for this test
	originalDefaultCEURL := defaultCEURL
	defaultCEURL = mockFallbackCE.URL
	defer func() { defaultCEURL = originalDefaultCEURL }()

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

	// With fallback URL in place, the request should succeed
	if status != http.StatusOK {
		t.Errorf("expected status %d when using fallback CE, got %d", http.StatusOK, status)
	}

	t.Log("✓ No matching CE correctly uses fallback and returns 200")
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
