package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
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

// TODO: replace the hardcoded concepts with data returned by the Cognition Agent(s)
func mockConcepts() []iocmemoryprovider.Concept {
	return []iocmemoryprovider.Concept{
		{
			ID:          "923e4567-e89b-12d3-a456-426614174000",
			Name:        "New Test Artificial Intelligence",
			Description: stringPtr("The simulation of human intelligence processes by machines"),
			Attributes: map[string]interface{}{
				"category":     "Technology",
				"founded_year": 1956,
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		},
		{
			ID:          "923e4567-e89b-12d3-a456-426614174001",
			Name:        "New Machine Learning",
			Description: stringPtr("A subset of AI that enables systems to learn from data"),
			Attributes: map[string]interface{}{
				"category":     "Computer Science",
				"parent_field": "AI",
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.2, 0.3, 0.4, 0.5, 0.6},
			},
		},
		{
			ID:          "923e4567-e89b-12d3-a456-426614174002",
			Name:        "Deep Learning",
			Description: stringPtr("A subset of ML using neural networks with multiple layers"),
			Attributes: map[string]interface{}{
				"category":     "Neural networks",
				"parent_field": "Machine Learning",
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.3, 0.4, 0.5, 0.6, 0.7},
			},
		},
	}
}

// TODO: replace the hardcoded relations with data returned by the Cognition Agent(s)
func mockRelations() []iocmemoryprovider.Relation {
	return []iocmemoryprovider.Relation{
		{
			ID:       "823e4567-e89b-12d3-a456-426614174000",
			Relation: "HAS_A_SUBFIELD",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174000",
				"923e4567-e89b-12d3-a456-426614174001",
			},
			Attributes: map[string]interface{}{
				"since":    1956,
				"strength": 0.9,
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "relation-embedding",
				Data: []float64{0.15, 0.25, 0.35, 0.45, 0.55},
			},
		},
		{
			ID:       "723e4567-e89b-12d3-a456-426614174000",
			Relation: "HAS_SUBFIELD",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174001",
				"923e4567-e89b-12d3-a456-426614174002",
			},
			Attributes: map[string]interface{}{
				"since":    1980,
				"strength": 0.95,
			},
		},
		{
			ID:       "623e4567-e89b-12d3-a456-426614174000",
			Relation: "RELATED_TO",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174000",
				"923e4567-e89b-12d3-a456-426614174002",
			},
			Attributes: map[string]interface{}{
				"relationship": "hierarchical",
				"direct":       "False",
			},
		},
	}

}

func resolveRecords(payload *iocmemoryprovider.KnowledgeGraphStoreRequest) *iocmemoryprovider.Records {
	if payload != nil && payload.Records != nil {
		if len(payload.Records.Concepts) > 0 || len(payload.Records.Relations) > 0 {
			return payload.Records
		}
	}

	return &iocmemoryprovider.Records{
		Concepts:  mockConcepts(),
		Relations: mockRelations(),
	}
}

// upsertSharedMemoriesHandler godoc
//
// @Summary     Upsert shared memories.
// @Description Upserts shared memory entries (concepts and relations) for a given workspace and multi-agentic system.
//
//	**Note:** The request payload and response structure are still **TBD** and subject to change.
//	Current schemas reflect a provisional contract and may be updated as the Cognition Agent
//	integration is finalized.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body iocmemoryprovider.KnowledgeGraphStoreRequest false "Upsert request (currently ignored; hard-coded data is used)"
//
// @Success     201 {object} iocmemoryprovider.KnowledgeGraphStoreResponse "Shared memories successfully upserted"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories [post]
func (a *App) upsertSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Upserting shared memories | workspace=%s mas=%s",
		workspaceID, masID,
	)

	var payload iocmemoryprovider.KnowledgeGraphStoreRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	req := iocmemoryprovider.NewKnowledgeGraphStoreRequest()
	req.WkspID = &workspaceID
	req.MasID = &masID
	req.ForceReplace = true

	req.Records = &iocmemoryprovider.Records{
		Concepts:  mockConcepts(),
		Relations: mockRelations(),
	}

	// Use payload concepts/relations if provided, else mocks
	req.Records = resolveRecords(&payload)

	resp, err := a.knowledgeMemSvcClient.UpsertKnowledgeGraph(ctx, req)
	if err != nil {
		log.Errorf(
			"UpsertKnowledgeGraph failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "failed to upsert shared memories"},
		)
	}

	return eh.RespondWithJSON(w, http.StatusCreated, resp)
}

