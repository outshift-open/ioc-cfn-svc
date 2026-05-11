package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
)

func TestMemoryOperationsHandler(t *testing.T) {
	// Save original ParsedConfig and restore after test
	originalConfig := ParsedConfig
	defer func() {
		cfnConfigMutex.Lock()
		ParsedConfig = originalConfig
		cfnConfigMutex.Unlock()
	}()

	// Create a mock memory provider server
	mockMemoryProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify custom header
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("expected X-Custom-Header to be 'test-value', got '%s'", r.Header.Get("X-Custom-Header"))
		}

		// Verify auth header was injected from config
		if r.Header.Get("Authorization") != "Token test-api-key" {
			t.Errorf("expected Authorization 'Token test-api-key', got '%s'", r.Header.Get("Authorization"))
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

	// Set up ParsedConfig to return the mock provider URL with token auth
	cfnConfigMutex.Lock()
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID: "ws-123",
				MultiAgenticSystems: []MASCfg{
					{
						ID: "mas-456",
						Agents: []AgentCfg{
							{
								AgentID: "agent-789",
								AgenticMemory: &MemoryCfg{
									Name:    "mem0",
									Enabled: true,
									Config: &MemConnConfig{
										URL: mockMemoryProvider.URL,
										Auth: &AuthConfig{
											Type: "token",
											Credentials: &AuthCreds{
												APIKey: "test-api-key",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	cfnConfigMutex.Unlock()

	// Create generic memory proxy client (same config as app.go)
	memoryCfg := httpclient.DefaultConfig()
	memoryCfg.Timeout = 5 * time.Minute
	memoryCfg.MaxRetries = 0
	memoryCfg.RetryableFunc = func(resp *http.Response, err error) bool { return false }
	memoryProxy := httpclient.NewWithConfig(memoryCfg)

	// Create app with the memory proxy client and mock database
	app := &App{memoryProxyClient: memoryProxy, db: client.NewMockDatabase()}

	// Create request payload (using relative path, handler will resolve full URL from config)
	requestPayload := memoryoperations.MemoryOperationRequest{
		Payload: memoryoperations.MemoryOperationPayload{
			HTTPRequestType: http.MethodPost,
			HTTPURL:         "/memories",
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
	// Save original ParsedConfig and restore after test
	originalConfig := ParsedConfig
	defer func() {
		cfnConfigMutex.Lock()
		ParsedConfig = originalConfig
		cfnConfigMutex.Unlock()
	}()

	tests := []struct {
		name           string
		payload        memoryoperations.MemoryOperationRequest
		setupConfig    func()
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing http-request-type",
			payload: memoryoperations.MemoryOperationRequest{
				Payload: memoryoperations.MemoryOperationPayload{
					HTTPURL: "/v1/memories",
				},
			},
			setupConfig:    func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "http-request-type is required",
		},
		{
			name: "config not found for agent",
			payload: memoryoperations.MemoryOperationRequest{
				Payload: memoryoperations.MemoryOperationPayload{
					HTTPRequestType: http.MethodGet,
				},
			},
			setupConfig: func() {
				cfnConfigMutex.Lock()
				ParsedConfig = &CfnConfigPayload{
					Workspaces: []WorkspaceConfig{},
				}
				cfnConfigMutex.Unlock()
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "failed to find memory provider config: workspace ws-123 not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up config for this test
			tt.setupConfig()

			app := &App{}
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

func TestGetMemoryProviderConfig(t *testing.T) {
	// Save original ParsedConfig and restore after test
	originalConfig := ParsedConfig
	defer func() {
		ParsedConfig = originalConfig
	}()

	tests := []struct {
		name             string
		config           *CfnConfigPayload
		workspaceID      string
		masID            string
		agentID          string
		expectedURL      string
		expectedProvider string
		expectedAuthType string
		expectError      bool
		errorMsg         string
	}{
		{
			name: "successful extraction with url and token auth",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "00000000-0000-0000-0000-000000000001",
					MultiAgenticSystems: []MASCfg{{
						ID: "e1271823-90ca-4581-86a4-f66a16ee154e",
						Agents: []AgentCfg{{
							AgentID: "default-agent-1",
							AgenticMemory: &MemoryCfg{
								Name: "mem0", Enabled: true,
								Config: &MemConnConfig{URL: "https://api.mem0.ai", Auth: &AuthConfig{Type: "token", Credentials: &AuthCreds{APIKey: "m0-xxx"}}},
							},
						}},
					}},
				}},
			},
			workspaceID:      "00000000-0000-0000-0000-000000000001",
			masID:            "e1271823-90ca-4581-86a4-f66a16ee154e",
			agentID:          "default-agent-1",
			expectedURL:      "https://api.mem0.ai",
			expectedProvider: "mem0",
			expectedAuthType: "token",
			expectError:      false,
		},
		{
			name: "url with no auth (self-hosted)",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID: "agent-1",
							AgenticMemory: &MemoryCfg{
								Name: "mem0", Enabled: true,
								Config: &MemConnConfig{URL: "http://ioc-mem0:8765", Auth: &AuthConfig{Type: "none"}},
							},
						}},
					}},
				}},
			},
			workspaceID:      "ws-1",
			masID:            "mas-1",
			agentID:          "agent-1",
			expectedURL:      "http://ioc-mem0:8765",
			expectedProvider: "mem0",
			expectedAuthType: "",
			expectError:      false,
		},
		{
			name: "url with no auth block at all",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID:       "agent-1",
							AgenticMemory: &MemoryCfg{Enabled: true, Config: &MemConnConfig{URL: "http://localhost:8765"}},
						}},
					}},
				}},
			},
			workspaceID:      "ws-1",
			masID:            "mas-1",
			agentID:          "agent-1",
			expectedURL:      "http://localhost:8765",
			expectedAuthType: "",
			expectError:      false,
		},
		{
			name: "multiple workspaces - find correct one",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{
					{ID: "ws-1", MultiAgenticSystems: []MASCfg{{ID: "mas-1", Agents: []AgentCfg{{AgentID: "agent-1", AgenticMemory: &MemoryCfg{Enabled: true, Config: &MemConnConfig{URL: "http://wrong-host:9999"}}}}}}},
					{ID: "ws-2", MultiAgenticSystems: []MASCfg{{ID: "mas-2", Agents: []AgentCfg{{AgentID: "agent-2", AgenticMemory: &MemoryCfg{Enabled: true, Config: &MemConnConfig{URL: "http://correct-host:7777"}}}}}}},
				},
			},
			workspaceID: "ws-2",
			masID:       "mas-2",
			agentID:     "agent-2",
			expectedURL: "http://correct-host:7777",
			expectError: false,
		},
		{
			name: "url with trailing slash stripped",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID:       "agent-1",
							AgenticMemory: &MemoryCfg{Enabled: true, Config: &MemConnConfig{URL: "https://api.mem0.ai/"}},
						}},
					}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectedURL: "https://api.mem0.ai",
			expectError: false,
		},
		{
			name: "bearer auth type",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID: "agent-1",
							AgenticMemory: &MemoryCfg{
								Name: "zep", Enabled: true,
								Config: &MemConnConfig{URL: "https://api.getzep.com", Auth: &AuthConfig{Type: "bearer", Credentials: &AuthCreds{AccessToken: "zep-xxx"}}},
							},
						}},
					}},
				}},
			},
			workspaceID:      "ws-1",
			masID:            "mas-1",
			agentID:          "agent-1",
			expectedURL:      "https://api.getzep.com",
			expectedProvider: "zep",
			expectedAuthType: "bearer",
			expectError:      false,
		},
		{
			name: "workspace not found",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{ID: "ws-1"}},
			},
			workspaceID: "non-existent-ws",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "workspace non-existent-ws not found",
		},
		{
			name: "mas not found",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID:                  "ws-1",
					MultiAgenticSystems: []MASCfg{{ID: "mas-1"}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "non-existent-mas",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "multi-agentic system non-existent-mas not found",
		},
		{
			name: "agent not found",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID:     "mas-1",
						Agents: []AgentCfg{{AgentID: "agent-1"}},
					}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "non-existent-agent",
			expectError: true,
			errorMsg:    "agent non-existent-agent not found",
		},
		{
			name: "memory disabled",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID:       "agent-1",
							AgenticMemory: &MemoryCfg{Enabled: false, Config: &MemConnConfig{URL: "http://ioc-mem0:8765"}},
						}},
					}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "agentic memory is disabled for this agent",
		},
		{
			name: "missing url in config",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID: "mas-1",
						Agents: []AgentCfg{{
							AgentID:       "agent-1",
							AgenticMemory: &MemoryCfg{Enabled: true, Config: &MemConnConfig{Auth: &AuthConfig{Type: "none"}}},
						}},
					}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "url not found in memory provider config",
		},
		{
			name: "missing agentic_memory",
			config: &CfnConfigPayload{
				Workspaces: []WorkspaceConfig{{
					ID: "ws-1",
					MultiAgenticSystems: []MASCfg{{
						ID:     "mas-1",
						Agents: []AgentCfg{{AgentID: "agent-1"}},
					}},
				}},
			},
			workspaceID: "ws-1",
			masID:       "mas-1",
			agentID:     "agent-1",
			expectError: true,
			errorMsg:    "agentic_memory not found for agent",
		},
		{
			name:        "no workspaces in config",
			config:      nil,
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
			ParsedConfig = tt.config
			cfnConfigMutex.Unlock()

			// Create app instance
			app := &App{}

			// Call function
			cfg, err := app.getMemoryProviderConfig(tt.workspaceID, tt.masID, tt.agentID)

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
				if cfg == nil {
					t.Fatal("expected config but got nil")
				}
				if cfg.baseURL != tt.expectedURL {
					t.Errorf("expected URL '%s', got '%s'", tt.expectedURL, cfg.baseURL)
				}
				if tt.expectedProvider != "" && cfg.providerName != tt.expectedProvider {
					t.Errorf("expected provider '%s', got '%s'", tt.expectedProvider, cfg.providerName)
				}
				if tt.expectedAuthType != "" {
					if cfg.auth == nil {
						t.Errorf("expected auth type '%s' but auth is nil", tt.expectedAuthType)
					} else if cfg.auth.authType != tt.expectedAuthType {
						t.Errorf("expected auth type '%s', got '%s'", tt.expectedAuthType, cfg.auth.authType)
					}
				}
				if tt.expectedAuthType == "" && cfg != nil && cfg.auth != nil {
					t.Errorf("expected no auth but got auth type '%s'", cfg.auth.authType)
				}
			}
		})
	}
}

