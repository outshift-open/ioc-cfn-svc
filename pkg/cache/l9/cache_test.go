// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package l9cache

import (
	"fmt"
	"sync"
	"testing"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

// Helper function to create a test L9 message
func createTestMessage(id string, parents []string, episode string, senderID string) *l9.L9 {
	return &l9.L9{
		Header: l9.L9Header{
			Protocol:    "SSTP",
			Version:     "1.0",
			Subprotocol: "SAB",
			Kind:        l9.KindKnowledge,
			Message: &l9.L9HeaderMessage{
				ID:      id,
				Parents: parents,
				Episode: episode,
			},
			Participants: l9.ParticipantSet{
				Actors: []l9.Actor{
					{ID: senderID, Role: "sender"},
				},
			},
		},
		Payload: l9.L9Payload{},
	}
}

func TestNew(t *testing.T) {
	cache := New()
	if cache == nil {
		t.Fatal("New() returned nil")
	}
	convs := cache.ListConversations()
	if len(convs) != 0 {
		t.Errorf("New cache should be empty, got %d conversations", len(convs))
	}
}

func TestAdd_SingleMessage(t *testing.T) {
	cache := New()
	msg := createTestMessage("msg1", []string{}, "episode1", "sender1")

	err := cache.Add(msg)
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	convs := cache.ListConversations()
	if len(convs) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(convs))
	}

	retrieved, err := cache.Get("msg1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if retrieved.Header.Message.ID != "msg1" {
		t.Errorf("Expected msg1, got %s", retrieved.Header.Message.ID)
	}
}

func TestAdd_NilMessage(t *testing.T) {
	cache := New()
	err := cache.Add(nil)
	if err != ErrInvalidMessage {
		t.Errorf("Expected ErrInvalidMessage, got %v", err)
	}
}

func TestAdd_InvalidMessage_NoID(t *testing.T) {
	cache := New()
	msg := &l9.L9{
		Header: l9.L9Header{
			Message: &l9.L9HeaderMessage{
				ID: "", // Empty ID
			},
		},
	}

	err := cache.Add(msg)
	if err == nil {
		t.Error("Expected error for message without ID")
	}
}

func TestAdd_InvalidMessage_NoMessageField(t *testing.T) {
	cache := New()
	msg := &l9.L9{
		Header: l9.L9Header{
			Message: nil, // No message field
		},
	}

	err := cache.Add(msg)
	if err == nil {
		t.Error("Expected error for message without Message field")
	}
}

func TestAdd_DuplicateMessage(t *testing.T) {
	cache := New()

	msg1 := createTestMessage("msg1", []string{}, "episode1", "sender1")
	msg1Updated := createTestMessage("msg1", []string{}, "episode1", "sender2") // Same ID, different content

	// Add first version
	if err := cache.Add(msg1); err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// Add duplicate - should update in place without creating duplicate conversation entry
	if err := cache.Add(msg1Updated); err != nil {
		t.Fatalf("Failed to add duplicate message: %v", err)
	}

	// Should only have one conversation
	convs := cache.ListConversations()
	if len(convs) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(convs))
	}

	// Should only have one message in conversation
	if convs[0].MessageCount != 1 {
		t.Errorf("Expected 1 message in conversation, got %d", convs[0].MessageCount)
	}

	// Retrieved message should be the updated version
	retrieved, err := cache.Get("msg1")
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if retrieved.Header.Participants.Actors[0].ID != "sender2" {
		t.Errorf("Expected updated message, got sender %s", retrieved.Header.Participants.Actors[0].ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	cache := New()
	_, err := cache.Get("nonexistent")
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound, got %v", err)
	}
}

func TestConversation_SingleRoot(t *testing.T) {
	cache := New()

	// Create a conversation: root -> msg2 -> msg3
	root := createTestMessage("root", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"root"}, "episode1", "sender1")
	msg3 := createTestMessage("msg3", []string{"msg2"}, "episode1", "sender1")

	cache.Add(root)
	cache.Add(msg2)
	cache.Add(msg3)

	// All messages should belong to the same conversation
	// Can query from any message in the conversation
	conv, err := cache.GetConversationByMessageID("msg2")
	if err != nil {
		t.Fatalf("GetConversationByMessageID() failed: %v", err)
	}

	if len(conv) != 3 {
		t.Errorf("Expected 3 messages in conversation, got %d", len(conv))
	}

	// Check they're in insertion order
	if conv[0].Header.Message.ID != "root" {
		t.Errorf("Expected first message to be root, got %s", conv[0].Header.Message.ID)
	}
}

