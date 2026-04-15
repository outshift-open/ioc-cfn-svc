package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/semanticnegotiation"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	api "github.com/cisco-eti/ioc-cfn-svc/pkg/generated/api"
)

// openapiAdapter implements the generated ServerInterface.
// It bridges between the generated API interfaces and existing handler logic.
type openapiAdapter struct {
	app *App
}

// Ensure openapiAdapter implements ServerInterface
var _ api.ServerInterface = (*openapiAdapter)(nil)

func newOpenAPIAdapter(app *App) *openapiAdapter {
	return &openapiAdapter{app: app}
}

// CreateOrUpdateSharedMemories implements POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories
func (o *openapiAdapter) CreateOrUpdateSharedMemories(w http.ResponseWriter, r *http.Request, workspaceId string, masId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Creating or updating shared memories | workspace=%s mas=%s", workspaceId, masId)
	// Decode request body
	var reqPayload api.CreateOrUpdateRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalCreateRequest(&reqPayload)

	// Recreate request with payload in body for existing handler
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()

	// Set path parameters using Go 1.22 SetPathValue
	newReq.SetPathValue("workspaceId", workspaceId)
	newReq.SetPathValue("masId", masId)

	// Call existing handler
	status, err := o.app.createOrUpdateSharedMemoriesHandler(w, newReq)
	if err != nil {
		// Error already written to response by handler
		return
	}

	if status != http.StatusCreated && status != http.StatusOK {
		// Handler returned non-success status
		return
	}
}

// FetchSharedMemories implements POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query
func (o *openapiAdapter) FetchSharedMemories(w http.ResponseWriter, r *http.Request, workspaceId string, masId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Fetching shared memories | workspace=%s mas=%s", workspaceId, masId)

	// Decode request body
	var reqPayload api.QueryRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalQueryRequest(&reqPayload)

	// Recreate request with payload in body
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()
	newReq.SetPathValue("workspaceId", workspaceId)
	newReq.SetPathValue("masId", masId)

	// Call existing handler
	status, err := o.app.fetchSharedMemoriesHandler(w, newReq)
	if err != nil {
		return
	}

	if status != http.StatusOK {
		return
	}
}

// OnboardVectorStore implements POST /api/workspaces/{workspaceId}/shared-memories/vector-store
func (o *openapiAdapter) OnboardVectorStore(w http.ResponseWriter, r *http.Request, workspaceId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Onboarding vector store | workspace=%s", workspaceId)

	// Decode request body (optional)
	var reqPayload api.OnboardVectorStoreRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalOnboardRequest(&reqPayload)

	// Recreate request with payload in body
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()
	newReq.SetPathValue("workspaceId", workspaceId)

	// Call existing handler
	status, err := o.app.onboardSharedMemoriesVectorStoreHandler(w, newReq)
	if err != nil {
		return
	}

	if status != http.StatusCreated && status != http.StatusOK {
		return
	}
}

// MemoryOperations implements POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations
func (o *openapiAdapter) MemoryOperations(w http.ResponseWriter, r *http.Request, workspaceId string, masId string, agentId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Memory operations | workspace=%s mas=%s agent=%s", workspaceId, masId, agentId)

	// Decode request body
	var reqPayload api.MemoryOperationRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalMemoryOpRequest(&reqPayload)

	// Recreate request with payload in body
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()
	newReq.SetPathValue("workspaceId", workspaceId)
	newReq.SetPathValue("masId", masId)
	newReq.SetPathValue("agentId", agentId)

	// Call existing handler
	status, err := o.app.memoryOperationsHandler(w, newReq)
	if err != nil {
		return
	}

	if status != http.StatusOK {
		return
	}
}

// StartSemanticNegotiation implements POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/start
func (o *openapiAdapter) StartSemanticNegotiation(w http.ResponseWriter, r *http.Request, workspaceId string, masId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Starting semantic negotiation | workspace=%s mas=%s", workspaceId, masId)

	// Decode request body
	var reqPayload api.StartRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalStartRequest(&reqPayload)

	// Recreate request with payload in body
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()
	newReq.SetPathValue("workspaceId", workspaceId)
	newReq.SetPathValue("masId", masId)

	// Call existing handler
	status, err := o.app.startSemanticNegotiationHandler(w, newReq)
	if err != nil {
		return
	}

	if status != http.StatusOK {
		return
	}
}

