package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// getMemoryProviderURL retrieves the memory provider URL from CfnConfig for a specific agent.
// It navigates: workspaces -> multi_agentic_systems -> agents -> agentic_memory -> config
func (a *App) getMemoryProviderURL(workspaceID, masID, agentID string) (string, error) {
	log := getLogger()

	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	// Navigate to workspaces
	workspaces, ok := CfnConfig["workspaces"].([]interface{})
	if !ok {
		return "", fmt.Errorf("workspaces not found in config")
	}

	// Find the workspace
	var workspace map[string]interface{}
	for _, ws := range workspaces {
		wsMap, ok := ws.(map[string]interface{})
		if !ok {
			continue
		}
		if wsMap["workspace_id"] == workspaceID {
			workspace = wsMap
			break
		}
	}
	if workspace == nil {
		return "", fmt.Errorf("workspace %s not found", workspaceID)
	}

	// Navigate to multi_agentic_systems
	masList, ok := workspace["multi_agentic_systems"].([]interface{})
	if !ok {
		return "", fmt.Errorf("multi_agentic_systems not found in workspace")
	}

	// Find the MAS
	var mas map[string]interface{}
	for _, m := range masList {
		masMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if masMap["id"] == masID {
			mas = masMap
			break
		}
	}
	if mas == nil {
		return "", fmt.Errorf("multi-agentic system %s not found", masID)
	}

	// Navigate to agents
	agentsList, ok := mas["agents"].([]interface{})
	if !ok {
		return "", fmt.Errorf("agents not found in multi-agentic system")
	}

	// Find the agent
	var agent map[string]interface{}
	for _, a := range agentsList {
		agentMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		if agentMap["agent_id"] == agentID {
			agent = agentMap
			break
		}
	}
	if agent == nil {
		return "", fmt.Errorf("agent %s not found", agentID)
	}

	// Get agentic_memory config
	agenticMemory, ok := agent["agentic_memory"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("agentic_memory not found for agent")
	}

	// Check if memory is enabled
	if enabled, ok := agenticMemory["enabled"].(bool); ok && !enabled {
		return "", fmt.Errorf("agentic memory is disabled for this agent")
	}

	// Get config with host and port
	memConfig, ok := agenticMemory["config"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("config not found in agentic_memory")
	}

	host, hostOk := memConfig["host"].(string)
	if !hostOk || host == "" {
		return "", fmt.Errorf("host not found in memory provider config")
	}

	var baseURL string

	// Check if host already contains a protocol (full URL)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		// Host is a full URL (e.g., "https://example.ngrok.io"), use as-is
		baseURL = strings.TrimSuffix(host, "/") // Remove trailing slash if present
		log.Debugf("resolved memory provider URL (full URL) for agent %s: %s", agentID, baseURL)
	} else {
		// Host is just a hostname (e.g., "localhost", "ioc-mem0"), build URL with port
		port, portOk := memConfig["port"].(float64) // JSON numbers are float64
		if !portOk {
			return "", fmt.Errorf("port not found in memory provider config for hostname %s", host)
		}
		baseURL = fmt.Sprintf("http://%s:%d", host, int(port))
		log.Debugf("resolved memory provider URL (hostname+port) for agent %s: %s", agentID, baseURL)
	}

	return baseURL, nil
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
	log := getLogger()

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

	// Get the memory provider base URL from synced config
	memoryProviderBaseURL, err := a.getMemoryProviderURL(workspaceID, masID, agentID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("failed to find memory provider config: %v", err),
		})
	}

	// Build full URL by properly joining base URL with path and query parameters
	baseURL, err := url.Parse(memoryProviderBaseURL)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("invalid base URL from config: %v", err),
		})
	}

	var targetURL string
	if req.Payload.HTTPURL != "" {
		// HTTPURL contains path with optional query params (e.g., "/v1/memories/add?user_id=123")
		pathURL, err := url.Parse(req.Payload.HTTPURL)
		if err != nil {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid path/query in http-url: %v", err),
			})
		}
		// Resolve the path relative to base URL
		targetURL = baseURL.ResolveReference(pathURL).String()
	} else {
		targetURL = baseURL.String()
	}

	// Marshal the request body if provided
	var requestBody []byte
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

	log.Infof("forwarding %s request to memory provider: %s", req.Payload.HTTPRequestType, targetURL)

	// Forward the request via the memory proxy client
	if a.memoryClient == nil {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "memory proxy client is not configured",
		})
	}

	proxyResp, err := a.memoryClient.ForwardRequest(r.Context(), req.Payload.HTTPRequestType, targetURL, requestBody, headers)
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
