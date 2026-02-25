package mem0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- ForwardRequest proxy safety tests ---

func TestForwardRequest_AllowsConfiguredHost(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-api-key" {
			t.Fatalf("expected auth header to be set, got: %q", got)
		}
		if r.URL.Path != "/api/v1/memories/" {
			t.Fatalf("expected path /api/v1/memories/, got: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.ForwardRequest(context.Background(), http.MethodGet, mem0Server.URL+"/api/v1/memories/", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HTTPStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.HTTPStatus)
	}
}

func TestForwardRequest_BlocksDifferentHost(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mem0Server.Close()

	otherServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer otherServer.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.ForwardRequest(context.Background(), http.MethodGet, otherServer.URL+"/api/v1/memories/", nil, nil)
	if err == nil {
		t.Fatal("expected error for different host, got nil")
	}
	if !strings.Contains(err.Error(), "target host must match configured mem0 endpoint") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForwardRequest_AllowsRelativePath(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/memories/filter" {
			t.Fatalf("expected path /api/v1/memories/filter, got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "total": 0})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.ForwardRequest(context.Background(), http.MethodPost, "/api/v1/memories/filter", []byte(`{"query":"q","user_id":"u"}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HTTPStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.HTTPStatus)
	}
}

// --- Typed API method tests ---

func TestCreateMemory(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/" {
			t.Fatalf("expected path /api/v1/memories/, got: %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body["text"] != "test memory" {
			t.Fatalf("expected text 'test memory', got: %v", body["text"])
		}
		if body["user_id"] != "user-1" {
			t.Fatalf("expected user_id 'user-1', got: %v", body["user_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "mem-123",
			"content": "test memory",
			"state":   "active",
			"user_id": "user-1",
		})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.CreateMemory(context.Background(), &CreateMemoryRequest{
		Text:   "test memory",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "mem-123" {
		t.Fatalf("expected id 'mem-123', got: %s", resp.ID)
	}
	if resp.Content != "test memory" {
		t.Fatalf("expected content 'test memory', got: %s", resp.Content)
	}
}

func TestListMemories(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("user_id") != "user-1" {
			t.Fatalf("expected user_id query param 'user-1', got: %s", r.URL.Query().Get("user_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "mem-1", "content": "memory one", "created_at": 123, "state": "active"},
				{"id": "mem-2", "content": "memory two", "created_at": 456, "state": "active"},
			},
			"total": 2,
			"page":  1,
			"size":  50,
			"pages": 1,
		})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.ListMemories(context.Background(), &ListMemoriesRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected total 2, got: %d", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got: %d", len(resp.Items))
	}
	if resp.Items[0].Content != "memory one" {
		t.Fatalf("expected first item content 'memory one', got: %s", resp.Items[0].Content)
	}
}

func TestFilterMemories(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/filter" {
			t.Fatalf("expected path /api/v1/memories/filter, got: %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body["query"] != "hobbies" {
			t.Fatalf("expected query 'hobbies', got: %v", body["query"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "mem-1", "content": "likes hiking", "created_at": 123, "state": "active"},
			},
			"total": 1,
			"page":  1,
			"size":  50,
			"pages": 1,
		})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.FilterMemories(context.Background(), &FilterMemoriesRequest{
		Query:  "hobbies",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got: %d", resp.Total)
	}
}

func TestDeleteMemories(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		ids, ok := body["memory_ids"].([]interface{})
		if !ok || len(ids) != 2 {
			t.Fatalf("expected 2 memory_ids, got: %v", body["memory_ids"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Successfully deleted 2 memories",
		})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.DeleteMemories(context.Background(), &DeleteMemoriesRequest{
		MemoryIDs: []string{"mem-1", "mem-2"},
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Message, "2 memories") {
		t.Fatalf("expected message about 2 memories, got: %s", resp.Message)
	}
}

func TestUpdateMemory(t *testing.T) {
	mem0Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/memories/mem-123" {
			t.Fatalf("expected path /api/v1/memories/mem-123, got: %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body["memory_content"] != "updated content" {
			t.Fatalf("expected memory_content 'updated content', got: %v", body["memory_content"])
		}
		if body["user_id"] != "user-1" {
			t.Fatalf("expected user_id 'user-1', got: %v", body["user_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "mem-123",
			"content": "updated content",
			"state":   "active",
			"user_id": "user-1",
		})
	}))
	defer mem0Server.Close()

	cfg := DefaultClientConfig()
	cfg.BaseURL = mem0Server.URL
	cfg.APIKey = "test-api-key" // test-only value

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.UpdateMemory(context.Background(), "mem-123", &UpdateMemoryRequest{
		MemoryContent: "updated content",
		UserID:        "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "updated content" {
		t.Fatalf("expected content 'updated content', got: %s", resp.Content)
	}
}

// --- Validation tests ---

func TestCreateMemory_ValidationFails(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseURL = "http://localhost:9999"
	cfg.APIKey = "test-api-key" // test-only value

	client, _ := NewClient(cfg)

	_, err := client.CreateMemory(context.Background(), &CreateMemoryRequest{})
	if err == nil || !strings.Contains(err.Error(), "text is required") {
		t.Fatalf("expected validation error for missing text, got: %v", err)
	}

	_, err = client.CreateMemory(context.Background(), &CreateMemoryRequest{Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "user_id is required") {
		t.Fatalf("expected validation error for missing user_id, got: %v", err)
	}
}

func TestDeleteMemories_ValidationFails(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseURL = "http://localhost:9999"
	cfg.APIKey = "test-api-key" // test-only value

	client, _ := NewClient(cfg)

	_, err := client.DeleteMemories(context.Background(), &DeleteMemoriesRequest{UserID: "u"})
	if err == nil || !strings.Contains(err.Error(), "memory_id") {
		t.Fatalf("expected validation error for empty memory_ids, got: %v", err)
	}

	_, err = client.DeleteMemories(context.Background(), &DeleteMemoriesRequest{MemoryIDs: []string{"id1"}})
	if err == nil || !strings.Contains(err.Error(), "user_id is required") {
		t.Fatalf("expected validation error for missing user_id, got: %v", err)
	}
}
