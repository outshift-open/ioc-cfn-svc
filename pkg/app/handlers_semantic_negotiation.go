package app

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/semanticnegotiation"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
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

	cogResp, err := a.cognitionAgentsClient.SendSemanticNegotiationStart(r.Context(), cogReq)
	if err != nil {
		log.Errorf("failed to start semantic negotiation, error: %s", err.Error())
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to start semantic negotiation"},
		)
	}

	resp := &semanticnegotiation.Response{
		Status:  cogResp.Status,
		Message: cogResp.Message,
		Result:  cogResp.Result,
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

	// Transform DTO to client request
	agentReplies := make([]cognitionagentclient.SemanticNegotiationAgentReply, len(reqPayload.AgentReplies))
	for i, reply := range reqPayload.AgentReplies {
		agentReplies[i] = cognitionagentclient.SemanticNegotiationAgentReply{
			AgentID: reply.AgentID,
			Action:  reply.Action,
			Offer:   reply.Offer,
		}
	}

	cogReq := &cognitionagentclient.SemanticNegotiationDecideRequest{
		SessionID:    reqPayload.SessionID,
		AgentReplies: agentReplies,
	}

	cogResp, err := a.cognitionAgentsClient.SendSemanticNegotiationDecide(r.Context(), cogReq)
	if err != nil {
		log.Errorf("failed to advance semantic negotiation, error: %s", err.Error())
		// Check if it's a 404 error (session not found)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to advance semantic negotiation"},
		)
	}

	resp := &semanticnegotiation.Response{
		Status:  cogResp.Status,
		Message: cogResp.Message,
		Result:  cogResp.Result,
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}
