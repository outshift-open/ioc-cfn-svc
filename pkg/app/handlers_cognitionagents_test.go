package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/cognitionagents"
)

var validHeader = cognitionagents.Header{
	WorkspaceID: "ws-456",
	MASID:       "mas-123",
	AgentID:     "agent-42",
}

// --------------- Memory Create ---------------

func TestMemoryCreateHandler_Success(t *testing.T) {
	app := &App{}
	body := cognitionagents.MemoryCreateRequest{Header: validHeader}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, err := app.cognitionAgentsMemoryCreateHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, code)

	var resp cognitionagents.MemoryCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, validHeader.WorkspaceID, resp.Header.WorkspaceID)
	assert.Equal(t, validHeader.MASID, resp.Header.MASID)
	assert.NotEmpty(t, resp.ResponseID)
	assert.Nil(t, resp.Error)
}

func TestMemoryCreateHandler_MissingHeader(t *testing.T) {
	app := &App{}
	body := cognitionagents.MemoryCreateRequest{Header: cognitionagents.Header{}}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsMemoryCreateHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.MemoryCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "workspace_id and mas_id are mandatory", resp.Error.Message)
}

func TestMemoryCreateHandler_InvalidJSON(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsMemoryCreateHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.MemoryCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "invalid JSON body", resp.Error.Message)
}

// --------------- Memory Search ---------------

func TestMemorySearchHandler_Success(t *testing.T) {
	app := &App{}
	body := cognitionagents.MemorySearchRequest{
		Header:    validHeader,
		Queries:   []string{"I am interested in inexpensive restaurant", "best coffee nearby"},
		Embedding: []float64{23, 45, 6},
		K:         10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, err := app.cognitionAgentsMemorySearchHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp cognitionagents.MemorySearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, validHeader.WorkspaceID, resp.Header.WorkspaceID)
	assert.NotEmpty(t, resp.ResponseID)
	assert.Nil(t, resp.Error)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "I am interested in inexpensive restaurant", resp.Results[0].Query)
	assert.Equal(t, "best coffee nearby", resp.Results[1].Query)
	assert.Empty(t, resp.Results[0].Hits)
	assert.Empty(t, resp.Results[1].Hits)
}

func TestMemorySearchHandler_MissingHeader(t *testing.T) {
	app := &App{}
	body := cognitionagents.MemorySearchRequest{
		Header:  cognitionagents.Header{},
		Queries: []string{"test"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsMemorySearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.MemorySearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "workspace_id and mas_id are mandatory", resp.Error.Message)
}

func TestMemorySearchHandler_InvalidJSON(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/search", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsMemorySearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.MemorySearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "invalid JSON body", resp.Error.Message)
}

// --------------- Concepts Search ---------------

func TestConceptsSearchHandler_Success(t *testing.T) {
	app := &App{}
	body := cognitionagents.ConceptsSearchRequest{Header: validHeader}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/concepts/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, err := app.cognitionAgentsConceptsSearchHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp cognitionagents.ConceptsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, validHeader.WorkspaceID, resp.Header.WorkspaceID)
	assert.NotEmpty(t, resp.ResponseID)
	assert.Nil(t, resp.Error)
	assert.Empty(t, resp.Results)
}

func TestConceptsSearchHandler_MissingHeader(t *testing.T) {
	app := &App{}
	body := cognitionagents.ConceptsSearchRequest{Header: cognitionagents.Header{}}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/concepts/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsConceptsSearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.ConceptsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "workspace_id and mas_id are mandatory", resp.Error.Message)
}

func TestConceptsSearchHandler_InvalidJSON(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/concepts/search", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionAgentsConceptsSearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.ConceptsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "invalid JSON body", resp.Error.Message)
}

// --------------- Paths Search ---------------

func TestPathsSearchHandler_Success(t *testing.T) {
	app := &App{}
	body := cognitionagents.PathsSearchRequest{
		Header: validHeader,
		Payload: cognitionagents.PathsSearchPayload{
			FromID:    "id-A",
			ToID:      "id-B",
			MaxDepth:  3,
			Relations: []string{"RELATED_TO"},
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/paths/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, err := app.cognitionagentsPathsSearchHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp cognitionagents.PathsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, validHeader.WorkspaceID, resp.Header.WorkspaceID)
	assert.NotEmpty(t, resp.ResponseID)
	assert.Nil(t, resp.Error)
	assert.Empty(t, resp.Paths)
}

func TestPathsSearchHandler_MissingHeader(t *testing.T) {
	app := &App{}
	body := cognitionagents.PathsSearchRequest{
		Header: cognitionagents.Header{},
		Payload: cognitionagents.PathsSearchPayload{
			FromID: "id-A",
			ToID:   "id-B",
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/paths/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionagentsPathsSearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.PathsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "workspace_id and mas_id are mandatory", resp.Error.Message)
}

func TestPathsSearchHandler_MissingPayloadIDs(t *testing.T) {
	app := &App{}
	body := cognitionagents.PathsSearchRequest{
		Header:  validHeader,
		Payload: cognitionagents.PathsSearchPayload{},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/paths/search", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionagentsPathsSearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.PathsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "from_id and to_id are mandatory", resp.Error.Message)
}

func TestPathsSearchHandler_InvalidJSON(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodPost, "/api/cfn/cfn-1/memory/paths/search", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()

	code, _ := app.cognitionagentsPathsSearchHandler(rr, req)
	assert.Equal(t, http.StatusBadRequest, code)

	var resp cognitionagents.PathsSearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "invalid JSON body", resp.Error.Message)
}
