package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// getCfnDummyHandler godoc
// @Summary		Get CFN dummy data
// @Description	Returns mock CFN data
// @Tags			cfn
// @Produce		json
// @Success		200	{object}	interface{}
// @Router			/api/v1/cfn/dummy [get]
func (a *App) getCfnDummyHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "cfn dummy response",
	})
}

// upsertSharedMemoriesHandler godoc
// @Summary		Upsert shared memories
// @Description	Upserts shared memory entries for a given workspace and multi-agentic system
// @Tags			shared-memories
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		systemId		path		string								true	"Multi-Agentic System ID"
// @Param		body			body		sharedmemory.SharedMemoryUpsertRequest	true	"Upsert request"
// @Success		201				{object}	sharedmemory.SharedMemoryUpsertResponse
// @Failure		400				{object}	map[string]string
// @Failure		500				{object}	map[string]string
// @Router		/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories [post]
func (a *App) upsertSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	// TODO: validate workspaceId and systemId path params
	//workspaceID := eh.PathParam(r, "workspaceId")
	//systemID := eh.PathParam(r, "systemId")

	var req sharedmemory.SharedMemoryUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	// TODO: persist shared memory for (workspaceID, systemID)
	// For now, return a lightweight mock response

	response := sharedmemory.SharedMemoryUpsertResponse{
		Status:  "success",
		Message: "shared memories upserted successfully",
	}

	return eh.RespondWithJSON(w, http.StatusCreated, response)
}

// fetchSharedMemoriesHandler godoc
// @Summary		Fetch shared memories
// @Description	Fetches shared memory entries for a given workspace and multi-agentic system
// @Tags			shared-memories
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		systemId		path		string								true	"Multi-Agentic System ID"
// @Param		body			body		sharedmemory.SharedMemoryQueryRequest	true	"Query request"
// @Success		200				{object}	sharedmemory.SharedMemoryQueryResponse
// @Failure		400				{object}	map[string]string
// @Failure		500				{object}	map[string]string
// @Router		/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories/query [post]
func (a *App) fetchSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	workspaceID := eh.PathParam(r, "workspaceId")
	systemID := eh.PathParam(r, "systemId")

	var req sharedmemory.SharedMemoryQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	_ = workspaceID
	_ = systemID
	_ = req
	// TODO: query shared memories for (workspaceID, systemId)

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.SharedMemoryQueryResponse{})
}

// memoryOperationsHandler godoc
// @Summary		Execute memory operations on remote memory provider
// @Description	Proxies HTTP requests to a remote memory provider for agent memory operations
// @Tags			memory-operations
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		masId		path		string								true	"Multi-Agentic System ID"
// @Param		agentId		path		string								true	"Agent ID"
// @Param		body		body		memoryoperations.MemoryOperationRequest	true	"Memory operation request"
// @Success		200			{object}	memoryoperations.MemoryOperationResponse
// @Failure		400			{object}	map[string]string
// @Failure		500			{object}	map[string]string
// @Router		/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations [post]
func (a *App) memoryOperationsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	agentID := eh.PathParam(r, "agentId")

	// Log path parameters for debugging
	log.Debugf("memory operation request: workspaceId=%s, masId=%s, agentId=%s", workspaceID, masID, agentID)

	// Parse the request body
	var req memoryoperations.MemoryOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid JSON body: %v", err),
		})
	}

	// Validate required fields
	if req.Payload.HTTPRequestType == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "http-request-type is required",
		})
	}

	// TODO: Get the URL from config we synced from Management plane
	if req.Payload.HTTPURL == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "http-url is required",
		})
	}

	// Marshal the request body if provided
	var requestBody []byte
	var err error
	if req.Payload.HTTPRequestBody != nil && len(req.Payload.HTTPRequestBody) > 0 {
		requestBody, err = json.Marshal(req.Payload.HTTPRequestBody)
		if err != nil {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("failed to marshal request body: %v", err),
			})
		}
	}

	// Prepare headers from the envelope
	headers := make(map[string]string)
	if req.Payload.HTTPHeaders != nil {
		headers = req.Payload.HTTPHeaders
	}

	// Ensure Content-Type is set for requests with body
	if requestBody != nil && headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	log.Infof("forwarding %s request to memory provider: %s", req.Payload.HTTPRequestType, req.Payload.HTTPURL)

	// Forward the request via the Agentic Memory Client (mem0 proxy)
	if a.mem0Client == nil {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "agentic memory client is not configured",
		})
	}

	proxyResp, err := a.mem0Client.ForwardRequest(r.Context(), req.Payload.HTTPRequestType, req.Payload.HTTPURL, requestBody, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to memory provider: %v", err),
		})
	}

	// Build the response envelope
	response := memoryoperations.MemoryOperationResponse{
		HTTPStatus:       proxyResp.HTTPStatus,
		HTTPHeaders:      proxyResp.HTTPHeaders,
		HTTPResponseBody: proxyResp.HTTPResponseBody,
	}

	log.Infof("memory provider responded with status: %d", proxyResp.HTTPStatus)

	// Return the response with 200 status (the actual HTTP status from the memory provider is in the response body)
	return eh.RespondWithJSON(w, http.StatusOK, response)
}
