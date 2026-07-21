// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package l9cache

import (
	"errors"
	"fmt"
	"sync"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

const (
	// MaxConversations is the maximum number of conversations to cache per workspace+MAS.
	// When exceeded, the oldest conversation is evicted (FIFO).
	MaxConversations = 1000
)

var (
	// ErrMessageNotFound is returned when a message ID doesn't exist in the cache.
	ErrMessageNotFound = errors.New("message not found")

	// ErrConversationNotFound is returned when a conversation root doesn't exist.
	ErrConversationNotFound = errors.New("conversation not found")

	// ErrInvalidMessage is returned when a message is malformed or missing required fields.
	ErrInvalidMessage = errors.New("invalid message")

	// ErrCycleDetected is returned when a parent cycle is detected in the message graph.
	ErrCycleDetected = errors.New("cycle detected in parent chain")
)

// MessageCache is a thread-safe in-memory cache for L9 protocol messages organized by conversation.
// Messages are linked through parent-child relationships, with each conversation having a root message.
//
// The cache uses FIFO eviction: when MaxConversations (1000) is exceeded, the oldest conversation is removed.
type MessageCache struct {
	mu sync.RWMutex

	// Core message storage
	messages map[string]*l9.L9 // message ID -> message

	// Conversation indexing
	conversations map[string][]string // session ID -> list of message IDs in insertion order
	msgToRoot     map[string]string   // message ID -> session ID

	// FIFO eviction
	conversationOrder []string // session IDs in creation order (oldest first)
}

// New creates a new MessageCache instance with FIFO eviction.
// The cache holds up to MaxConversations (100). When exceeded, the oldest is evicted.
func New() *MessageCache {
	return &MessageCache{
		messages:          make(map[string]*l9.L9),
		conversations:     make(map[string][]string),
		msgToRoot:         make(map[string]string),
		conversationOrder: make([]string, 0),
	}
}

// Add inserts an L9 message into the cache. It automatically indexes the message
// by conversation. Returns an error if the message is invalid or if a cycle is
// detected in the parent chain. If a message with the same ID already exists,
// it will be silently replaced.
func (c *MessageCache) Add(msg *l9.L9) error {
	if msg == nil {
		return ErrInvalidMessage
	}

	if msg.Header.Message == nil || msg.Header.Message.ID == "" {
		return fmt.Errorf("%w: missing message ID", ErrInvalidMessage)
	}

	msgID := msg.Header.Message.ID

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if message already exists
	if _, exists := c.messages[msgID]; exists {
		// Already cached, update in place without adding to conversation again
		c.messages[msgID] = msg
		return nil
	}

	// Store the message
	c.messages[msgID] = msg

	// Find the conversation root
	sessionID, err := c.findRootLocked(msgID)
	if err != nil {
		// Clean up on error
		delete(c.messages, msgID)
		return err
	}

	// Check if this is a new conversation
	isNewConversation := len(c.conversations[sessionID]) == 0

	// Index by conversation
	c.conversations[sessionID] = append(c.conversations[sessionID], msgID)
	c.msgToRoot[msgID] = sessionID

	// Track conversation order for FIFO eviction
	if isNewConversation {
		c.conversationOrder = append(c.conversationOrder, sessionID)

		// Evict oldest conversation if limit exceeded
		if len(c.conversations) > MaxConversations {
			oldestRootID := c.conversationOrder[0]
			_ = c.evictConversationLocked(oldestRootID) // Ignore error, best effort
		}
	}

	return nil
}

// Get retrieves a message by ID. Returns ErrMessageNotFound if not cached.
func (c *MessageCache) Get(msgID string) (*l9.L9, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msg, ok := c.messages[msgID]
	if !ok {
		return nil, ErrMessageNotFound
	}
	return msg, nil
}

// GetConversationByMessageID retrieves all messages in the conversation that
// contains the specified message, ordered by insertion.
// Returns ErrMessageNotFound if the message doesn't exist.
func (c *MessageCache) GetConversationByMessageID(msgID string) ([]*l9.L9, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find which conversation this message belongs to
	sessionID, ok := c.msgToRoot[msgID]
	if !ok {
		return nil, ErrMessageNotFound
	}

	msgIDs, ok := c.conversations[sessionID]
	if !ok {
		// This shouldn't happen if msgToRoot is consistent, but handle gracefully
		return nil, ErrMessageNotFound
	}

	messages := make([]*l9.L9, 0, len(msgIDs))
	for _, id := range msgIDs {
		if msg, ok := c.messages[id]; ok {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// GetLastNBeforeMessage retrieves up to N messages ending at the specified message,
// including the target message itself. Messages are returned in reverse order (most recent first).
// This is useful for getting context around a specific message.
func (c *MessageCache) GetLastNBeforeMessage(msgID string, n int) ([]*l9.L9, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find which conversation this message belongs to
	sessionID, ok := c.msgToRoot[msgID]
	if !ok {
		return nil, ErrMessageNotFound
	}

	msgIDs, ok := c.conversations[sessionID]
	if !ok {
		// This shouldn't happen if msgToRoot is consistent, but handle gracefully
		return nil, ErrMessageNotFound
	}

	// Find the index of the target message
	targetIdx := -1
	for i, id := range msgIDs {
		if id == msgID {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		return nil, ErrMessageNotFound
	}

	// Get up to N messages ending at (and including) this one
	start := targetIdx - n + 1
	if start < 0 {
		start = 0
	}

	messages := make([]*l9.L9, 0, n)
	// Reverse order - most recent first (target message first)
	for i := targetIdx; i >= start; i-- {
		if msg, ok := c.messages[msgIDs[i]]; ok {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}


// ListConversations returns a list of all conversation roots with metadata.
func (c *MessageCache) ListConversations() []ConversationInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos := make([]ConversationInfo, 0, len(c.conversations))
	for sessionID, msgIDs := range c.conversations {
		if len(msgIDs) == 0 {
			continue
		}

		infos = append(infos, ConversationInfo{
			SessionID: sessionID,
		})
	}

	return infos
}

// EvictConversation removes all messages in the conversation containing the
// specified message from the cache. Returns ErrMessageNotFound if the message
// doesn't exist.
func (c *MessageCache) EvictConversation(msgID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find which conversation this message belongs to
	sessionID, ok := c.msgToRoot[msgID]
	if !ok {
		return ErrMessageNotFound
	}

	return c.evictConversationLocked(sessionID)
}

// evictConversationLocked removes all messages in a conversation by root ID.
// Must be called with lock held.
func (c *MessageCache) evictConversationLocked(sessionID string) error {
	msgIDs, ok := c.conversations[sessionID]
	if !ok {
		return ErrConversationNotFound
	}

	// Remove all messages in the conversation
	for _, id := range msgIDs {
		c.evictMessageLocked(id)
	}

	// Remove conversation
	delete(c.conversations, sessionID)

	// Remove from conversation order
	for i, id := range c.conversationOrder {
		if id == sessionID {
			// Remove by slicing around it
			copy(c.conversationOrder[i:], c.conversationOrder[i+1:])
			c.conversationOrder = c.conversationOrder[:len(c.conversationOrder)-1]
			break
		}
	}

	return nil
}

// Clear removes all messages from the cache.
func (c *MessageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = make(map[string]*l9.L9)
	c.conversations = make(map[string][]string)
	c.msgToRoot = make(map[string]string)
	c.conversationOrder = make([]string, 0)
}

// findRootLocked recursively walks the parent chain to find the conversation root.
// A root is a message with an empty parents array.
// Must be called with lock held.
func (c *MessageCache) findRootLocked(msgID string) (string, error) {
	visited := make(map[string]struct{})
	return c.findRootRecursiveLocked(msgID, visited)
}

func (c *MessageCache) findRootRecursiveLocked(msgID string, visited map[string]struct{}) (string, error) {
	// Cycle detection
	if _, ok := visited[msgID]; ok {
		return "", fmt.Errorf("%w: message %s", ErrCycleDetected, msgID)
	}
	visited[msgID] = struct{}{}

	msg, ok := c.messages[msgID]
	if !ok {
		// If not in cache yet, assume it's the root
		return msgID, nil
	}

	if msg.Header.Message == nil || len(msg.Header.Message.Parents) == 0 {
		// Found the root
		return msgID, nil
	}

	// Follow first parent (Python implementation does this)
	parentID := msg.Header.Message.Parents[0]
	return c.findRootRecursiveLocked(parentID, visited)
}

// evictMessageLocked removes a single message and updates all indices.
// Must be called with lock held.
func (c *MessageCache) evictMessageLocked(msgID string) {
	if _, ok := c.messages[msgID]; !ok {
		return
	}

	// Remove from messages
	delete(c.messages, msgID)

	// Remove from conversation index
	sessionID, ok := c.msgToRoot[msgID]
	if !ok {
		return
	}

	msgList, ok := c.conversations[sessionID]
	if !ok {
		delete(c.msgToRoot, msgID)
		return
	}

	// Find and remove from slice, preserving order
	for i, id := range msgList {
		if id == msgID {
			// Remove by slicing around it (preserves order)
			copy(msgList[i:], msgList[i+1:])
			msgList = msgList[:len(msgList)-1]

			if len(msgList) > 0 {
				c.conversations[sessionID] = msgList
			} else {
				delete(c.conversations, sessionID)
			}
			break
		}
	}

	delete(c.msgToRoot, msgID)
}
