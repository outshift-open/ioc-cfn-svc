package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/cognitiveagents"
)

func TestCognitiveAgentsMemoryHandler_Success(t *testing.T) {
	app := &App{}

	body := cognitiveagents.MemoryQueryRequest{
		MASID:       "mas-123",
		WorkspaceID: "ws-456",
		Queries:     []string{"I am interested in inexpensive restaurant", "best coffee nearby"},
		Embedding:   []float64{23, 45, 6},
		K:           10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/memory/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	code, err := app.cognitiveAgentsMemoryHandler(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	var resp cognitiveagents.MemoryQueryResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "I am interested in inexpensive restaurant", resp.Results[0].Query)
	assert.Equal(t, "best coffee nearby", resp.Results[1].Query)
	assert.Empty(t, resp.Results[0].Hits)
	assert.Empty(t, resp.Results[1].Hits)
}

