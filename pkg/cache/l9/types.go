// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package l9cache

// ConversationInfo contains metadata about a conversation.
type ConversationInfo struct {
	RootID       string `json:"root_id"`
	MessageCount int    `json:"message_count"`
}
