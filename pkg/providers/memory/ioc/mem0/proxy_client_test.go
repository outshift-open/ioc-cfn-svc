// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package mem0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────
// Config and Client Creation Tests
// ──────────────────────────────────────────────────────────

func TestDefaultProxyClientConfig(t *testing.T) {
	cfg := DefaultProxyClientConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", cfg.Timeout)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
}

func TestNewProxyClient_WithConfig(t *testing.T) {
	cfg := &ProxyClientConfig{
		APIKey:  "test-key",
		Timeout: 10 * time.Second,
	}
	client := NewProxyClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.cfg.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got %q", client.cfg.APIKey)
	}
	if client.cfg.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.cfg.Timeout)
	}
}

func TestNewProxyClient_WithNilConfig(t *testing.T) {
	client := NewProxyClient(nil)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	// Should use defaults
	if client.cfg.Timeout != 5*time.Minute {
		t.Errorf("expected default timeout 5m, got %v", client.cfg.Timeout)
	}
}

// ──────────────────────────────────────────────────────────
// ForwardRequest Tests
// ──────────────────────────────────────────────────────────

func TestForwardRequest_Success(t *testing.T) {
	// Mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("X-Custom") != "test-value" {
			t.Errorf("expected X-Custom header, got %q", r.Header.Get("X-Custom"))
		}
		if r.Header.Get("Authorization") != "Token test-api-key" {
			t.Errorf("expected Authorization header with API key, got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"id":     "123",
		})
	}))
	defer mockServer.Close()

	cfg := &ProxyClientConfig{APIKey: "test-api-key"}
	client := NewProxyClient(cfg)

	headers := map[string]string{
		"X-Custom": "test-value",
	}
	body := []byte(`{"test":"data"}`)

	resp, err := client.ForwardRequest(context.Background(), "POST", mockServer.URL, body, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HTTPStatus != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.HTTPStatus)
	}
	if resp.HTTPResponseBody["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp.HTTPResponseBody["status"])
	}
}

