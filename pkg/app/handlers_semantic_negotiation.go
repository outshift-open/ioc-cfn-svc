package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/semanticalignment"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
)

// startSemanticAlignmentHandler godoc
//
// @Summary     Start semantic alignment session
// @Description Initiates a new semantic alignment session with multiple agents.
//
// @Tags        semantic-alignment
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body semanticalignment.StartRequest true "Semantic negotiation start request"
//
// @Success     200 {object} semanticalignment.StartResponse "Negotiation session started successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/start [post]
func (a *App) startSemanticAlignmentHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Starting semantic alignment | workspace=%s mas=%s",
		workspaceID, masID,
	)

	operationID := uuid.New().String()

	var reqPayload semanticalignment.StartRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr("invalid JSON body"))

			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Validate required fields
	if reqPayload.SessionID == "" {
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr("session_id is required"))

		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "session_id is required"},
		)
	}
	if reqPayload.ContentText == "" {
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr("content_text is required"))

		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "content_text is required"},
		)
	}
	if len(reqPayload.Agents) == 0 {
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr("agents list cannot be empty"))

		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "agents list cannot be empty"},
		)
	}

	// Transform DTO to client request
	agents := make([]cognitionagentclient.SemanticAlignmentAgent, len(reqPayload.Agents))
	for i, agent := range reqPayload.Agents {
		agents[i] = cognitionagentclient.SemanticAlignmentAgent{
			ID:   agent.ID,
			Name: agent.Name,
		}
	}

	cogReq := &cognitionagentclient.SemanticAlignmentStartRequest{
		SessionID:   reqPayload.SessionID,
		ContentText: reqPayload.ContentText,
		Agents:      agents,
		NSteps:      reqPayload.NSteps,
	}

	cogResp, err := a.cognitionAgentsClient.SendSemanticAlignmentStart(r.Context(), cogReq, workspaceID, masID)
	if err != nil {
		log.Errorf("failed to start semantic alignment, error: %s", err.Error())

		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr(err.Error()))

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to start semantic alignment"},
		)
	}

	// The initiate endpoint returns an SSTPNegotiateMessage envelope whose
	// payload is an InitiateResponse. Re-marshal the payload and decode into
	// our typed StartResponse so callers get a clean, structured response.
	resp, err := mapInitiatePayloadToStartResponse(cogResp)
	if err != nil {
		log.Errorf("failed to map initiate response | workspace=%s mas=%s err=%v", workspaceID, masID, err)
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "FAILED", common.StrToPtr(err.Error()))
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to parse negotiation response"})
	}

	// Store token metrics to TimescaleDB (fire-and-forget)
	if cogResp.Meta != nil {
		workspaceUUID, _ := uuid.Parse(workspaceID)
		masUUID, _ := uuid.Parse(masID)
		agentID := "unknown"
		if len(reqPayload.Agents) > 0 {
			agentID = reqPayload.Agents[0].ID
		}
		// Extract CE ID from response metadata
		var ceID *uuid.UUID
		if cogResp.Meta.CEID != "" {
			if parsed, err := uuid.Parse(cogResp.Meta.CEID); err == nil {
				ceID = &parsed
			} else {
				log.Warnf("Invalid CE ID in response metadata: %s", cogResp.Meta.CEID)
			}
		}
		a.storeTokenMetricsAsync(
			workspaceUUID,
			masUUID,
			agentID,
			"semantic_alignment",
			reqPayload.SessionID,
			ceID,
			&common.TokenUsageMeta{
				Tokens: common.TokenUsage{
					Prompt:     cogResp.Meta.Tokens.Prompt,
					Completion: cogResp.Meta.Tokens.Completion,
					Total:      cogResp.Meta.Tokens.Total,
					Model:      cogResp.Meta.Tokens.Model,
				},
				LatencyMs: cogResp.Meta.LatencyMs,
				CostUsd:   cogResp.Meta.CostUsd,
				Timestamp: cogResp.Meta.Timestamp,
			},
		)
	}

	a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentStart, "SUCCESS", nil)

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

