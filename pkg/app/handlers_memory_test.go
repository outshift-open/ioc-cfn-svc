package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	mem0client "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/mem0"
)

func TestMemoryOperationsHandler(t *testing.T) {
	// Create a mock memory provider server
	mockMemoryProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify headers
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("expected X-Custom-Header to be 'test-value', got '%s'", r.Header.Get("X-Custom-Header"))
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify request body
		if reqBody["test-field"] != "test-data" {
			t.Errorf("expected test-field to be 'test-data', got '%v'", reqBody["test-field"])
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Response-Header", "response-value")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"message":    "memory created",
			"memory-id":  "123",
		})
	}))
	defer mockMemoryProvider.Close()

	// Create mem0 client pointing at the mock server
	mem0Cfg := mem0client.DefaultClientConfig()
	mem0Cfg.BaseURL = mockMemoryProvider.URL
	mem0Cfg.APIKey = "test-api-key" // test-only value, not a real credential
	mem0, err := mem0client.NewClient(mem0Cfg)
	if err != nil {
		t.Fatalf("failed to create mem0 client: %v", err)
	}

	// Create app with the mem0 client
	app := &App{mem0Client: mem0}

	// Create request payload
	requestPayload := memoryoperations.MemoryOperationRequest{
		Payload: memoryoperations.MemoryOperationPayload{
			HTTPRequestType: http.MethodPost,
			HTTPURL:         mockMemoryProvider.URL + "/memories",
			HTTPRequestBody: map[string]interface{}{
				"test-field": "test-data",
			},
			HTTPHeaders: map[string]string{
				"X-Custom-Header": "test-value",
			},
		},
	}

	// Marshal request payload
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(http.MethodPost, "/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Set path parameters (simulating router behavior)
	req.SetPathValue("workspaceId", "ws-123")
	req.SetPathValue("masId", "mas-456")
	req.SetPathValue("agentId", "agent-789")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	statusCode, err := app.memoryOperationsHandler(rr, req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Check status code
	if statusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	// Parse response body
	var response memoryoperations.MemoryOperationResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check HTTP status from memory provider
	if response.HTTPStatus != http.StatusCreated {
		t.Errorf("expected HTTP status %d, got %d", http.StatusCreated, response.HTTPStatus)
	}

	// Check response headers
	if response.HTTPHeaders["X-Response-Header"] != "response-value" {
		t.Errorf("expected X-Response-Header to be 'response-value', got '%s'", response.HTTPHeaders["X-Response-Header"])
	}

	// Check response body
	if response.HTTPResponseBody["status"] != "success" {
		t.Errorf("expected status to be 'success', got '%v'", response.HTTPResponseBody["status"])
	}

	if response.HTTPResponseBody["memory-id"] != "123" {
		t.Errorf("expected memory-id to be '123', got '%v'", response.HTTPResponseBody["memory-id"])
	}
}

func TestMemoryOperationsHandlerValidation(t *testing.T) {
	app := &App{}

	tests := []struct {
		name           string
		payload        memoryoperations.MemoryOperationRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing http-request-type",
			payload: memoryoperations.MemoryOperationRequest{
				Payload: memoryoperations.MemoryOperationPayload{
					HTTPURL: "http://example.com",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "http-request-type is required",
		},
		{
			name: "missing http-url",
			payload: memoryoperations.MemoryOperationRequest{
				Payload: memoryoperations.MemoryOperationPayload{
					HTTPRequestType: http.MethodGet,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "http-url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody, _ := json.Marshal(tt.payload)
			req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("workspaceId", "ws-123")
			req.SetPathValue("masId", "mas-456")
			req.SetPathValue("agentId", "agent-789")

			rr := httptest.NewRecorder()
			statusCode, _ := app.memoryOperationsHandler(rr, req)

			if statusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, statusCode)
			}

			var errResp map[string]string
			json.NewDecoder(rr.Body).Decode(&errResp)
			if errResp["error"] != tt.expectedError {
				t.Errorf("expected error '%s', got '%s'", tt.expectedError, errResp["error"])
			}
		})
	}
}

func TestGetMemoryProviderURL(t *testing.T) {
	// Save original CfnConfig and restore after test
	originalConfig := CfnConfig
	defer func() {
		CfnConfig = originalConfig
	}()

	tests := []struct {
		name        string
		config      map[string]any
		workspaceID string
		masID       string
		agentID     string
		expectedURL string
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful extraction with valid config",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "00000000-0000-0000-0000-000000000001",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "e1271823-90ca-4581-86a4-f66a16ee154e",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "default-agent-1",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"host": "ioc-mem0",
												"port": float64(8765),
											},
											"enabled": true,
										},
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "00000000-0000-0000-0000-000000000001",
			masID:       "e1271823-90ca-4581-86a4-f66a16ee154e",
			agentID:     "default-agent-1",
			expectedURL: "http://ioc-mem0:8765",
			expectError: false,
		},
		{
			name: "multiple workspaces - find correct one",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"host": "wrong-host",
												"port": float64(9999),
											},
											"enabled": true,
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"workspace_id": "ws-2",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-2",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-2",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"host": "correct-host",
												"port": float64(7777),
											},
											"enabled": true,
										},
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-2",
			masID:       "mas-2",
			agentID:     "agent-2",
			expectedURL: "http://correct-host:7777",
			expectError: false,
		},
		{
			name: "workspace not found",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
					},
				},
			},
			workspaceID: "non-existent-ws",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "workspace non-existent-ws not found",
		},
		{
			name: "mas not found",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "non-existent-mas",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "multi-agentic system non-existent-mas not found",
		},
		{
			name: "agent not found",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "non-existent-agent",
			expectError: true,
			errorMsg:    "agent non-existent-agent not found",
		},
		{
			name: "memory disabled",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"host": "ioc-mem0",
												"port": float64(8765),
											},
											"enabled": false,
										},
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "agentic memory is disabled for this agent",
		},
		{
			name: "missing host in config",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"port": float64(8765),
											},
											"enabled": true,
										},
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "host or port not found in memory provider config",
		},
		{
			name: "missing port in config",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
										"agentic_memory": map[string]interface{}{
											"config": map[string]interface{}{
												"host": "ioc-mem0",
											},
											"enabled": true,
										},
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "host or port not found in memory provider config",
		},
		{
			name: "missing agentic_memory",
			config: map[string]any{
				"workspaces": []interface{}{
					map[string]interface{}{
						"workspace_id": "ws-1",
						"multi_agentic_systems": []interface{}{
							map[string]interface{}{
								"id": "mas-1",
								"agents": []interface{}{
									map[string]interface{}{
										"agent_id": "agent-1",
									},
								},
							},
						},
					},
				},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "agentic_memory not found for agent",
		},
		{
			name: "no workspaces in config",
			config: map[string]any{
				"other_field": "value",
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "workspaces not found in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test config
			cfnConfigMutex.Lock()
			CfnConfig = tt.config
			cfnConfigMutex.Unlock()

			// Create app instance
			app := &App{}

			// Call function
			url, err := app.getMemoryProviderURL(tt.workspaceID, tt.masID, tt.agentID)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if url != tt.expectedURL {
					t.Errorf("expected URL '%s', got '%s'", tt.expectedURL, url)
				}
			}
		})
	}
}
