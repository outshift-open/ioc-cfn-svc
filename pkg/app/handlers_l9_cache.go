// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"net/http"
	"strconv"

	l9cache "github.com/outshift-open/ioc-cfn-svc/pkg/cache/l9"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

// cacheL9Message attempts to cache an L9 message in the appropriate workspace+MAS cache.
// Extracts workspaceID and masID from the message participants.groups.
// Failures are logged but don't block processing.
func (a *App) cacheL9Message(msg *l9.L9) {
	if msg == nil {
		return
	}

	log := getLogger()

	// Fail fast if message ID is missing
	if msg.Header.Message == nil || msg.Header.Message.ID == "" {
		log.Warnf("skipping cache: message ID is missing")
		return
	}

	msgID := msg.Header.Message.ID

	// Extract workspace and MAS IDs from message
	routingInfo, err := extractRoutingInfo(msg)
	if err != nil {
		log.Warnf("failed to extract routing info for caching message %s: %v", msgID, err)
		return
	}

	// Get the cache for this workspace+MAS
	cache := a.getCacheForWorkspaceMAS(routingInfo.workspaceID, routingInfo.masID)

	if err := cache.Add(msg); err != nil {
		log.Warnf("failed to cache message %s in %s:%s: %v",
			msgID, routingInfo.workspaceID, routingInfo.masID, err)
	} else {
		log.Debugf("cached message %s in %s:%s",
			msgID, routingInfo.workspaceID, routingInfo.masID)
	}
}

// l9CacheHandler handles L9 message cache queries for a specific workspace+MAS.
// Behavior depends on query parameters:
// - No params: list all conversations
// - ?msgID=xxx: get full conversation
// - ?msgID=xxx&n=10: get last N messages ending at msgID (includes target)
//
// @Summary Query L9 message cache for a workspace+MAS
// @Description List conversations, get conversation by message ID, or get last N messages ending at message ID
// @Tags l9-cache
// @Produce json
// @Param ceid path string true "Cognition Engine ID"
// @Param wsId path string true "Workspace ID"
// @Param masId path string true "Multi-Agentic System ID"
// @Param msgID query string false "Message ID to query conversation or context"
// @Param n query int false "Number of messages to retrieve ending at msgID (includes target, max 1000)"
// @Success 200 {object} map[string]interface{} "Conversation list, full conversation, or context messages"
// @Failure 400 {object} map[string]string "Invalid query parameters"
// @Failure 404 {object} map[string]string "Message or conversation not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/cognition-engines/{ceId}/l9-cache/workspaces/{wsId}/multi-agentic-systems/{masId} [get]
func (a *App) l9CacheHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	// Extract path parameters
	ceID := eh.PathParam(r, "ceId")
	workspaceID := eh.PathParam(r, "wsId")
	masID := eh.PathParam(r, "masId")

	if ceID == "" || workspaceID == "" || masID == "" {
		log.Error("Missing required path parameters")
		return respondError(w, http.StatusBadRequest, "CE ID, workspace ID, and MAS ID are required")
	}

	// Validate CE exists in configuration
	cfnConfigMutex.RLock()
	if ParsedConfig == nil {
		cfnConfigMutex.RUnlock()
		log.Error("CFN configuration not available")
		return respondError(w, http.StatusInternalServerError, "configuration not available")
	}
	ce := ParsedConfig.FindCE(ceID)
	cfnConfigMutex.RUnlock()

	if ce == nil {
		log.Warnf("CE %s not found in configuration", ceID)
		return respondError(w, http.StatusNotFound, "cognition engine not found")
	}

	log.Infof("CE %s (%s) querying L9 cache for workspace=%s, mas=%s",
		ceID, ce.Name, workspaceID, masID)

	// Get the cache for this workspace+MAS
	cache := a.getCacheForWorkspaceMAS(workspaceID, masID)

	msgID := r.URL.Query().Get("msgID")
	nStr := r.URL.Query().Get("n")

	// Case 1: No params - list all conversations
	if msgID == "" {
		return a.listConversationsHandler(w, r, cache, workspaceID, masID)
	}

	// Case 2: msgID + n - get last N messages (ending at msgID)
	if nStr != "" {
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			log.Errorf("Invalid n parameter: %s", nStr)
			return respondError(w, http.StatusBadRequest, "n must be a positive integer")
		}
		if n > 1000 {
			log.Warnf("n parameter too large: %d, capping at 1000", n)
			n = 1000
		}
		return a.getPreviousNHandler(w, r, cache, workspaceID, masID, msgID, n)
	}

	// Case 3: msgID only - get full conversation
	return a.getConversationHandler(w, r, cache, workspaceID, masID, msgID)
}