func TestConversation_MultipleConversations(t *testing.T) {
	cache := New()

	// Conversation 1
	root1 := createTestMessage("root1", []string{}, "episode1", "sender1")
	msg1a := createTestMessage("msg1a", []string{"root1"}, "episode1", "sender1")

	// Conversation 2
	root2 := createTestMessage("root2", []string{}, "episode2", "sender2")
	msg2a := createTestMessage("msg2a", []string{"root2"}, "episode2", "sender2")

	cache.Add(root1)
	cache.Add(msg1a)
	cache.Add(root2)
	cache.Add(msg2a)

	conv1, err := cache.GetConversationByMessageID("msg1a")
	if err != nil {
		t.Fatalf("GetConversationByMessageID(msg1a) failed: %v", err)
	}
	if len(conv1) != 2 {
		t.Errorf("Expected 2 messages in conversation 1, got %d", len(conv1))
	}

	conv2, err := cache.GetConversationByMessageID("root2")
	if err != nil {
		t.Fatalf("GetConversationByMessageID(root2) failed: %v", err)
	}
	if len(conv2) != 2 {
		t.Errorf("Expected 2 messages in conversation 2, got %d", len(conv2))
	}
}


// TestWholeConversation_LinearChain tests a simple linear conversation
func TestWholeConversation_LinearChain(t *testing.T) {
	cache := New()

	// Create a linear conversation: root -> msg2 -> msg3 -> msg4
	messages := []struct {
		id      string
		parents []string
		episode string
	}{
		{"root", []string{}, "planning"},
		{"msg2", []string{"root"}, "planning"},
		{"msg3", []string{"msg2"}, "execution"},
		{"msg4", []string{"msg3"}, "execution"},
	}

	// Add all messages
	for _, m := range messages {
		msg := createTestMessage(m.id, m.parents, m.episode, "agent1")
		err := cache.Add(msg)
		if err != nil {
			t.Fatalf("Add(%s) failed: %v", m.id, err)
		}
	}

	// Get conversation from any message
	conv, err := cache.GetConversationByMessageID("msg3")
	if err != nil {
		t.Fatalf("GetConversationByMessageID() failed: %v", err)
	}

	// Verify all messages present
	if len(conv) != 4 {
		t.Errorf("Expected 4 messages in conversation, got %d", len(conv))
	}

	// Verify ordering (insertion order)
	expectedOrder := []string{"root", "msg2", "msg3", "msg4"}
	for i, msg := range conv {
		if msg.Header.Message.ID != expectedOrder[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedOrder[i], msg.Header.Message.ID)
		}
	}

	// Verify episodes are preserved
	if conv[0].Header.Message.Episode != "planning" {
		t.Errorf("Expected root to have episode 'planning', got %s", conv[0].Header.Message.Episode)
	}
	if conv[2].Header.Message.Episode != "execution" {
		t.Errorf("Expected msg3 to have episode 'execution', got %s", conv[2].Header.Message.Episode)
	}

	// Test context window - get last 2 messages ending at msg4 (includes msg4 itself)
	context, err := cache.GetLastNBeforeMessage("msg4", 2)
	if err != nil {
		t.Fatalf("GetLastNBeforeMessage() failed: %v", err)
	}

	if len(context) != 2 {
		t.Errorf("Expected 2 context messages, got %d", len(context))
	}

	// Context should be most recent first (msg4, then msg3)
	if context[0].Header.Message.ID != "msg4" {
		t.Errorf("Expected first context to be msg4, got %s", context[0].Header.Message.ID)
	}
	if context[1].Header.Message.ID != "msg3" {
		t.Errorf("Expected second context to be msg3, got %s", context[1].Header.Message.ID)
	}
}