// decideSemanticAlignmentHandler godoc
//
// @Summary     Advance semantic alignment session
// @Description Advances an existing semantic alignment session with agent replies.
//
// @Tags        semantic-alignment
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body semanticalignment.DecideRequest true "Semantic negotiation decide request"
//
// @Success     200 {object} semanticalignment.DecideResponse "Negotiation step executed successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Session not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/decide [post]
func (a *App) decideSemanticAlignmentHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Advancing semantic alignment | workspace=%s mas=%s",
		workspaceID, masID,
	)

	operationID := uuid.New().String()

	var reqPayload semanticalignment.DecideRequest
	if r.Body != nil {
		defer r.Body.Close()

		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr("invalid JSON body"))

			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Validate required fields
	if reqPayload.SessionID == "" {
		errMsg := "session_id is required"
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr(errMsg))
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": errMsg},
		)
	}
	if len(reqPayload.AgentReplies) == 0 {
		errMsg := "agent_replies list cannot be empty"
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr(errMsg))

		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": errMsg},
		)
	}

	agentReplies, err := buildAgentReplyEnvelopes(reqPayload.SessionID, reqPayload.AgentReplies)
	if err != nil {
		errMsg := "failed to build agent reply envelopes"
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr(errMsg))
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
	}

	cogReq := &cognitionagentclient.SemanticAlignmentDecideRequest{
		SessionID:    reqPayload.SessionID,
		AgentReplies: agentReplies,
	}

	cogResp, err := a.cognitionAgentsClient.SendSemanticAlignmentDecide(r.Context(), cogReq, workspaceID, masID)
	if err != nil {
		log.Errorf("failed to advance semantic alignment, error: %s", err.Error())
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr(err.Error()))

		if errors.Is(err, cognitionagentclient.ErrNotFound) {
			return eh.RespondWithJSON(
				w,
				http.StatusNotFound,
				map[string]string{"error": fmt.Sprintf("session %q not found", reqPayload.SessionID)},
			)
		}
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to advance semantic alignment"},
		)
	}

	// The decide endpoint returns a flat JSON object (not an SSTP envelope).
	round := 0
	if cogResp.Round != nil {
		round = *cogResp.Round
	}
	resp := &semanticalignment.DecideResponse{
		Status:      cogResp.Status,
		SessionID:   cogResp.SessionID,
		Round:       round,
		Messages:    cogResp.Messages,
		FinalResult: cogResp.FinalResult,
	}

	// Pass through token metadata from cognition agent if available
	if cogResp.Meta != nil {
		resp.Meta = &common.TokenUsageMeta{
			Tokens: common.TokenUsage{
				Prompt:     cogResp.Meta.Tokens.Prompt,
				Completion: cogResp.Meta.Tokens.Completion,
				Total:      cogResp.Meta.Tokens.Total,
				Model:      cogResp.Meta.Tokens.Model,
			},
			LatencyMs: cogResp.Meta.LatencyMs,
			CostUsd:   cogResp.Meta.CostUsd,
			Timestamp: cogResp.Meta.Timestamp,
		}
	}

	// If agreement is reached, persist the final result to shared memory.
	if cogResp.Status == "agreed" && len(cogResp.FinalResult) > 0 {
		log.Infof("agreement has been reached, final result is being persisted to the shared memory")
		// Wrap FinalResult in an array to match the extraction endpoint's `data: list[dict]` schema.
		finalResultJSON, err := json.Marshal([]map[string]interface{}{cogResp.FinalResult})
		if err != nil {
			log.Errorf("failed to marshal final_result for persistence | workspace=%s mas=%s session=%s err=%v",
				workspaceID, masID, reqPayload.SessionID, err)
			a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr("failed to marshall final result for persistence"))
			resp.SharedMemory = &semanticalignment.SharedMemoryResult{
				Persisted: false,
				Error:     "failed to marshal final result for persistence",
			}
		} else {
			persistReq := sharedmemory.CreateOrUpdateRequest{
				RequestId: common.StrToPtr(reqPayload.SessionID),
				Payload: cognitionagentclient.ExtractionPayload{
					Metadata: cognitionagentclient.ExtractionPayloadMetadata{
						Format: common.FormatSemNeg,
					},
					Data: json.RawMessage(finalResultJSON),
				},
			}
			if _, err := a.createOrUpdateSharedMemoriesCore(context.Background(), workspaceID, masID, persistReq); err != nil {
				log.Errorf("failed to persist negotiation agreement | workspace=%s mas=%s session=%s err=%v",
					workspaceID, masID, reqPayload.SessionID, err)
				a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "FAILED", common.StrToPtr("failed to persist negotiation agreement"))
				resp.SharedMemory = &semanticalignment.SharedMemoryResult{
					Persisted: false,
					Error:     "failed to persist negotiation agreement to shared memory",
				}
			} else {
				log.Infof("persisted negotiation agreement to shared memory | workspace=%s mas=%s session=%s",
					workspaceID, masID, reqPayload.SessionID)
				resp.SharedMemory = &semanticalignment.SharedMemoryResult{
					Persisted: true,
				}
			}
		}
	}

	a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSemanticAlignmentDecide, "SUCCESS", nil)
	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

