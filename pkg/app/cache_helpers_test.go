// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

func TestWorkspaceMASCacheIsolation(t *testing.T) {
	app := &App{}

	// Create messages for different workspace+MAS pairs
	msg1 := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "SSTP",
			Version:     "1.0",
			Subprotocol: "SAB",
			Kind:        l9.KindKnowledge,
			Message: &l9.L9HeaderMessage{
				ID:      "msg1",
				Parents: []string{},
			},
			Participants: l9.ParticipantSet{
				Groups: &l9.ParticipantSetGroups{
					"workspace_id": "ws1",
					"mas_id":       "mas1",
				},
			},
		},
	}

	msg2 := &l9.L9{
		Header: l9.L9Header{
			Protocol:    "SSTP",
			Version:     "1.0",
			Subprotocol: "SAB",
			Kind:        l9.KindKnowledge,
			Message: &l9.L9HeaderMessage{
				ID:      "msg2",
				Parents: []string{},
			},
			Participants: l9.ParticipantSet{
				Groups: &l9.ParticipantSetGroups{
					"workspace_id": "ws2",
					"mas_id":       "mas2",
				},
			},
		},
	}

	// Cache messages
	app.cacheL9Message(msg1)
	app.cacheL9Message(msg2)

	// Verify they went to different caches
	cache1 := app.getCacheForWorkspaceMAS("ws1", "mas1")
	cache2 := app.getCacheForWorkspaceMAS("ws2", "mas2")

	if cache1 == cache2 {
		t.Error("Expected different caches for different workspace+MAS pairs")
	}

	// Verify msg1 is in cache1
	retrieved1, err := cache1.Get("msg1")
	if err != nil {
		t.Errorf("Expected msg1 in ws1:mas1 cache, got error: %v", err)
	}
	if retrieved1.Header.Message.ID != "msg1" {
		t.Errorf("Expected msg1, got %s", retrieved1.Header.Message.ID)
	}

	// Verify msg2 is in cache2
	retrieved2, err := cache2.Get("msg2")
	if err != nil {
		t.Errorf("Expected msg2 in ws2:mas2 cache, got error: %v", err)
	}
	if retrieved2.Header.Message.ID != "msg2" {
		t.Errorf("Expected msg2, got %s", retrieved2.Header.Message.ID)
	}

	// Verify msg1 is NOT in cache2
	_, err = cache2.Get("msg1")
	if err == nil {
		t.Error("msg1 should not be in ws2:mas2 cache")
	}

	// Verify msg2 is NOT in cache1
	_, err = cache1.Get("msg2")
	if err == nil {
		t.Error("msg2 should not be in ws1:mas1 cache")
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		workspace string
		mas       string
		expected  string
	}{
		{"ws1", "mas1", "ws1:mas1"},
		{"workspace-123", "mas-456", "workspace-123:mas-456"},
		{"", "", ":"},
	}

	for _, tt := range tests {
		result := cacheKey(tt.workspace, tt.mas)
		if result != tt.expected {
			t.Errorf("cacheKey(%s, %s) = %s, want %s",
				tt.workspace, tt.mas, result, tt.expected)
		}
	}
}
