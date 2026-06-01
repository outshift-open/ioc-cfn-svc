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

func TestAssociateMASCEHandler(t *testing.T) {
	testWorkspaceID := "95f5863c-41b6-4dee-bbc2-eee3156b7d10"
	testMASID := "e9b5592f-326d-42e3-8bbe-29cf876ebc7c"
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		expectedPath := "/api/workspaces/" + testWorkspaceID + "/multi-agentic-systems/" + testMASID + "/cognition-engines"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Parse the forwarded request
		var payload cognitionengine.MASCEAssociateRequest
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		assert.Equal(t, testCEID, payload.CEID)

		// Return success response
		resp := cognitionengine.MASCEAssociateResponse{
			CEID:      testCEID,
			MASID:     testMASID,
			CreatedAt: "2026-06-01T15:30:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
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
	reqBody := cognitionengine.MASCEAssociateRequest{
		CEID: testCEID,
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/"+testWorkspaceID+"/multi-agentic-systems/"+testMASID+"/cognition-engines",
		bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", testWorkspaceID)
	req.SetPathValue("masId", testMASID)
	w := httptest.NewRecorder()

	_, err = app.associateMASCEHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp cognitionengine.MASCEAssociateResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, testCEID, resp.CEID)
	assert.Equal(t, testMASID, resp.MASID)
}

func TestAssociateMASCEHandler_InvalidCEID(t *testing.T) {
	testWorkspaceID := "95f5863c-41b6-4dee-bbc2-eee3156b7d10"
	testMASID := "e9b5592f-326d-42e3-8bbe-29cf876ebc7c"

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	reqBody := cognitionengine.MASCEAssociateRequest{
		CEID: "invalid-uuid",
	}
	reqBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/"+testWorkspaceID+"/multi-agentic-systems/"+testMASID+"/cognition-engines",
		bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", testWorkspaceID)
	req.SetPathValue("masId", testMASID)
	w := httptest.NewRecorder()

	_, err = app.associateMASCEHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "invalid ce_id format")
}

func TestDisassociateMASCEHandler(t *testing.T) {
	testWorkspaceID := "95f5863c-41b6-4dee-bbc2-eee3156b7d10"
	testMASID := "e9b5592f-326d-42e3-8bbe-29cf876ebc7c"
	testCEID := "550e8400-e29b-41d4-a716-446655440000"

	// Setup mock management plane server
	mgmtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		expectedPath := "/api/workspaces/" + testWorkspaceID + "/multi-agentic-systems/" + testMASID + "/cognition-engines/" + testCEID
		assert.Equal(t, expectedPath, r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer mgmtServer.Close()

	os.Setenv("MGMT_URL", mgmtServer.URL)
	defer os.Unsetenv("MGMT_URL")

	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	req := httptest.NewRequest(http.MethodDelete,
		"/api/workspaces/"+testWorkspaceID+"/multi-agentic-systems/"+testMASID+"/cognition-engines/"+testCEID,
		nil)
	req.SetPathValue("workspaceId", testWorkspaceID)
	req.SetPathValue("masId", testMASID)
	req.SetPathValue("ceId", testCEID)
	w := httptest.NewRecorder()

	_, err := app.disassociateMASCEHandler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDisassociateMASCEHandler_InvalidUUIDs(t *testing.T) {
	originalCfnID := CfnID
	CfnID = "test-cfn-id"
	defer func() { CfnID = originalCfnID }()

	app := &App{}

	tests := []struct {
		name        string
		workspaceID string
		masID       string
		ceID        string
		errMsg      string
	}{
		{
			name:        "invalid workspace_id",
			workspaceID: "invalid",
			masID:       "e9b5592f-326d-42e3-8bbe-29cf876ebc7c",
			ceID:        "550e8400-e29b-41d4-a716-446655440000",
			errMsg:      "invalid workspace_id format",
		},
		{
			name:        "invalid mas_id",
			workspaceID: "95f5863c-41b6-4dee-bbc2-eee3156b7d10",
			masID:       "invalid",
			ceID:        "550e8400-e29b-41d4-a716-446655440000",
			errMsg:      "invalid mas_id format",
		},
		{
			name:        "invalid ce_id",
			workspaceID: "95f5863c-41b6-4dee-bbc2-eee3156b7d10",
			masID:       "e9b5592f-326d-42e3-8bbe-29cf876ebc7c",
			ceID:        "invalid",
			errMsg:      "invalid ce_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete,
				"/api/workspaces/"+tt.workspaceID+"/multi-agentic-systems/"+tt.masID+"/cognition-engines/"+tt.ceID,
				nil)
			req.SetPathValue("workspaceId", tt.workspaceID)
			req.SetPathValue("masId", tt.masID)
			req.SetPathValue("ceId", tt.ceID)
			w := httptest.NewRecorder()

			_, err := app.disassociateMASCEHandler(w, req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp map[string]string
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Contains(t, resp["error"], tt.errMsg)
		})
	}
}