// initiateRaw is the union of both response shapes from /negotiate/initiate:
// - SSTP envelope path: payload contains InitiateResponse fields (current_round, trace, etc.)
// - async_execute path (via Python CFN): flat dict with status, session_id, issues, messages, etc.
type initiateRaw struct {
	// Shared
	Status    string `json:"status"`
	SessionID string `json:"session_id"`

	// SSTP envelope path (InitiateResponse)
	TotalRounds  int `json:"total_rounds"`
	CurrentRound *struct {
		Round          int               `json:"round"`
		ProposerID     string            `json:"proposer_id"`
		NextProposerID *string           `json:"next_proposer_id"`
		Offer          map[string]string `json:"offer"`
		Decisions      []struct {
			ParticipantID string            `json:"participant_id"`
			Action        string            `json:"action"`
			Offer         map[string]string `json:"offer,omitempty"`
		} `json:"decisions"`
	} `json:"current_round"`
	Trace *struct {
		Rounds []struct {
			Round          int               `json:"round"`
			ProposerID     string            `json:"proposer_id"`
			NextProposerID *string           `json:"next_proposer_id"`
			Offer          map[string]string `json:"offer"`
			Decisions      []struct {
				ParticipantID string            `json:"participant_id"`
				Action        string            `json:"action"`
				Offer         map[string]string `json:"offer,omitempty"`
			} `json:"decisions"`
		} `json:"rounds"`
		FinalAgreement []struct {
			IssueID      string `json:"issue_id"`
			ChosenOption string `json:"chosen_option"`
		} `json:"final_agreement"`
		Timedout bool `json:"timedout"`
		Broken   bool `json:"broken"`
	} `json:"trace"`

	// async_execute path
	Issues          []string            `json:"issues"`
	OptionsPerIssue map[string][]string `json:"options_per_issue"`
	NSteps          int                 `json:"n_steps"`
	Round           int                 `json:"round"`
	Messages        []json.RawMessage   `json:"messages"`
}