// TestWholeConversation_MultiPhaseNegotiation simulates a realistic negotiation workflow
func TestWholeConversation_MultiPhaseNegotiation(t *testing.T) {
	cache := New()

	// Negotiation flow:
	// 1. Initial offer (root)
	// 2. Counter-offer (reply to root)
	// 3. Counter-counter (reply to counter)
	// 4. Agreement (reply to last counter)
	// 5. Confirmation (reply to agreement)

	negotiation := []struct {
		id       string
		parents  []string
		episode  string
		action   string
	}{
		{"offer-1", []string{}, "negotiation", "initial_offer"},
		{"counter-1", []string{"offer-1"}, "negotiation", "counter_offer"},
		{"counter-2", []string{"counter-1"}, "negotiation", "counter_offer"},
		{"agree-1", []string{"counter-2"}, "agreement", "accept"},
		{"confirm-1", []string{"agree-1"}, "agreement", "confirm"},
	}

	for _, n := range negotiation {
		msg := createTestMessage(n.id, n.parents, n.episode, "agent1")
		cache.Add(msg)
	}

	// Verify full negotiation chain
	conv, _ := cache.GetConversationByMessageID("counter-2")
	if len(conv) != 5 {
		t.Errorf("Expected 5 messages in negotiation, got %d", len(conv))
	}

	// Count messages per episode (manual filtering)
	negotiationCount := 0
	agreementCount := 0
	for _, msg := range conv {
		switch msg.Header.Message.Episode {
		case "negotiation":
			negotiationCount++
		case "agreement":
			agreementCount++
		}
	}

	if negotiationCount != 3 {
		t.Errorf("Expected 3 negotiation messages, got %d", negotiationCount)
	}
	if agreementCount != 2 {
		t.Errorf("Expected 2 agreement messages, got %d", agreementCount)
	}

	// Get last 3 messages ending at agree-1 (includes agree-1 which is in "agreement" episode)
	context, _ := cache.GetLastNBeforeMessage("agree-1", 3)
	if len(context) != 3 {
		t.Errorf("Expected 3 context messages, got %d", len(context))
	}

	// First should be agree-1 (agreement episode), rest should be negotiation
	if context[0].Header.Message.ID != "agree-1" {
		t.Errorf("Expected first to be agree-1, got %s", context[0].Header.Message.ID)
	}
	if context[0].Header.Message.Episode != "agreement" {
		t.Errorf("Expected agree-1 to be in agreement episode, got %s", context[0].Header.Message.Episode)
	}
	// Rest should be from negotiation phase
	for i := 1; i < len(context); i++ {
		if context[i].Header.Message.Episode != "negotiation" {
			t.Errorf("Expected negotiation context at index %d, got episode %s", i, context[i].Header.Message.Episode)
		}
	}
}

// TestWholeConversation_LongRunningDialogue tests a large conversation
func TestWholeConversation_LongRunningDialogue(t *testing.T) {
	cache := New()

	// Create a 50-message dialogue
	const numMessages = 50
	var prevID string

	for i := 0; i < numMessages; i++ {
		msgID := fmt.Sprintf("msg-%d", i)
		var parents []string
		if i > 0 {
			parents = []string{prevID}
		}

		// Change episodes every 10 messages
		episode := fmt.Sprintf("phase-%d", i/10)

		msg := createTestMessage(msgID, parents, episode, "agent1")
		err := cache.Add(msg)
		if err != nil {
			t.Fatalf("Add(msg-%d) failed: %v", i, err)
		}

		prevID = msgID
	}

	// Verify full conversation
	conv, err := cache.GetConversationByMessageID("msg-25")
	if err != nil {
		t.Fatalf("GetConversationByMessageID() failed: %v", err)
	}

	if len(conv) != numMessages {
		t.Errorf("Expected %d messages, got %d", numMessages, len(conv))
	}

	// Get context window at different points
	testCases := []struct {
		msgID    string
		n        int
		expected int
	}{
		{"msg-5", 3, 3},      // Last 3 ending at msg-5 (includes msg-5)
		{"msg-15", 10, 10},   // Last 10 ending at msg-15 (includes msg-15)
		{"msg-0", 5, 1},      // Root - only msg-0 itself (it's first)
		{"msg-49", 20, 20},   // Last message, last 20 messages
	}

	for _, tc := range testCases {
		context, err := cache.GetLastNBeforeMessage(tc.msgID, tc.n)
		if err != nil {
			t.Fatalf("GetLastNBeforeMessage(%s, %d) failed: %v", tc.msgID, tc.n, err)
		}

		if len(context) != tc.expected {
			t.Errorf("Message %s: expected %d context messages, got %d",
				tc.msgID, tc.expected, len(context))
		}
	}

	// Verify ListConversations shows 1 conversation with 50 messages
	convs := cache.ListConversations()
	if len(convs) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(convs))
	}

	if convs[0].MessageCount != numMessages {
		t.Errorf("Expected conversation with %d messages, got %d",
			numMessages, convs[0].MessageCount)
	}
}

