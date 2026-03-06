package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/memoryoperations"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
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
// @Param       body        body iocmemoryprovider.KnowledgeGraphStoreRequest false "Upsert request"
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

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()

	// Audit: start of knowledge ingestion
	startAuditInfo, _ := json.Marshal(map[string]string{
		"status": "STARTED",
	})
	startAudit := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      masID,
		AuditType:               audit.AuditTypeKnowledgeIngestion,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: masID,
		AuditInformation:        datatypes.JSON(startAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if err := a.db.CreateAuditEvent(startAudit); err != nil {
		log.Errorf("failed to create start audit event: %v", err)
	}

	resp, err := a.knowledgeMemSvcClient.UpsertKnowledgeGraph(ctx, req)
	if err != nil {
		log.Errorf(
			"UpsertKnowledgeGraph failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)

		// Audit: end of knowledge ingestion (failure)
		errMsg := err.Error()
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		endAudit := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMemoryProvider,
			ResourceIdentifier:      masID,
			AuditType:               audit.AuditTypeKnowledgeIngestion,
			// TODO: AuditResourceIdentifier may change to a different identifier if required.
			AuditResourceIdentifier: masID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create end audit event: %v", auditErr)
		}

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "failed to upsert shared memories"},
		)
	}

	// Audit: end of knowledge ingestion (success)
	endAuditInfo, _ := json.Marshal(map[string]string{
		"status": "SUCCESS",
	})
	endAudit := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMemoryProvider,
		ResourceIdentifier:      masID,
		AuditType:               audit.AuditTypeKnowledgeIngestion,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: masID,
		AuditInformation:        datatypes.JSON(endAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
		log.Errorf("failed to create end audit event: %v", auditErr)
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
// @Param       body        body iocmemoryprovider.KnowledgeGraphQueryRequest false "Query request"
//
// @Success     200 {object} iocmemoryprovider.KnowledgeGraphQueryResponse "Query executed successfully"
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

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()

	// Audit: start of knowledge query
	startAuditInfo, _ := json.Marshal(map[string]string{
		"status": "STARTED",
	})
	startAudit := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      masID,
		AuditType:               audit.AuditTypeKnowledgeQuery,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: masID,
		AuditInformation:        datatypes.JSON(startAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if err := a.db.CreateAuditEvent(startAudit); err != nil {
		log.Errorf("failed to create start audit event: %v", err)
	}

	resp, err := queryFn(ctx, req)
	if err != nil {
		log.Errorf(
			"Knowledge graph query failed | type=%s workspace=%s mas=%s err=%v",
			queryType, workspaceID, masID, err,
		)

		// Audit: end of knowledge query (failure)
		errMsg := err.Error()
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		endAudit := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMemoryProvider,
			ResourceIdentifier:      masID,
			AuditType:               audit.AuditTypeKnowledgeQuery,
			// TODO: AuditResourceIdentifier may change to a different identifier if required.
			AuditResourceIdentifier: masID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create end audit event: %v", auditErr)
		}

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

	// Audit: end of knowledge query (success)
	endAuditInfo, _ := json.Marshal(map[string]string{
		"status": "SUCCESS",
	})
	endAudit := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMemoryProvider,
		ResourceIdentifier:      masID,
		AuditType:               audit.AuditTypeKnowledgeQuery,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: masID,
		AuditInformation:        datatypes.JSON(endAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
		log.Errorf("failed to create end audit event: %v", auditErr)
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

// memoryProviderConfig holds the resolved memory provider configuration for an agent.
type memoryProviderConfig struct {
	baseURL      string
	providerName string
	auth         *memoryProviderAuth // nil when type="none" or auth absent
}

// memoryProviderAuth holds auth credentials parsed from the management plane config.
type memoryProviderAuth struct {
	authType    string // "none", "token", "bearer", "basic", "custom"
	apiKey      string // for "token"
	accessToken string // for "bearer"
	username    string // for "basic"
	password    string // for "basic"
	headerName  string // for "custom"
	headerValue string // for "custom"
}

// getMemoryProviderConfig retrieves the full memory provider config (URL + auth) from CfnConfig for a specific agent.
// It navigates: workspaces -> multi_agentic_systems -> agents -> agentic_memory -> config
func (a *App) getMemoryProviderConfig(workspaceID, masID, agentID string) (*memoryProviderConfig, error) {
	log := getLogger()

	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	log.Debugf("resolving memory provider config for ws=%s mas=%s agent=%s", workspaceID, masID, agentID)

	// Navigate to workspaces
	workspaces, ok := CfnConfig["workspaces"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("workspaces not found in config")
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
		return nil, fmt.Errorf("workspace %s not found", workspaceID)
	}

	// Navigate to multi_agentic_systems
	masList, ok := workspace["multi_agentic_systems"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("multi_agentic_systems not found in workspace")
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
		return nil, fmt.Errorf("multi-agentic system %s not found", masID)
	}

	// Navigate to agents
	agentsList, ok := mas["agents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("agents not found in multi-agentic system")
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
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	// Get agentic_memory config
	agenticMemory, ok := agent["agentic_memory"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("agentic_memory not found for agent")
	}

	// Check if memory is enabled
	if enabled, ok := agenticMemory["enabled"].(bool); ok && !enabled {
		return nil, fmt.Errorf("agentic memory is disabled for this agent")
	}

	// Get config
	memConfig, ok := agenticMemory["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("config not found in agentic_memory")
	}

	// Read URL (new format from management plane)
	baseURL, urlOk := memConfig["url"].(string)
	if !urlOk || baseURL == "" {
		return nil, fmt.Errorf("url not found in memory provider config")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Read memory_provider_name
	providerName, _ := agenticMemory["memory_provider_name"].(string)

	// Parse auth from config
	var auth *memoryProviderAuth
	if authMap, ok := memConfig["auth"].(map[string]interface{}); ok {
		authType, _ := authMap["type"].(string)
		if authType != "" && authType != "none" {
			auth = &memoryProviderAuth{authType: authType}
			if creds, ok := authMap["credentials"].(map[string]interface{}); ok {
				auth.apiKey, _ = creds["api_key"].(string)
				auth.accessToken, _ = creds["access_token"].(string)
				auth.username, _ = creds["username"].(string)
				auth.password, _ = creds["password"].(string)
				auth.headerName, _ = creds["header_name"].(string)
				auth.headerValue, _ = creds["header_value"].(string)
			} else {
				log.Warnf("auth type is %q but credentials block is missing", authType)
			}
		}
	}

	log.Debugf("resolved memory provider for agent %s: url=%s provider=%s authType=%s",
		agentID, baseURL, providerName, func() string {
			if auth != nil {
				return auth.authType
			}
			return "none"
		}())

	return &memoryProviderConfig{
		baseURL:      baseURL,
		providerName: providerName,
		auth:         auth,
	}, nil
}

// getMemoryProviderURL is a convenience wrapper that returns just the base URL.
func (a *App) getMemoryProviderURL(workspaceID, masID, agentID string) (string, error) {
	cfg, err := a.getMemoryProviderConfig(workspaceID, masID, agentID)
	if err != nil {
		return "", err
	}
	return cfg.baseURL, nil
}

// injectAuthHeaders sets the appropriate Authorization header based on the provider auth config.
// It always strips any user-provided Authorization header first (security).
func injectAuthHeaders(headers map[string]string, auth *memoryProviderAuth) {
	log := getLogger()

	// SECURITY: Always strip any user-provided Authorization header
	delete(headers, "Authorization")

	if auth == nil {
		return
	}

	switch auth.authType {
	case "token":
		if auth.apiKey == "" {
			log.Warnf("auth type is 'token' but api_key is empty, skipping auth")
			return
		}
		headers["Authorization"] = "Token " + auth.apiKey
	case "bearer":
		if auth.accessToken == "" {
			log.Warnf("auth type is 'bearer' but access_token is empty, skipping auth")
			return
		}
		headers["Authorization"] = "Bearer " + auth.accessToken
	case "basic":
		if auth.username == "" || auth.password == "" {
			log.Warnf("auth type is 'basic' but username/password is empty, skipping auth")
			return
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(auth.username + ":" + auth.password))
		headers["Authorization"] = "Basic " + encoded
	case "custom":
		if auth.headerName == "" || auth.headerValue == "" {
			log.Warnf("auth type is 'custom' but header_name/header_value is empty, skipping auth")
			return
		}
		headers[auth.headerName] = auth.headerValue
	case "none", "":
		// No auth needed
	default:
		log.Warnf("unknown auth type %q, skipping auth", auth.authType)
	}
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

	// Get the memory provider config (URL + auth) from synced config
	providerCfg, err := a.getMemoryProviderConfig(workspaceID, masID, agentID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("failed to find memory provider config: %v", err),
		})
	}

	// Build full URL by properly joining base URL with path and query parameters
	baseURL, err := url.Parse(providerCfg.baseURL)
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

	// Inject auth headers from management plane config
	injectAuthHeaders(headers, providerCfg.auth)

	log.Infof("forwarding %s request to memory provider: %s", req.Payload.HTTPRequestType, targetURL)

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()
	// Audit: start of memory operation
	startAuditInfo, _ := json.Marshal(map[string]string{
		"status": "STARTED",
	})
	startAudit := &audit.Audit{
		OperationID:        &operationID,
		ResourceType:       audit.ResourceTypeMASAgent,
		ResourceIdentifier: masID,
		AuditType:          audit.AuditTypeMemoryOperation,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: agentID,
		AuditInformation:        datatypes.JSON(startAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(startAudit); auditErr != nil {
		log.Errorf("failed to create start audit event: %v", auditErr)
	}

	// Forward the request via the memory proxy client
	if a.memoryProxyClient == nil {
		// Audit: end of memory operation (failure - client not configured)
		errMsg := "memory proxy client is not configured"
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		endAudit := &audit.Audit{
			OperationID:        &operationID,
			ResourceType:       audit.ResourceTypeMemoryProvider,
			ResourceIdentifier: masID,
			AuditType:          audit.AuditTypeMemoryOperation,
			// TODO: AuditResourceIdentifier may change to a different identifier if required.
			AuditResourceIdentifier: agentID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create end audit event: %v", auditErr)
		}

		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "memory proxy client is not configured",
		})
	}

	method := strings.ToUpper(strings.TrimSpace(req.Payload.HTTPRequestType))
	proxyResp, err := a.memoryProxyClient.Do(r.Context(), method, targetURL, requestBody, headers)
	if err != nil {
		// Audit: end of memory operation (failure)
		errMsg := err.Error()
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		endAudit := &audit.Audit{
			OperationID:        &operationID,
			ResourceType:       audit.ResourceTypeMemoryProvider,
			ResourceIdentifier: masID,
			AuditType:          audit.AuditTypeMemoryOperation,
			// TODO: AuditResourceIdentifier may change to a different identifier if required.
			AuditResourceIdentifier: agentID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create end audit event: %v", auditErr)
		}

		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to memory provider: %v", err),
		})
	}
	defer proxyResp.Body.Close()

	// Read and parse response body
	respBody, err := io.ReadAll(proxyResp.Body)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read memory provider response: %v", err)
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		endAudit := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMemoryProvider,
			ResourceIdentifier:      masID,
			AuditType:               audit.AuditTypeMemoryOperation,
			AuditResourceIdentifier: agentID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create end audit event: %v", auditErr)
		}
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": errMsg,
		})
	}

	var respJSON map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &respJSON); err != nil {
			respJSON = map[string]interface{}{"raw": string(respBody)}
		}
	}

	// Extract response headers (take first value)
	respHeaders := make(map[string]string)
	for k, vals := range proxyResp.Header {
		if len(vals) > 0 {
			respHeaders[k] = vals[0]
		}
	}

	// Build the response envelope
	response := memoryoperations.MemoryOperationResponse{
		HTTPStatus:       proxyResp.StatusCode,
		HTTPHeaders:      respHeaders,
		HTTPResponseBody: respJSON,
	}

	log.Infof("memory provider responded with status: %d", proxyResp.StatusCode)

	// Audit: end of memory operation (success)
	endAuditInfo, _ := json.Marshal(map[string]string{
		"status":      "SUCCESS",
		"http_status": fmt.Sprintf("%d", proxyResp.StatusCode),
	})
	endAudit := &audit.Audit{
		OperationID:        &operationID,
		ResourceType:       audit.ResourceTypeMemoryProvider,
		ResourceIdentifier: masID,
		AuditType:          audit.AuditTypeMemoryOperation,
		// TODO: AuditResourceIdentifier may change to a different identifier if required.
		AuditResourceIdentifier: agentID,
		AuditInformation:        datatypes.JSON(endAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
		log.Errorf("failed to create end audit event: %v", auditErr)
	}

	// Return the response with 200 status (the actual HTTP status from the memory provider is in the response body)
	return eh.RespondWithJSON(w, http.StatusOK, response)
}


// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