// mapInitiatePayloadToStartResponse maps the initiate response from cogResp into
// a typed StartResponse. It handles two response shapes:
//   - SSTP envelope: cogResp.Payload contains InitiateResponse fields (current_round, trace, etc.)
//   - Flat dict: cogResp carries status/session_id/issues/messages directly (async_execute path)
func mapInitiatePayloadToStartResponse(cogResp *cognitionagentclient.SemanticAlignmentResponse) (*semanticalignment.StartResponse, error) {
	// Prefer the SSTP envelope payload if populated, otherwise fall back to the
	// top-level flat fields on cogResp.
	source := cogResp.Payload
	if len(source) == 0 {
		// Build a map from the flat fields so we can use a single decode path.
		b, err := json.Marshal(cogResp)
		if err != nil {
			return nil, fmt.Errorf("marshal cogResp: %w", err)
		}
		if err := json.Unmarshal(b, &source); err != nil {
			return nil, fmt.Errorf("unmarshal cogResp to map: %w", err)
		}
	}

	b, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal initiate source: %w", err)
	}

	var raw initiateRaw
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal initiate payload: %w", err)
	}

	resp := &semanticalignment.StartResponse{
		Status:          raw.Status,
		SessionID:       raw.SessionID,
		TotalRounds:     raw.TotalRounds,
		Issues:          raw.Issues,
		OptionsPerIssue: raw.OptionsPerIssue,
		NSteps:          raw.NSteps,
		Round:           raw.Round,
		Messages:        raw.Messages,
	}

	// Pass through token metadata from cognition agent if available
	if cogResp.Meta != nil {
		resp.Meta = &common.TokenUsageMeta{
			Tokens: common.TokenUsage{
				Prompt:     cogResp.Meta.Tokens.Prompt,
				Completion: cogResp.Meta.Tokens.Completion,
				Total:      cogResp.Meta.Tokens.Total,
				Model:      cogResp.Meta.Tokens.Model,
			},
			LatencyMs: cogResp.Meta.LatencyMs,
			CostUsd:   cogResp.Meta.CostUsd,
			Timestamp: cogResp.Meta.Timestamp,
		}
	}

	if raw.CurrentRound != nil {
		cr := &semanticalignment.RoundOffer{
			Round:          raw.CurrentRound.Round,
			ProposerID:     raw.CurrentRound.ProposerID,
			NextProposerID: raw.CurrentRound.NextProposerID,
			Offer:          raw.CurrentRound.Offer,
		}
		for _, d := range raw.CurrentRound.Decisions {
			cr.Decisions = append(cr.Decisions, semanticalignment.AgentDecision{
				ParticipantID: d.ParticipantID,
				Action:        d.Action,
				Offer:         d.Offer,
			})
		}
		resp.CurrentRound = cr
	}

	if raw.Trace != nil {
		trace := &semanticalignment.NegotiationTrace{
			Timedout: raw.Trace.Timedout,
			Broken:   raw.Trace.Broken,
		}
		for _, r := range raw.Trace.Rounds {
			ro := semanticalignment.RoundOffer{
				Round:          r.Round,
				ProposerID:     r.ProposerID,
				NextProposerID: r.NextProposerID,
				Offer:          r.Offer,
			}
			for _, d := range r.Decisions {
				ro.Decisions = append(ro.Decisions, semanticalignment.AgentDecision{
					ParticipantID: d.ParticipantID,
					Action:        d.Action,
					Offer:         d.Offer,
				})
			}
			trace.Rounds = append(trace.Rounds, ro)
		}
		for _, fa := range raw.Trace.FinalAgreement {
			trace.FinalAgreement = append(trace.FinalAgreement, semanticalignment.NegotiationOutcome{
				IssueID:      fa.IssueID,
				ChosenOption: fa.ChosenOption,
			})
		}
		resp.Trace = trace
	}

	return resp, nil
}

// buildAgentReplyEnvelopes wraps each AgentReply in a minimal SSTP envelope
// so the downstream negotiation service receives the expected wire format.
func buildAgentReplyEnvelopes(sessionID string, replies []semanticalignment.AgentReply) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(replies))
	for _, reply := range replies {
		payload := map[string]interface{}{
			"participant_id": reply.ParticipantID,
			"action":         reply.Action,
		}
		if len(reply.Offer) > 0 {
			payload["offer"] = reply.Offer
		}
		envelope := map[string]interface{}{
			"kind":       "negotiate",
			"version":    "0",
			"message_id": sessionID,
			"semantic_context": map[string]interface{}{
				"session_id": sessionID,
			},
			"payload": payload,
		}
		b, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("marshal agent reply for participant %q: %w", reply.ParticipantID, err)
		}
		out = append(out, json.RawMessage(b))
	}
	return out, nil
}
