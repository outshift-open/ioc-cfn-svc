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
