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

// setupCETestConfig sets up ParsedConfig with a test CE for testing CE operations
func setupCETestConfig(ceID string) func() {
	originalParsedConfig := ParsedConfig
	ParsedConfig = &CfnConfigPayload{
		CognitionEngines: []EngineCfg{
			{
				ID:      ceID,
				Name:    "Test CE",
				URL:     "http://test-ce:9004",
				Enabled: true,
			},
		},
	}
	return func() { ParsedConfig = originalParsedConfig }
}

func TestRegisterCognitionEngineHandler(t *testing.T) {
	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/cognition-engines", r.URL.Path)

		// Parse the forwarded request
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify CFN context was added
		assert.Equal(t, "test-cfn-id", payload["cfn_id"])
		assert.Equal(t, "Knowledge Management CE", payload["name"])
		assert.Equal(t, "knowledge", payload["kind"])
		assert.Equal(t, "query", payload["subkind"])
		assert.Equal(t, "http://ce-host:9004", payload["url"])
		assert.Equal(t, "1.0.0", payload["version"])

		// Return success response
		resp := cognitionengine.RegisterResponse{
			CEID:             "ce-123",
			CFNID:            "test-cfn-id",
			Name:             "Knowledge Management CE",
			Version:          "1.0.0",
			Kind:             "knowledge",
			Subkind:          "query",
			Enabled:          true,
			MASAutoAssociate: false,
			Status:           "online",
			Created:          true,
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
		Kind:         "knowledge",
		Subkind:      "query",
		URL:          "http://ce-host:9004",
		Version:      "1.0.0",
		Capabilities: []string{"ingestion", "retrieval"},
		Metrics:      []string{"kb.documents.indexed", "kb.search.latency_ms"},
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines", bytes.NewReader(reqBytes))
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
	assert.Equal(t, "Knowledge Management CE", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "knowledge", resp.Kind)
	assert.Equal(t, "query", resp.Subkind)
	assert.True(t, resp.Enabled)
	assert.False(t, resp.MASAutoAssociate)
	assert.Equal(t, "online", resp.Status)
	assert.True(t, resp.Created)
}

func TestRegisterCognitionEngineHandler_MissingCFNID(t *testing.T) {
	// Clear CFN ID to simulate unregistered CFN
	originalCfnID := CfnID
	CfnID = ""
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	reqBody := cognitionengine.RegisterRequest{
		Name:    "Knowledge Management CE",
		Kind:    "knowledge",
		Subkind: "query",
		URL:     "http://ce-host:9004",
		Version: "1.0.0",
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines", bytes.NewReader(reqBytes))
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
			reqBody:     cognitionengine.RegisterRequest{Kind: "knowledge", Subkind: "query", URL: "http://ce-host:9004", Version: "1.0.0"},
			expectedErr: "name is required",
		},
		{
			name:        "missing kind",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Subkind: "query", URL: "http://ce-host:9004", Version: "1.0.0"},
			expectedErr: "kind is required",
		},
		{
			name:        "missing url",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Kind: "knowledge", Subkind: "query", Version: "1.0.0"},
			expectedErr: "url is required",
		},
		{
			name:        "missing version",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Kind: "knowledge", Subkind: "query", URL: "http://ce-host:9004"},
			expectedErr: "version is required",
		},
		{
			name:        "invalid url format",
			reqBody:     cognitionengine.RegisterRequest{Name: "CE", Kind: "knowledge", Subkind: "query", URL: "not-a-valid-url:::", Version: "1.0.0"},
			expectedErr: "invalid url format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBytes, err := json.Marshal(tt.reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines", bytes.NewReader(reqBytes))
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
	// Setup mock management plane server that returns a 404 (CFN not found)
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "CFN not found",
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
		Name:    "Knowledge Management CE",
		Kind:    "knowledge",
		Subkind: "query",
		URL:     "http://ce-host:9004",
		Version: "1.0.0",
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/cognition-engines", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	_, err = app.registerCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "not found")
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

	// Set up ParsedConfig with CE data
	cleanup := setupCETestConfig("550e8400-e29b-41d4-a716-446655440000")
	defer cleanup()

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

	// Set up ParsedConfig with CE data
	cleanup := setupCETestConfig("550e8400-e29b-41d4-a716-446655440000")
	defer cleanup()

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

	// Set up ParsedConfig with CE data
	cleanup := setupCETestConfig("550e8400-e29b-41d4-a716-446655440001")
	defer cleanup()

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