// listConversationsHandler returns metadata for all cached conversations in a workspace+MAS.
func (a *App) listConversationsHandler(w http.ResponseWriter, r *http.Request,
	cache *l9cache.MessageCache, workspaceID, masID string) (int, error) {
	log := getLogger()

	convs := cache.ListConversations()
	log.Infof("Listed %d conversations from cache for %s:%s", len(convs), workspaceID, masID)

	return eh.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id":  workspaceID,
		"mas_id":        masID,
		"conversations": convs,
	})
}

// getConversationHandler retrieves all messages in the conversation containing the specified message.
func (a *App) getConversationHandler(w http.ResponseWriter, r *http.Request,
	cache *l9cache.MessageCache, workspaceID, masID, msgID string) (int, error) {
	log := getLogger()

	msgs, err := cache.GetConversationByMessageID(msgID)
	if err != nil {
		if errors.Is(err, l9cache.ErrMessageNotFound) {
			log.Warnf("Message not found in cache: %s in %s:%s", msgID, workspaceID, masID)
			return respondError(w, http.StatusNotFound, "message not found in cache")
		}
		log.Errorf("Failed to get conversation: %v", err)
		return respondError(w, http.StatusInternalServerError, "failed to retrieve conversation")
	}

	// Extract session ID from first message
	// Session ID = Conversation ID = Root Message ID (the first message with no parents)
	// This uniquely identifies the entire conversation/session across all messages
	var sessionID string
	if len(msgs) > 0 && msgs[0].Header.Message != nil {
		sessionID = msgs[0].Header.Message.ID
	}

	// Extract unique participants from all messages in the conversation
	participants := extractParticipants(msgs)

	log.Infof("Retrieved conversation for msgID=%s in %s:%s, session=%s, count=%d, participants=%d",
		msgID, workspaceID, masID, sessionID, len(msgs), len(participants))

	return eh.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": workspaceID,
		"mas_id":       masID,
		"session_id":   sessionID,
		"participants": participants,
		"messages":     msgs,
	})
}

// getPreviousNHandler retrieves up to N messages ending at the specified message (including the target).
func (a *App) getPreviousNHandler(w http.ResponseWriter, r *http.Request,
	cache *l9cache.MessageCache, workspaceID, masID, msgID string, n int) (int, error) {
	log := getLogger()

	msgs, err := cache.GetLastNBeforeMessage(msgID, n)
	if err != nil {
		if errors.Is(err, l9cache.ErrMessageNotFound) {
			log.Warnf("Message not found in cache: %s in %s:%s", msgID, workspaceID, masID)
			return respondError(w, http.StatusNotFound, "message not found in cache")
		}
		log.Errorf("Failed to get previous messages: %v", err)
		return respondError(w, http.StatusInternalServerError, "failed to retrieve context")
	}

	// Get full conversation to extract session ID and all participants
	fullConv, err := cache.GetConversationByMessageID(msgID)
	if err != nil {
		log.Errorf("Failed to get full conversation for session/participants: %v", err)
		return respondError(w, http.StatusInternalServerError, "failed to retrieve conversation metadata")
	}

	// Extract session ID from first message in full conversation
	var sessionID string
	if len(fullConv) > 0 && fullConv[0].Header.Message != nil {
		sessionID = fullConv[0].Header.Message.ID
	}

	// Extract participants from entire conversation, not just the N messages
	participants := extractParticipants(fullConv)

	log.Infof("Retrieved %d messages (up to %d ending at msgID=%s) in %s:%s, session=%s, participants=%d",
		len(msgs), n, msgID, workspaceID, masID, sessionID, len(participants))

	return eh.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": workspaceID,
		"mas_id":       masID,
		"session_id":   sessionID,
		"participants": participants,
		"messages":     msgs,
	})
}

// extractParticipants extracts unique participants from a list of L9 messages.
// Returns a list of participants with their actor IDs and all roles they played
// across the given messages.
func extractParticipants(msgs []*l9.L9) []map[string]interface{} {
	// Track unique participants: actor ID -> set of roles
	participantRoles := make(map[string]map[string]struct{})

	for _, msg := range msgs {
		if msg.Header.Participants.Actors == nil {
			continue
		}

		for _, actor := range msg.Header.Participants.Actors {
			if actor.ID == "" {
				continue
			}

			// Initialize role set for new actor
			if participantRoles[actor.ID] == nil {
				participantRoles[actor.ID] = make(map[string]struct{})
			}

			// Add this role to the actor's set
			if actor.Role != "" {
				participantRoles[actor.ID][actor.Role] = struct{}{}
			}
		}
	}

	// Convert to response format
	result := make([]map[string]interface{}, 0, len(participantRoles))
	for actorID, roleSet := range participantRoles {
		// Convert role set to sorted slice for consistent output
		roles := make([]string, 0, len(roleSet))
		for role := range roleSet {
			roles = append(roles, role)
		}

		result = append(result, map[string]interface{}{
			"id":    actorID,
			"roles": roles,
		})
	}

	return result
}
