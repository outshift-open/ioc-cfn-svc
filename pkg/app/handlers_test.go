package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCfnDummyHandler(t *testing.T) {
	// Create a mock app with minimal setup
	app := &App{}

	// Create request
	req, err := http.NewRequest(http.MethodGet, "/api/v1/cfn/dummy", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	statusCode, err := app.getCfnDummyHandler(rr, req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Check status code
	if statusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	// Parse response body
	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check response fields
	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}

	if response["message"] != "cfn dummy response" {
		t.Errorf("expected message 'cfn dummy response', got '%s'", response["message"])
	}
}