// resolveQueryRecords uses request payload if the concept IDs aren't empty, otherwise it falls back to hardcoded IDs
func resolveQueryRecords(
	payload *iocmemoryprovider.KnowledgeGraphQueryRequest,
) iocmemoryprovider.QueryRecords {

	if payload != nil && len(payload.Records.Concepts) > 0 {
		return payload.Records
	}

	// Fallback to hardcoded node ids
	return iocmemoryprovider.QueryRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{
			{ID: "923e4567-e89b-12d3-a456-426614174000"},
			{ID: "923e4567-e89b-12d3-a456-426614174001"},
		},
	}
}

// fetchSharedMemoriesHandler godoc
//
// @Summary     Fetch shared memories
// @Description Queries shared memories for a given workspace and multi-agentic system using a graph path query.
//
//	**Note:** The request payload and response structure are still **TBD** and subject to change.
//	Current schemas reflect a provisional contract and may be updated as the Cognition Agent
//	integration is finalized.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
//

// @Success     200 {object} iocmemoryprovider.KnowledgeGraphStoreResponse "Query executed successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query [post]
func (a *App) fetchSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Fetching shared memories | workspace=%s mas=%s",
		workspaceID, masID,
	)

	var payload iocmemoryprovider.KnowledgeGraphQueryRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	queryType := iocmemoryprovider.QueryTypePath // default

	if payload.QueryCriteria != nil && payload.QueryCriteria.QueryType != "" {
		queryType = strings.ToLower(payload.QueryCriteria.QueryType)
	}

	queryFns := map[string]func(
		context.Context,
		*iocmemoryprovider.KnowledgeGraphQueryRequest,
	) (*iocmemoryprovider.KnowledgeGraphQueryResponse, error){
		iocmemoryprovider.QueryTypePath:      a.knowledgeMemSvcClient.QueryKnowledgeGraphPath,
		iocmemoryprovider.QueryTypeNeighbour: a.knowledgeMemSvcClient.QueryKnowledgeGraphNeighbor,
		iocmemoryprovider.QueryTypeConcept:   a.knowledgeMemSvcClient.QueryKnowledgeGraphConcept,
	}

	queryFn, ok := queryFns[queryType]
	if !ok {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": fmt.Sprintf(
					"invalid query_type, valid values are: %s, %s, %s",
					iocmemoryprovider.QueryTypePath,
					iocmemoryprovider.QueryTypeNeighbour,
					iocmemoryprovider.QueryTypeConcept,
				),
			},
		)
	}

	useDirection := false
	criteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
		queryType,
		nil, // unlimited depth
		&useDirection,
	)

	req := iocmemoryprovider.NewKnowledgeGraphQueryRequest(criteria)
	req.WkspID = &workspaceID
	req.MasID = &masID
	req.Records = resolveQueryRecords(&payload)

	resp, err := queryFn(ctx, req)
	if err != nil {
		log.Errorf(
			"Knowledge graph query failed | type=%s workspace=%s mas=%s err=%v",
			queryType, workspaceID, masID, err,
		)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to fetch shared memories, %v", err.Error())},
		)
	}

	log.Infof(
		"Query succeeded | status=%s records=%d",
		resp.Status,
		len(resp.Records),
	)

	return eh.RespondWithJSON(w, http.StatusOK, resp)
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

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