// TestWholeConversation_FilterByEpisode demonstrates client-side episode filtering
func TestWholeConversation_FilterByEpisode(t *testing.T) {
	cache := New()

	// Create conversation with mixed episodes
	messages := []struct {
		id      string
		parents []string
		episode string
	}{
		{"msg1", []string{}, "planning"},
		{"msg2", []string{"msg1"}, "planning"},
		{"msg3", []string{"msg2"}, "execution"},
		{"msg4", []string{"msg3"}, "execution"},
		{"msg5", []string{"msg4"}, "execution"},
		{"msg6", []string{"msg5"}, "review"},
	}

	for _, m := range messages {
		cache.Add(createTestMessage(m.id, m.parents, m.episode, "agent1"))
	}

	// Get full conversation
	conv, _ := cache.GetConversationByMessageID("msg3")

	// Client-side filter for "execution" messages
	var executionMsgs []*l9.L9
	for _, msg := range conv {
		if msg.Header.Message.Episode == "execution" {
			executionMsgs = append(executionMsgs, msg)
		}
	}

	if len(executionMsgs) != 3 {
		t.Errorf("Expected 3 execution messages, got %d", len(executionMsgs))
	}

	// Verify they're the right ones
	expectedIDs := []string{"msg3", "msg4", "msg5"}
	for i, msg := range executionMsgs {
		if msg.Header.Message.ID != expectedIDs[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedIDs[i], msg.Header.Message.ID)
		}
	}

	// Client-side helper function for filtering
	filterByEpisode := func(msgs []*l9.L9, episode string) []*l9.L9 {
		var filtered []*l9.L9
		for _, msg := range msgs {
			if msg.Header.Message.Episode == episode {
				filtered = append(filtered, msg)
			}
		}
		return filtered
	}

	planningMsgs := filterByEpisode(conv, "planning")
	if len(planningMsgs) != 2 {
		t.Errorf("Expected 2 planning messages, got %d", len(planningMsgs))
	}

	reviewMsgs := filterByEpisode(conv, "review")
	if len(reviewMsgs) != 1 {
		t.Errorf("Expected 1 review message, got %d", len(reviewMsgs))
	}
}

func TestGetLastNBeforeMessage(t *testing.T) {
	cache := New()

	// Create a conversation chain: root -> msg2 -> msg3 -> msg4 -> msg5
	root := createTestMessage("root", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"root"}, "episode1", "sender1")
	msg3 := createTestMessage("msg3", []string{"msg2"}, "episode1", "sender1")
	msg4 := createTestMessage("msg4", []string{"msg3"}, "episode1", "sender1")
	msg5 := createTestMessage("msg5", []string{"msg4"}, "episode1", "sender1")

	cache.Add(root)
	cache.Add(msg2)
	cache.Add(msg3)
	cache.Add(msg4)
	cache.Add(msg5)

	// Get last 2 messages ending at msg4 (should be msg4 and msg3, in reverse order)
	before, err := cache.GetLastNBeforeMessage("msg4", 2)
	if err != nil {
		t.Fatalf("GetLastNBeforeMessage() failed: %v", err)
	}

	if len(before) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(before))
	}

	// Target message comes first (most recent), then previous
	if before[0].Header.Message.ID != "msg4" {
		t.Errorf("Expected first to be msg4 (target), got %s", before[0].Header.Message.ID)
	}
	if before[1].Header.Message.ID != "msg3" {
		t.Errorf("Expected second to be msg3, got %s", before[1].Header.Message.ID)
	}
}