func TestListCognitionEnginesHandler(t *testing.T) {
	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/cognition-engines", r.URL.Path)

		// Return list response
		resp := cognitionengine.CognitionEngineList{
			CognitionEngines: []cognitionengine.CognitionEngineListItem{
				{
					ID:               "ce-123",
					CFNID:            "test-cfn-id",
					Name:             "Test CE",
					Version:          "1.0.0",
					Kind:             "knowledge",
					Subkind:          "query",
					URL:              "http://ce-host:9004",
					Enabled:          true,
					MASAutoAssociate: false,
					Status:           "online",
				},
			},
			Total: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodGet, "/api/cognition-engines", nil)
	w := httptest.NewRecorder()

	_, err := app.listCognitionEnginesHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp cognitionengine.CognitionEngineList
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.CognitionEngines, 1)
	assert.Equal(t, "ce-123", resp.CognitionEngines[0].ID)
}

func TestGetCognitionEngineHandler(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/cognition-engines/"+testCEID, r.URL.Path)

		// Return detail response
		resp := cognitionengine.CognitionEngineDetail{
			ID:               testCEID,
			CFNID:            "test-cfn-id",
			Name:             "Test CE",
			Version:          "1.0.0",
			Kind:             "knowledge",
			Subkind:          "query",
			URL:              "http://ce-host:9004",
			Enabled:          true,
			MASAutoAssociate: false,
			Status:           "online",
			Capabilities:     []string{"ingestion"},
			CreatedAt:        "2026-05-21T09:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Set up ParsedConfig with CE data
	cleanup := setupCETestConfig(testCEID)
	defer cleanup()

	app := &App{}

	req := httptest.NewRequest(http.MethodGet, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.getCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp cognitionengine.CognitionEngineDetail
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, testCEID, resp.ID)
	assert.Equal(t, "Test CE", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
}

func TestDeleteCognitionEngineHandler(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle RefreshConfig GET request
		if r.Method == http.MethodGet && r.URL.Path == "/api/cognition-fabric-nodes/test-cfn-id" {
			// Return config with CE disabled and no MAS associations to allow delete to proceed
			config := CfnConfigPayload{
				Workspaces: []WorkspaceConfig{},
				CognitionEngines: []EngineCfg{
					{
						ID:      testCEID,
						Name:    "Test CE",
						URL:     "http://ce-host:9004",
						Enabled: false, // Must be disabled to allow delete
					},
				},
			}
			resp := map[string]interface{}{
				"config": config,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Handle DELETE request
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/cognition-engines/"+testCEID, r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Set up initial ParsedConfig to avoid nil during CE existence check
	cfnConfigMutex.Lock()
	oldConfig := ParsedConfig
	ParsedConfig = &CfnConfigPayload{
		CognitionEngines: []EngineCfg{
			{ID: testCEID, Name: "Test CE", Enabled: false},
		},
	}
	cfnConfigMutex.Unlock()
	defer func() {
		cfnConfigMutex.Lock()
		ParsedConfig = oldConfig
		cfnConfigMutex.Unlock()
	}()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.deleteCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteCognitionEngineHandler_EnabledCE(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server for RefreshConfig
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/cognition-fabric-nodes/test-cfn-id" {
			config := CfnConfigPayload{
				Workspaces: []WorkspaceConfig{},
				CognitionEngines: []EngineCfg{
					{
						ID:      testCEID,
						Name:    "Test CE",
						URL:     "http://ce-host:9004",
						Enabled: true, // CE is enabled - should block delete
					},
				},
			}
			resp := map[string]interface{}{
				"config": config,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.deleteCognitionEngineHandler(w, req)
	require.NoError(t, err)

	// Should return 409 Conflict when CE is enabled
	assert.Equal(t, http.StatusConflict, w.Code)

	var respBody map[string]string
	err = json.NewDecoder(w.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Contains(t, respBody["error"], "must be disabled before it can be deleted")
}

func TestDeleteCognitionEngineHandler_WithMASAssociation(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server for RefreshConfig
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/cognition-fabric-nodes/test-cfn-id" {
			config := CfnConfigPayload{
				Workspaces: []WorkspaceConfig{
					{
						ID:            "ws-1",
						WorkspaceName: "Test Workspace",
						MultiAgenticSystems: []MASCfg{
							{
								ID:   "mas-1",
								Name: "Test MAS",
								CognitionEngines: []MASEngineCfg{
									{
										ID:   testCEID,
										Name: "Test CE",
									},
								},
							},
						},
					},
				},
				CognitionEngines: []EngineCfg{
					{
						ID:      testCEID,
						Name:    "Test CE",
						URL:     "http://ce-host:9004",
						Enabled: false, // CE is disabled but has MAS association
					},
				},
			}
			resp := map[string]interface{}{
				"config": config,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.deleteCognitionEngineHandler(w, req)
	require.NoError(t, err)

	// Should return 409 Conflict when CE has active MAS associations
	assert.Equal(t, http.StatusConflict, w.Code)

	var respBody map[string]string
	err = json.NewDecoder(w.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Contains(t, respBody["error"], "active MAS associations")
}

func TestDeleteCognitionEngineHandler_RefreshConfigFails(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server that returns error on refresh
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle RefreshConfig GET request - return error
		if r.Method == http.MethodGet && r.URL.Path == "/api/cognition-fabric-nodes/test-cfn-id" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "internal server error",
			})
			return
		}
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Set up ParsedConfig with disabled CE
	cfnConfigMutex.Lock()
	oldConfig := ParsedConfig
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{},
		CognitionEngines: []EngineCfg{
			{
				ID:      testCEID,
				Name:    "Test CE",
				URL:     "http://ce-host:9004",
				Enabled: false,
			},
		},
	}
	cfnConfigMutex.Unlock()
	defer func() {
		cfnConfigMutex.Lock()
		ParsedConfig = oldConfig
		cfnConfigMutex.Unlock()
	}()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.deleteCognitionEngineHandler(w, req)
	require.NoError(t, err)

	// Should return 503 Service Unavailable when RefreshConfig fails
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var respBody map[string]string
	err = json.NewDecoder(w.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Contains(t, respBody["error"], "config refresh failed")
}

func TestDeleteCognitionEngineHandler_DisabledCE_NoMASAssociation(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle RefreshConfig GET request
		if r.Method == http.MethodGet && r.URL.Path == "/api/cognition-fabric-nodes/test-cfn-id" {
			// Return config with disabled CE but no MAS associations
			config := CfnConfigPayload{
				Workspaces: []WorkspaceConfig{},
				CognitionEngines: []EngineCfg{
					{
						ID:      testCEID,
						Name:    "Test CE",
						URL:     "http://ce-host:9004",
						Enabled: false, // CE is disabled
					},
				},
			}
			resp := map[string]interface{}{
				"config": config,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Handle DELETE request - should succeed because no MAS association
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/cognition-engines/"+testCEID, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	// Set up ParsedConfig with disabled CE and no MAS associations
	cfnConfigMutex.Lock()
	oldConfig := ParsedConfig
	ParsedConfig = &CfnConfigPayload{
		Workspaces: []WorkspaceConfig{},
		CognitionEngines: []EngineCfg{
			{
				ID:      testCEID,
				Name:    "Test CE",
				URL:     "http://ce-host:9004",
				Enabled: false, // CE is disabled - allows delete
			},
		},
	}
	cfnConfigMutex.Unlock()
	defer func() {
		cfnConfigMutex.Lock()
		ParsedConfig = oldConfig
		cfnConfigMutex.Unlock()
	}()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete, "/api/cognition-engines/"+testCEID, nil)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.deleteCognitionEngineHandler(w, req)
	require.NoError(t, err)

	// Should return 204 No Content - delete succeeds when CE is disabled and has no MAS association
	assert.Equal(t, http.StatusNoContent, w.Code)
}
