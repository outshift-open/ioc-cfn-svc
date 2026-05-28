package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/cognitionengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterCognitionEngineHandler(t *testing.T) {
	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/cognition-engines/register", r.URL.Path)

		// Parse the forwarded request
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify CFN context was added
		assert.Equal(t, "test-cfn-id", payload["cfn_id"])
		assert.Equal(t, "Knowledge Management CE", payload["name"])
		assert.Equal(t, "knowledge_management", payload["type"])
		assert.Equal(t, "http://ce-host:9004", payload["url"])

		// Return success response
		resp := cognitionengine.RegisterResponse{
			CEID:    "ce-123",
			CFNID:   "test-cfn-id",
			Message: "Cognition Engine registered successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer mgmtServer.Close()

	// Set environment variable to point to mock server
	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	// Set mock CFN ID
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Create test app
	app := &App{}

	// Create test request
	reqBody := cognitionengine.RegisterRequest{
		Name:         "Knowledge Management CE",
		Type:         "knowledge_management",
		URL:          "http://ce-host:9004",
		Capabilities: []string{"ingestion", "retrieval"},
		Metrics:      []string{"kb.documents.indexed", "kb.search.latency_ms"},
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines/register", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler
	_, err = app.registerCognitionEngineHandler(w, req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp cognitionengine.RegisterResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ce-123", resp.CEID)
	assert.Equal(t, "test-cfn-id", resp.CFNID)
	assert.Equal(t, "Cognition Engine registered successfully", resp.Message)
}

func TestRegisterCognitionEngineHandler_MissingCFNID(t *testing.T) {
	// Clear CFN ID to simulate unregistered CFN
	originalCfnID := CfnID
	CfnID = ""
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	reqBody := cognitionengine.RegisterRequest{
		Name: "Knowledge Management CE",
		Type: "knowledge_management",
		URL:  "http://ce-host:9004",
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines/register", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	_, err = app.registerCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "CFN not registered")
}

func TestRegisterCognitionEngineHandler_InvalidRequest(t *testing.T) {
	app := &App{}

	// Set mock CFN ID
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	tests := []struct {
		name        string
		reqBody     cognitionengine.RegisterRequest
		expectedErr string
	}{
		{
			name:        "missing name",
			reqBody:     cognitionengine.RegisterRequest{Type: "knowledge_management", URL: "http://ce-host:9004"},
			expectedErr: "name is required",
		},
		{
			name:        "missing type",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", URL: "http://ce-host:9004"},
			expectedErr: "type is required",
		},
		{
			name:        "missing url",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Type: "knowledge_management"},
			expectedErr: "url is required",
		},
		{
			name:        "invalid url format",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Type: "knowledge_management", URL: "not-a-valid-url:::"},
			expectedErr: "invalid url format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBytes, err := json.Marshal(tt.reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines/register", bytes.NewReader(reqBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			_, err = app.registerCognitionEngineHandler(w, req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp map[string]string
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Contains(t, resp["error"], tt.expectedErr)
		})
	}
}

func TestRegisterCognitionEngineHandler_ManagementPlaneError(t *testing.T) {
	// Setup mock management plane server that returns an error
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "CE with this name already exists",
		})
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	reqBody := cognitionengine.RegisterRequest{
		Name: "Knowledge Management CE",
		Type: "knowledge_management",
		URL:  "http://ce-host:9004",
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines/register", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	_, err = app.registerCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "already exists")
}

func TestCognitionEngineHeartbeatHandler(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/cognition-engines/"+testCEID+"/heartbeat", r.URL.Path)

		// Return success response
		resp := cognitionengine.HeartbeatResponse{
			Status:   "online",
			LastSeen: "2026-05-21T10:30:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer mgmtServer.Close()

	// Set environment variable to point to mock server
	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	// Set mock CFN ID
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Create test app
	app := &App{}

	// Create test request (PUT with no body for heartbeat)
	req := httptest.NewRequest(http.MethodPut, "/api/cognition-engines/"+testCEID+"/heartbeat", nil)
	req.Header.Set("Accept", "application/json")
	req.SetPathValue("ceId", testCEID)

	w := httptest.NewRecorder()

	// Call the handler
	_, err := app.cognitionEngineHeartbeatHandler(w, req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp cognitionengine.HeartbeatResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "online", resp.Status)
	assert.Equal(t, "2026-05-21T10:30:00Z", resp.LastSeen)
}

func TestCognitionEngineHeartbeatHandler_MissingCFNID(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Clear CFN ID to simulate unregistered CFN
	originalCfnID := CfnID
	CfnID = ""
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodPut, "/api/cognition-engines/"+testCEID+"/heartbeat", nil)
	req.Header.Set("Accept", "application/json")
	req.SetPathValue("ceId", testCEID)

	w := httptest.NewRecorder()

	_, err := app.cognitionEngineHeartbeatHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "CFN not registered")
}

func TestCognitionEngineHeartbeatHandler_CENotFound(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440001"

	// Setup mock management plane server that returns 404
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Cognition Engine not found",
		})
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodPut, "/api/cognition-engines/"+testCEID+"/heartbeat", nil)
	req.Header.Set("Accept", "application/json")
	req.SetPathValue("ceId", testCEID)

	w := httptest.NewRecorder()

	_, err := app.cognitionEngineHeartbeatHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "not found")
}

func TestCognitionEngineHeartbeatHandler_InvalidCEID(t *testing.T) {
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	tests := []struct {
		name   string
		ceID   string
		errMsg string
	}{
		{
			name:   "invalid uuid format",
			ceID:   "not-a-uuid",
			errMsg: "invalid ce_id format",
		},
		{
			name:   "empty string uuid",
			ceID:   "",
			errMsg: "ce_id is required",
		},
		{
			name:   "malformed uuid",
			ceID:   "12345",
			errMsg: "invalid ce_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/cognition-engines/"+tt.ceID+"/heartbeat", nil)
			req.Header.Set("Accept", "application/json")
			req.SetPathValue("ceId", tt.ceID)

			w := httptest.NewRecorder()

			_, err := app.cognitionEngineHeartbeatHandler(w, req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp map[string]string
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Contains(t, resp["error"], tt.errMsg)
		})
	}
}