func TestInjectAuthHeaders(t *testing.T) {
	tests := []struct {
		name           string
		auth           *memoryProviderAuth
		initialHeaders map[string]string
		expectedHeader string // expected Authorization header value
		expectedCustom map[string]string // for custom auth type
	}{
		{
			name: "token auth",
			auth: &memoryProviderAuth{
				authType: "token",
				apiKey:   "m0-test-key",
			},
			initialHeaders: map[string]string{"Content-Type": "application/json"},
			expectedHeader: "Token m0-test-key",
		},
		{
			name: "bearer auth",
			auth: &memoryProviderAuth{
				authType:    "bearer",
				accessToken: "zep-test-token",
			},
			initialHeaders: map[string]string{"Content-Type": "application/json"},
			expectedHeader: "Bearer zep-test-token",
		},
		{
			name: "basic auth",
			auth: &memoryProviderAuth{
				authType: "basic",
				username: "user",
				password: "pass",
			},
			initialHeaders: map[string]string{},
			expectedHeader: "Basic dXNlcjpwYXNz", // base64("user:pass")
		},
		{
			name: "custom auth",
			auth: &memoryProviderAuth{
				authType:    "custom",
				headerName:  "X-Api-Key",
				headerValue: "custom-key-123",
			},
			initialHeaders: map[string]string{},
			expectedCustom: map[string]string{"X-Api-Key": "custom-key-123"},
		},
		{
			name:           "nil auth",
			auth:           nil,
			initialHeaders: map[string]string{"Content-Type": "application/json"},
			expectedHeader: "",
		},
		{
			name: "none auth type",
			auth: &memoryProviderAuth{
				authType: "none",
			},
			initialHeaders: map[string]string{},
			expectedHeader: "",
		},
		{
			name: "strips user-provided Authorization header",
			auth: &memoryProviderAuth{
				authType: "token",
				apiKey:   "server-key",
			},
			initialHeaders: map[string]string{
				"Authorization": "Token user-injected-key",
			},
			expectedHeader: "Token server-key",
		},
		{
			name: "token with empty api_key skips auth",
			auth: &memoryProviderAuth{
				authType: "token",
				apiKey:   "",
			},
			initialHeaders: map[string]string{},
			expectedHeader: "",
		},
		{
			name: "bearer with empty access_token skips auth",
			auth: &memoryProviderAuth{
				authType:    "bearer",
				accessToken: "",
			},
			initialHeaders: map[string]string{},
			expectedHeader: "",
		},
		{
			name: "basic with empty password skips auth",
			auth: &memoryProviderAuth{
				authType: "basic",
				username: "user",
				password: "",
			},
			initialHeaders: map[string]string{},
			expectedHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(map[string]string)
			for k, v := range tt.initialHeaders {
				headers[k] = v
			}

			injectAuthHeaders(headers, tt.auth)

			if tt.expectedCustom != nil {
				for k, v := range tt.expectedCustom {
					if headers[k] != v {
						t.Errorf("expected header %s=%s, got %s", k, v, headers[k])
					}
				}
			} else {
				if tt.expectedHeader == "" {
					if _, exists := headers["Authorization"]; exists {
						t.Errorf("expected no Authorization header, got '%s'", headers["Authorization"])
					}
				} else {
					if headers["Authorization"] != tt.expectedHeader {
						t.Errorf("expected Authorization '%s', got '%s'", tt.expectedHeader, headers["Authorization"])
					}
				}
			}
		})
	}
}

