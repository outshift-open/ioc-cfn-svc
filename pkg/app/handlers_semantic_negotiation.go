package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/semanticnegotiation"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// startSemanticNegotiationHandler godoc
//
// @Summary     Start semantic negotiation session
// @Description Initiates a new semantic negotiation session with multiple agents.
//
// @Tags        semantic-negotiation
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body semanticnegotiation.StartRequest true "Semantic negotiation start request"
//
// @Success     200 {object} semanticnegotiation.Response "Negotiation session started successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/start [post]
func (a *App) startSemanticNegotiationHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Starting semantic negotiation | workspace=%s mas=%s",
		workspaceID, masID,
	)

	var reqPayload semanticnegotiation.StartRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Validate required fields
	if reqPayload.SessionID == "" {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "session_id is required"},
		)
	}
	if reqPayload.ContentText == "" {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "content_text is required"},
		)
	}
	if len(reqPayload.Agents) == 0 {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "agents list cannot be empty"},
		)
	}

	// Transform DTO to client request
	agents := make([]cognitionagentclient.SemanticNegotiationAgent, len(reqPayload.Agents))
	for i, agent := range reqPayload.Agents {
		agents[i] = cognitionagentclient.SemanticNegotiationAgent{
			ID:   agent.ID,
			Name: agent.Name,
		}
	}

	cogReq := &cognitionagentclient.SemanticNegotiationStartRequest{
		SessionID:   reqPayload.SessionID,
		ContentText: reqPayload.ContentText,
		Agents:      agents,
		NSteps:      reqPayload.NSteps,
	}

	cogResp, err := a.cognitionAgentsClient.SendSemanticNegotiationStart(r.Context(), cogReq, workspaceID, masID)
	if err != nil {
		log.Errorf("failed to start semantic negotiation, error: %s", err.Error())
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to start semantic negotiation"},
		)
	}

	// The initiate endpoint returns an SSTPNegotiateMessage envelope.
	// Extract status from payload for convenience and surface the full envelope.
	status := ""
	if cogResp.Payload != nil {
		if s, ok := cogResp.Payload["status"].(string); ok {
			status = s
		}
	}

	resp := &semanticnegotiation.Response{
		Status:   status,
		Envelope: cogResp.Payload,
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

// decideSemanticNegotiationHandler godoc
//
// @Summary     Advance semantic negotiation session
// @Description Advances an existing semantic negotiation session with agent replies.
//
// @Tags        semantic-negotiation
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body semanticnegotiation.DecideRequest true "Semantic negotiation decide request"
//
// @Success     200 {object} semanticnegotiation.Response "Negotiation step executed successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Session not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/decide [post]
func (a *App) decideSemanticNegotiationHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Advancing semantic negotiation | workspace=%s mas=%s",
		workspaceID, masID,
	)

	var reqPayload semanticnegotiation.DecideRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Validate required fields
	if reqPayload.SessionID == "" {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "session_id is required"},
		)
	}
	if len(reqPayload.AgentReplies) == 0 {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "agent_replies list cannot be empty"},
		)
	}

	cogReq := &cognitionagentclient.SemanticNegotiationDecideRequest{
		SessionID:    reqPayload.SessionID,
		AgentReplies: reqPayload.AgentReplies,
	}

	cogResp, err := a.cognitionAgentsClient.SendSemanticNegotiationDecide(r.Context(), cogReq, workspaceID, masID)
	if err != nil {
		log.Errorf("failed to advance semantic negotiation, error: %s", err.Error())
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to advance semantic negotiation"},
		)
	}

	// The decide endpoint returns a flat JSON object (not an SSTP envelope).
	resp := &semanticnegotiation.Response{
		Status:      cogResp.Status,
		SessionID:   cogResp.SessionID,
		Round:       cogResp.Round,
		Messages:    cogResp.Messages,
		FinalResult: cogResp.FinalResult,
	}

	// If agreement is reached, persist the final result to shared memory.
	if cogResp.Status == "agreed" && len(cogResp.FinalResult) > 0 {
		log.Infof("agreement has been reached, final result is being persisted to the shared memory")
		// Wrap FinalResult in an array to match the extraction endpoint's `data: list[dict]` schema.
		finalResultJSON, err := json.Marshal([]map[string]interface{}{cogResp.FinalResult})
		if err != nil {
			log.Errorf("failed to marshal final_result for persistence | workspace=%s mas=%s session=%s err=%v",
				workspaceID, masID, reqPayload.SessionID, err)
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
			} else {
				log.Infof("persisted negotiation agreement to shared memory | workspace=%s mas=%s session=%s",
					workspaceID, masID, reqPayload.SessionID)
			}
		}
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}