// DecideSemanticNegotiation implements POST /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/decide
func (o *openapiAdapter) DecideSemanticNegotiation(w http.ResponseWriter, r *http.Request, workspaceId string, masId string) {
	log := getLogger()
	ctx := r.Context()

	log.Infof("Deciding semantic negotiation | workspace=%s mas=%s", workspaceId, masId)

	// Decode request body
	var reqPayload api.DecideRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	// Transform to internal DTO
	internalReq := transformToInternalDecideRequest(&reqPayload)

	// Recreate request with payload in body
	payloadBytes, _ := json.Marshal(internalReq)
	newReq, _ := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(payloadBytes))
	newReq.Header = r.Header.Clone()
	newReq.SetPathValue("workspaceId", workspaceId)
	newReq.SetPathValue("masId", masId)

	// Call existing handler
	status, err := o.app.decideSemanticNegotiationHandler(w, newReq)
	if err != nil {
		return
	}

	if status != http.StatusOK {
		return
	}
}

// Helper functions for transforming generated types to internal DTOs

func transformToInternalCreateRequest(req *api.CreateOrUpdateRequest) *sharedmemory.CreateOrUpdateRequest {
	// Convert Data from map[string]interface{} to json.RawMessage
	dataBytes, _ := json.Marshal(req.Payload.Data)

	internal := &sharedmemory.CreateOrUpdateRequest{
		RequestId: req.RequestId,
		Header:    transformHeader(req.Header),
		// Transform ExtractionPayload from generated to cognitionagentclient type
		Payload: cognitionagentclient.ExtractionPayload{
			Metadata: cognitionagentclient.ExtractionPayloadMetadata{
				Format: string(req.Payload.Metadata.Format),
			},
			Data: dataBytes,
		},
	}

	return internal
}

func transformToInternalQueryRequest(req *api.QueryRequest) *sharedmemory.QueryRequest {
	internal := &sharedmemory.QueryRequest{
		Header:         transformHeader(&req.Header),
		Intent:         &req.Intent,
		RequestId:      req.RequestId,
		SearchStrategy: req.SearchStrategy,
	}

	if req.AdditionalContext != nil {
		internal.AdditionalContext = *req.AdditionalContext
	}

	return internal
}

func transformToInternalOnboardRequest(req *api.OnboardVectorStoreRequest) *sharedmemory.OnboardVectorStoreRequest {
	internal := &sharedmemory.OnboardVectorStoreRequest{
		RequestId: req.RequestId,
		Header:    transformHeader(req.Header),
	}

	return internal
}

func transformHeader(h *api.Header) *sharedmemory.Header {
	if h == nil {
		return nil
	}
	return &sharedmemory.Header{
		AgentID: h.AgentId,
	}
}

func transformToInternalMemoryOpRequest(req *api.MemoryOperationRequest) *memoryoperations.MemoryOperationRequest {
	internal := &memoryoperations.MemoryOperationRequest{
		Payload: memoryoperations.MemoryOperationPayload{
			HTTPRequestType: string(req.Payload.HttpRequestType),
		},
	}

	if req.Payload.HttpUrl != nil {
		internal.Payload.HTTPURL = *req.Payload.HttpUrl
	}

	if req.Payload.HttpRequestBody != nil {
		internal.Payload.HTTPRequestBody = *req.Payload.HttpRequestBody
	}

	if req.Payload.HttpHeaders != nil {
		internal.Payload.HTTPHeaders = *req.Payload.HttpHeaders
	}

	return internal
}

func transformToInternalStartRequest(req *api.StartRequest) *semanticnegotiation.StartRequest {
	internal := &semanticnegotiation.StartRequest{
		SessionID:   req.SessionId,
		ContentText: req.ContentText,
		NSteps:      req.NSteps,
	}

	if len(req.Agents) > 0 {
		internal.Agents = make([]semanticnegotiation.Agent, len(req.Agents))
		for i, agent := range req.Agents {
			internal.Agents[i] = semanticnegotiation.Agent{
				ID:   agent.Id,
				Name: agent.Name,
			}
		}
	}

	return internal
}

func transformToInternalDecideRequest(req *api.DecideRequest) *semanticnegotiation.DecideRequest {
	internal := &semanticnegotiation.DecideRequest{
		SessionID: req.SessionId,
	}

	if len(req.AgentReplies) > 0 {
		internal.AgentReplies = make([]semanticnegotiation.AgentReply, len(req.AgentReplies))
		for i, reply := range req.AgentReplies {
			internal.AgentReplies[i] = semanticnegotiation.AgentReply{
				AgentID: reply.AgentId,
				Action:  string(reply.Action),
			}
			if reply.Offer != nil {
				internal.AgentReplies[i].Offer = *reply.Offer
			}
		}
	}

	return internal
}

// Helper utility functions

func respondWithError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