func TestGetLastNBeforeMessage_MoreThanAvailable(t *testing.T) {
	cache := New()

	root := createTestMessage("root", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"root"}, "episode1", "sender1")
	msg3 := createTestMessage("msg3", []string{"msg2"}, "episode1", "sender1")

	cache.Add(root)
	cache.Add(msg2)
	cache.Add(msg3)

	// Request more than available (10) ending at msg2 (should get msg2 and root)
	before, err := cache.GetLastNBeforeMessage("msg2", 10)
	if err != nil {
		t.Fatalf("GetLastNBeforeMessage() failed: %v", err)
	}

	if len(before) != 2 {
		t.Errorf("Expected 2 messages (msg2 and root), got %d", len(before))
	}

	// Most recent first (msg2), then root
	if before[0].Header.Message.ID != "msg2" {
		t.Errorf("Expected msg2, got %s", before[0].Header.Message.ID)
	}
	if before[1].Header.Message.ID != "root" {
		t.Errorf("Expected root, got %s", before[1].Header.Message.ID)
	}
}

func TestGetLastNBeforeMessage_RootMessage(t *testing.T) {
	cache := New()

	root := createTestMessage("root", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"root"}, "episode1", "sender1")

	cache.Add(root)
	cache.Add(msg2)

	// Request messages ending at root (should return just root, since it's the first)
	before, err := cache.GetLastNBeforeMessage("root", 5)
	if err != nil {
		t.Fatalf("GetLastNBeforeMessage() failed: %v", err)
	}

	if len(before) != 1 {
		t.Errorf("Expected 1 message (root itself), got %d", len(before))
	}

	if before[0].Header.Message.ID != "root" {
		t.Errorf("Expected root, got %s", before[0].Header.Message.ID)
	}
}

func TestGetLastNBeforeMessage_NotFound(t *testing.T) {
	cache := New()

	_, err := cache.GetLastNBeforeMessage("nonexistent", 5)
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound, got %v", err)
	}
}

func TestListConversations(t *testing.T) {
	cache := New()

	root1 := createTestMessage("root1", []string{}, "episode1", "sender1")
	msg1a := createTestMessage("msg1a", []string{"root1"}, "episode1", "sender1")

	root2 := createTestMessage("root2", []string{}, "episode2", "sender2")

	cache.Add(root1)
	cache.Add(msg1a)
	cache.Add(root2)

	convs := cache.ListConversations()
	if len(convs) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(convs))
	}

	// Check metadata
	for _, conv := range convs {
		if conv.RootID == "root1" {
			if conv.MessageCount != 2 {
				t.Errorf("Expected root1 to have 2 messages, got %d", conv.MessageCount)
			}
		} else if conv.RootID == "root2" {
			if conv.MessageCount != 1 {
				t.Errorf("Expected root2 to have 1 message, got %d", conv.MessageCount)
			}
		}
	}
}


func TestEvictConversation(t *testing.T) {
	cache := New()

	root := createTestMessage("root", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"root"}, "episode1", "sender1")

	cache.Add(root)
	cache.Add(msg2)

	convs := cache.ListConversations()
	if len(convs) != 1 || convs[0].MessageCount != 2 {
		t.Errorf("Expected 1 conversation with 2 messages")
	}

	// Evict using any message in the conversation
	err := cache.EvictConversation("msg2")
	if err != nil {
		t.Fatalf("EvictConversation() failed: %v", err)
	}

	convs = cache.ListConversations()
	if len(convs) != 0 {
		t.Errorf("Expected 0 conversations after eviction, got %d", len(convs))
	}

	_, err = cache.GetConversationByMessageID("root")
	if err != ErrMessageNotFound {
		t.Errorf("Expected ErrMessageNotFound after eviction, got %v", err)
	}
}