func TestMemoryOperationsHandlerNoAuth(t *testing.T) {
	// Create a mock memory provider server that verifies NO Authorization header
	mockMemoryProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no Authorization header for no-auth provider, got '%s'", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	defer mockMemoryProvider.Close()

	// Set up ParsedConfig with auth type "none"
	setParsedConfigForTest(t, &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID: "ws-123",
				MultiAgenticSystems: []MASCfg{
					{
						ID: "mas-456",
						Agents: []AgentCfg{
							{
								AgentID: "agent-789",
								AgenticMemory: &MemoryCfg{
									Enabled: true,
									Config: &MemConnConfig{
										URL: mockMemoryProvider.URL,
										Auth: &AuthConfig{Type: "none"},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	memoryCfg := httpclient.DefaultConfig()
	memoryCfg.Timeout = 5 * time.Minute
	memoryCfg.MaxRetries = 0
	memoryCfg.RetryableFunc = func(resp *http.Response, err error) bool { return false }
	memoryProxy := httpclient.NewWithConfig(memoryCfg)

	app := &App{memoryProxyClient: memoryProxy, db: client.NewMockDatabase()}

	requestPayload := memoryoperations.MemoryOperationRequest{
		Payload: memoryoperations.MemoryOperationPayload{
			HTTPRequestType: http.MethodGet,
			HTTPURL:         "/v1/memories",
		},
	}

	requestBody, _ := json.Marshal(requestPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/ws-123/multi-agentic-systems/mas-456/agents/agent-789/memory-operations", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", "ws-123")
	req.SetPathValue("masId", "mas-456")
	req.SetPathValue("agentId", "agent-789")

	rr := httptest.NewRecorder()
	statusCode, err := app.memoryOperationsHandler(rr, req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	var response memoryoperations.MemoryOperationResponse
	json.NewDecoder(rr.Body).Decode(&response)

	if response.HTTPStatus != http.StatusOK {
		t.Errorf("expected HTTP status %d, got %d", http.StatusOK, response.HTTPStatus)
	}

	if response.HTTPResponseBody["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", response.HTTPResponseBody["status"])
	}
}

// TestGetMemoryProviderURL tests the convenience wrapper
func TestGetMemoryProviderURL(t *testing.T) {
	setParsedConfigForTest(t, &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{
			{
				ID: "ws-1",
				MultiAgenticSystems: []MASCfg{
					{
						ID: "mas-1",
						Agents: []AgentCfg{
							{
								AgentID: "agent-1",
								AgenticMemory: &MemoryCfg{
									Enabled: true,
									Config:  &MemConnConfig{URL: "https://api.mem0.ai"},
								},
							},
						},
					},
				},
			},
		},
	})

	app := &App{}
	url, err := app.getMemoryProviderURL("ws-1", "mas-1", "agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://api.mem0.ai" {
		t.Errorf("expected URL 'https://api.mem0.ai', got '%s'", url)
	}
}