func TestForwardRequest_SecurityStripsUserAuthHeader(t *testing.T) {
	// This is a CRITICAL security test
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		// Should receive server-configured auth, NOT user-provided auth
		if authHeader != "Token server-key" {
			t.Errorf("SECURITY VIOLATION: expected 'Token server-key', got %q", authHeader)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mockServer.Close()

	cfg := &ProxyClientConfig{APIKey: "server-key"}
	client := NewProxyClient(cfg)

	// Attacker tries to inject their own auth header
	maliciousHeaders := map[string]string{
		"Authorization": "Bearer attacker-token",
	}

	_, err := client.ForwardRequest(context.Background(), "GET", mockServer.URL, nil, maliciousHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Test passes if mock server verifies correct auth header
}

func TestForwardRequest_NoAuthWhenNotConfigured(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			t.Errorf("expected no Authorization header, got %q", authHeader)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mockServer.Close()

	// Client with no API key
	cfg := &ProxyClientConfig{APIKey: ""}
	client := NewProxyClient(cfg)

	_, err := client.ForwardRequest(context.Background(), "GET", mockServer.URL, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForwardRequest_ValidationErrors(t *testing.T) {
	client := NewProxyClient(nil)

	tests := []struct {
		name      string
		method    string
		targetURL string
		wantErr   string
	}{
		{
			name:      "empty method",
			method:    "",
			targetURL: "http://example.com",
			wantErr:   "method and targetURL are required",
		},
		{
			name:      "empty URL",
			method:    "GET",
			targetURL: "",
			wantErr:   "method and targetURL are required",
		},
		{
			name:      "unsupported method",
			method:    "TRACE",
			targetURL: "http://example.com",
			wantErr:   "unsupported HTTP method: TRACE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ForwardRequest(context.Background(), tt.method, tt.targetURL, nil, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestForwardRequest_MethodNormalization(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)

	// Test lowercase and whitespace handling
	_, err := client.ForwardRequest(context.Background(), "  post  ", mockServer.URL, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForwardRequest_ContentTypeHandling(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", contentType)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)

	// Should auto-set Content-Type when body is present
	body := []byte(`{"test":"data"}`)
	_, err := client.ForwardRequest(context.Background(), "POST", mockServer.URL, body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForwardRequest_UserContentTypePreserved(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "text/plain" {
			t.Errorf("expected Content-Type 'text/plain', got %q", contentType)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)

	// User provides custom Content-Type
	headers := map[string]string{"Content-Type": "text/plain"}
	body := []byte("plain text")
	_, err := client.ForwardRequest(context.Background(), "POST", mockServer.URL, body, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForwardRequest_NonJSONResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain text response"))
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)

	resp, err := client.ForwardRequest(context.Background(), "GET", mockServer.URL, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-JSON should be wrapped
	if resp.HTTPResponseBody["raw"] != "plain text response" {
		t.Errorf("expected raw text wrapped, got %v", resp.HTTPResponseBody)
	}
}

// ──────────────────────────────────────────────────────────
// Typed Convenience Methods Tests
// ──────────────────────────────────────────────────────────

func TestCreateMemory_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/" {
			t.Errorf("expected path /api/v1/memories/, got %s", r.URL.Path)
		}

		var req CreateMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Text != "test memory" {
			t.Errorf("expected text 'test memory', got %q", req.Text)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateMemoryResponse{
			ID:      "mem-123",
			Content: "test memory",
			UserID:  "user-1",
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &CreateMemoryRequest{
		Text:   "test memory",
		UserID: "user-1",
	}

	resp, err := client.CreateMemory(context.Background(), mockServer.URL, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "mem-123" {
		t.Errorf("expected ID 'mem-123', got %q", resp.ID)
	}
}

func TestCreateMemory_Duplicate(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OpenMemory returns null for duplicates (empty JSON object)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &CreateMemoryRequest{
		Text:   "duplicate memory",
		UserID: "user-1",
	}

	resp, err := client.CreateMemory(context.Background(), mockServer.URL, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return nil for duplicates
	if resp != nil {
		t.Errorf("expected nil response for duplicate, got %+v", resp)
	}
}

func TestCreateMemory_ValidationError(t *testing.T) {
	client := NewProxyClient(nil)

	tests := []struct {
		name string
		req  *CreateMemoryRequest
	}{
		{
			name: "missing text",
			req:  &CreateMemoryRequest{UserID: "user-1"},
		},
		{
			name: "missing user_id",
			req:  &CreateMemoryRequest{Text: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CreateMemory(context.Background(), "http://example.com", tt.req)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestListMemories_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Errorf("expected user_id=user-1, got %s", r.URL.Query().Get("user_id"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListMemoriesResponse{
			Items: []MemoryListItem{
				{ID: "mem-1", Content: "memory 1"},
				{ID: "mem-2", Content: "memory 2"},
			},
			Total: 2,
			Page:  1,
			Size:  10,
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &ListMemoriesRequest{UserID: "user-1"}

	resp, err := client.ListMemories(context.Background(), mockServer.URL, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}
}

func TestGetMemory_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/mem-123" {
			t.Errorf("expected path /api/v1/memories/mem-123, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GetMemoryResponse{
			ID:   "mem-123",
			Text: "memory content",
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	resp, err := client.GetMemory(context.Background(), mockServer.URL, "mem-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "mem-123" {
		t.Errorf("expected ID 'mem-123', got %q", resp.ID)
	}
}

func TestUpdateMemory_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/mem-123" {
			t.Errorf("expected path /api/v1/memories/mem-123, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(UpdateMemoryResponse{
			ID:      "mem-123",
			Content: "updated content",
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &UpdateMemoryRequest{
		MemoryContent: "updated content",
		UserID:        "user-1",
	}

	resp, err := client.UpdateMemory(context.Background(), mockServer.URL, "mem-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "mem-123" {
		t.Errorf("expected ID 'mem-123', got %q", resp.ID)
	}
}

func TestDeleteMemories_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DeleteMemoriesResponse{
			Message: "deleted successfully",
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &DeleteMemoriesRequest{
		MemoryIDs: []string{"mem-1", "mem-2"},
		UserID:    "user-1",
	}

	resp, err := client.DeleteMemories(context.Background(), mockServer.URL, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "deleted successfully" {
		t.Errorf("expected success message, got %q", resp.Message)
	}
}

func TestFilterMemories_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/filter" {
			t.Errorf("expected path /api/v1/memories/filter, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(FilterMemoriesResponse{
			Items: []MemoryListItem{
				{ID: "mem-1", Content: "filtered memory"},
			},
			Total: 1,
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	req := &FilterMemoriesRequest{
		Query:  "search term",
		UserID: "user-1",
	}

	resp, err := client.FilterMemories(context.Background(), mockServer.URL, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(resp.Items))
	}
}

func TestGetCategories_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/categories" {
			t.Errorf("expected path /api/v1/memories/categories, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Errorf("expected user_id=user-1, got %s", r.URL.Query().Get("user_id"))
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CategoriesResponse{
			Categories: []Category{
				{ID: "cat-1", Name: "work"},
				{ID: "cat-2", Name: "personal"},
				{ID: "cat-3", Name: "ideas"},
			},
			Total: 3,
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	resp, err := client.GetCategories(context.Background(), mockServer.URL, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Categories) != 3 {
		t.Errorf("expected 3 categories, got %d", len(resp.Categories))
	}
	if resp.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Total)
	}
}

func TestGetCategories_ValidationError(t *testing.T) {
	client := NewProxyClient(nil)
	_, err := client.GetCategories(context.Background(), "http://example.com", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestGetRelatedMemories_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/mem-123/related" {
			t.Errorf("expected path /api/v1/memories/mem-123/related, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Errorf("expected user_id=user-1, got %s", r.URL.Query().Get("user_id"))
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RelatedMemoriesResponse{
			Items: []MemoryListItem{
				{ID: "rel-1", Content: "related memory 1"},
				{ID: "rel-2", Content: "related memory 2"},
			},
			Total: 2,
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	resp, err := client.GetRelatedMemories(context.Background(), mockServer.URL, "mem-123", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 related memories, got %d", len(resp.Items))
	}
}

func TestGetRelatedMemories_ValidationError(t *testing.T) {
	client := NewProxyClient(nil)

	tests := []struct {
		name      string
		memoryID  string
		userID    string
	}{
		{"missing memory_id", "", "user-1"},
		{"missing user_id", "mem-123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetRelatedMemories(context.Background(), "http://example.com", tt.memoryID, tt.userID)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestGetStats_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/stats/" {
			t.Errorf("expected path /api/v1/stats/, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Errorf("expected user_id=user-1, got %s", r.URL.Query().Get("user_id"))
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(StatsResponse{
			TotalMemories: 42,
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)
	resp, err := client.GetStats(context.Background(), mockServer.URL, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalMemories != 42 {
		t.Errorf("expected total_memories=42, got %d", resp.TotalMemories)
	}
}

func TestGetStats_ValidationError(t *testing.T) {
	client := NewProxyClient(nil)
	_, err := client.GetStats(context.Background(), "http://example.com", "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// ──────────────────────────────────────────────────────────
// Error Handling Tests
// ──────────────────────────────────────────────────────────

func TestTypedMethods_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"detail": "invalid request",
		})
	}))
	defer mockServer.Close()

	client := NewProxyClient(nil)

	_, err := client.CreateMemory(context.Background(), mockServer.URL, &CreateMemoryRequest{
		Text:   "test",
		UserID: "user-1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Detail != "invalid request" {
		t.Errorf("expected detail 'invalid request', got %q", apiErr.Detail)
	}
}
