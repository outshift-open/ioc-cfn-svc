package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
	"github.com/stretchr/testify/assert"
)

func TestL9Handler(t *testing.T) {
	app := &App{}

	// Create a sample L9 message
	l9Msg := l9.L9{
		Header: l9.L9Header{
			Protocol:    "sstp",
			Subprotocol: "test",
			Version:     "1.0",
			Kind:        l9.KindIntent,
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{
						ID:   "sender-1",
						Role: "client",
					},
					{
						ID:   "receiver-1",
						Role: "server",
					},
				},
			},
		},
		Payload: l9.L9Payload{},
	}

	// Marshal to JSON
	body, err := json.Marshal(l9Msg)
	assert.NoError(t, err)

	// Create request with path parameters
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws-1/multi-agentic-systems/mas-1/cognition-engines/ce-1/l9", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", "ws-1")
	req.SetPathValue("masId", "mas-1")
	req.SetPathValue("ceId", "ce-1")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handler
	code, err := app.l9Handler(w, req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)

	// Verify response
	var responseMsgJSON map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &responseMsgJSON)
	assert.NoError(t, err)

	// Check that response has header and payload
	assert.Contains(t, responseMsgJSON, "header")
	assert.Contains(t, responseMsgJSON, "payload")
}

func TestL9Handler_InvalidJSON(t *testing.T) {
	app := &App{}

	// Create request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws-1/multi-agentic-systems/mas-1/cognition-engines/ce-1/l9", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", "ws-1")
	req.SetPathValue("masId", "mas-1")
	req.SetPathValue("ceId", "ce-1")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handler
	code, _ := app.l9Handler(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, code)
}
