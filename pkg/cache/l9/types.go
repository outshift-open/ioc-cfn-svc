// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package l9cache

// ConversationInfo contains metadata about a conversation (session).
type ConversationInfo struct {
	// SessionID is the conversation ID (root message ID - the first message with no parents).
	// This uniquely identifies the entire conversation/session.
	SessionID string `json:"session_id"`
}