func TestFIFOEviction(t *testing.T) {
	cache := New()

	// Fill cache to MaxConversations
	for i := 0; i < MaxConversations; i++ {
		msgID := fmt.Sprintf("conv%d-root", i)
		msg := createTestMessage(msgID, []string{}, "episode1", "sender1")
		if err := cache.Add(msg); err != nil {
			t.Fatalf("Failed to add message %d: %v", i, err)
		}
	}

	convs := cache.ListConversations()
	if len(convs) != MaxConversations {
		t.Fatalf("Expected %d conversations, got %d", MaxConversations, len(convs))
	}

	// Add one more - should evict oldest (conv0-root)
	newMsg := createTestMessage("conv-new", []string{}, "episode1", "sender1")
	if err := cache.Add(newMsg); err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	convs = cache.ListConversations()
	if len(convs) != MaxConversations {
		t.Errorf("Expected %d conversations after eviction, got %d", MaxConversations, len(convs))
	}

	// Oldest (conv0-root) should be evicted
	_, err := cache.Get("conv0-root")
	if err != ErrMessageNotFound {
		t.Errorf("Expected conv0-root to be evicted, but found it")
	}

	// Newest should exist
	_, err = cache.Get("conv-new")
	if err != nil {
		t.Errorf("Expected conv-new to exist, got error: %v", err)
	}

	// Second oldest (conv1-root) should still exist
	_, err = cache.Get("conv1-root")
	if err != nil {
		t.Errorf("Expected conv1-root to exist, got error: %v", err)
	}
}

func TestClear(t *testing.T) {
	cache := New()

	msg1 := createTestMessage("msg1", []string{}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"msg1"}, "episode1", "sender1")

	cache.Add(msg1)
	cache.Add(msg2)

	convs := cache.ListConversations()
	if len(convs) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(convs))
	}

	cache.Clear()

	convs = cache.ListConversations()
	if len(convs) != 0 {
		t.Errorf("Expected 0 conversations after Clear(), got %d", len(convs))
	}
}

func TestConcurrency(t *testing.T) {
	cache := New()
	const numGoroutines = 10
	const messagesPerGoroutine = 5 // Keep under MaxConversations

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes - each goroutine creates messages in separate conversations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msgID := fmt.Sprintf("msg-%d-%d", goroutineID, j)
				msg := createTestMessage(msgID, []string{}, "episode1", fmt.Sprintf("sender%d", goroutineID))
				err := cache.Add(msg)
				if err != nil {
					t.Errorf("Concurrent Add() failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	convs := cache.ListConversations()
	if len(convs) == 0 {
		t.Errorf("Expected conversations after concurrent adds, got 0")
	}

	// Concurrent reads - test that reads don't panic/race
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			msgID := fmt.Sprintf("msg-%d-0", goroutineID)
			// May fail if conversation was evicted, that's ok - just testing for races
			_, _ = cache.GetConversationByMessageID(msgID)
		}(i)
	}
	wg.Wait()
}

func TestCycleDetection(t *testing.T) {
	cache := New()

	// Create a cycle: msg1 -> msg2 -> msg1
	msg1 := createTestMessage("msg1", []string{"msg2"}, "episode1", "sender1")
	msg2 := createTestMessage("msg2", []string{"msg1"}, "episode1", "sender1")

	// Add msg2 first
	err := cache.Add(msg2)
	if err != nil {
		t.Fatalf("Add(msg2) failed: %v", err)
	}

	// Adding msg1 should detect the cycle
	err = cache.Add(msg1)
	if err == nil {
		t.Error("Expected cycle detection error, got nil")
	}
	if err != nil && err != ErrCycleDetected {
		t.Logf("Got error (expected cycle-related): %v", err)
	}
}


func BenchmarkAdd(b *testing.B) {
	cache := New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), []string{}, "episode1", "sender1")
		cache.Add(msg)
	}
}

func BenchmarkGet(b *testing.B) {
	cache := New()
	msg := createTestMessage("msg1", []string{}, "episode1", "sender1")
	cache.Add(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("msg1")
	}
}

func BenchmarkGetConversation(b *testing.B) {
	cache := New()
	root := createTestMessage("root", []string{}, "episode1", "sender1")
	cache.Add(root)

	for i := 0; i < 100; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), []string{"root"}, "episode1", "sender1")
		cache.Add(msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetConversationByMessageID("msg50")
	}
}

func BenchmarkConcurrentReads(b *testing.B) {
	cache := New()
	for i := 0; i < 1000; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), []string{}, "episode1", "sender1")
		cache.Add(msg)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.GetConversationByMessageID("msg500")
		}
	})
}
