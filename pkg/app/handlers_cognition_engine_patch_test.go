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

func TestPatchCognitionEngineHandler(t *testing.T) {
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/cognition-engines/"+testCEID, r.URL.Path)

		// Parse the forwarded request
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify patch payload
		assert.Equal(t, false, payload["enabled"])

		// Return success response
		resp := cognitionengine.CognitionEngineDetail{
			ID:         testCEID,
			CFNID:      "test-cfn-id",
			Name:       "Test CE",
			Version:    "1.0.0",
			Type:       "knowledge_management",
			URL:        "http://ce-host:9004",
			Enabled:    false, // Updated
			AutoAttach: false,
			Status:     "online",
			CreatedAt:  "2026-05-21T09:00:00Z",
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

	// Create test request
	enabled := false
	reqBody := cognitionengine.PatchRequest{
		Enabled: &enabled,
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/cognition-engines/"+testCEID, bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err = app.patchCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp cognitionengine.CognitionEngineDetail
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, testCEID, resp.ID)
	assert.False(t, resp.Enabled)
}

func TestPatchCognitionEngineHandler_InvalidCEID(t *testing.T) {
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	enabled := false
	reqBody := cognitionengine.PatchRequest{
		Enabled: &enabled,
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/cognition-engines/invalid-uuid", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("ceId", "invalid-uuid")
	w := httptest.NewRecorder()

	_, err = app.patchCognitionEngineHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "invalid ce_id format")
}
