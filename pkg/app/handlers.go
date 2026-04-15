package app

import (
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
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

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
// @Summary		Proxy API requests to a remote memory provider
// @Description	Forwards REST API requests to a remote memory provider (Mem0, Graphiti, etc.) for agent-specific memory operations.
// @Description	The memory provider base URL and auth credentials are auto-resolved from management plane config based on workspace/MAS/agent IDs.
// @Description	The `http-url` field should contain the relative path and query parameters to append to the provider base URL.
// @Description
// @Description	**GET example** — retrieve memories:
// @Description	```json
// @Description	{
// @Description	  "header": {},
// @Description	  "payload": {
// @Description	    "http-request-type": "GET",
// @Description	    "http-url": "v1/memories/?user_id=curl-test-user",
// @Description	    "http-request-body": {},
// @Description	    "http-headers": {}
// @Description	  }
// @Description	}
// @Description	```
// @Description
// @Description	**POST example** — add memories:
// @Description	```json
// @Description	{
// @Description	  "header": {},
// @Description	  "payload": {
// @Description	    "http-request-type": "POST",
// @Description	    "http-url": "/v1/memories/",
// @Description	    "http-request-body": {
// @Description	      "messages": [{"role": "user", "content": "I prefer dark mode in all my apps"}],
// @Description	      "user_id": "curl-test-user"
// @Description	    },
// @Description	    "http-headers": {}
// @Description	  }
// @Description	}
// @Description	```
// @Tags			memory-operations
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		masId		path		string								true	"Multi-Agentic System ID"
// @Param		agentId		path		string								true	"Agent ID"
// @Param		body		body		object	true	"Memory operation request (see MemoryOperationRequest)"
// @Success		200			{object}	object	"Proxied response (actual provider status is in http-status field)"
// @Failure		400			{object}	map[string]string	"Invalid request body or missing http-request-type"
// @Failure		404			{object}	map[string]string	"Memory provider config not found for agent"
// @Failure		502			{object}	map[string]string	"Failed to forward request to memory provider"
// @Failure		503			{object}	map[string]string	"Memory proxy client not configured"
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

	// Build the audit request snapshot (included in all audit entries for this handler)
	auditRequest := map[string]interface{}{
		"http_method": req.Payload.HTTPRequestType,
		"http_url":    targetURL,
	}
	if req.Payload.HTTPRequestBody != nil {
		auditRequest["http_request_body"] = req.Payload.HTTPRequestBody
	}

	// helper to create a single audit entry for this operation
	createAudit := func(status string, extra map[string]interface{}) {
		info := map[string]interface{}{
			"status":  status,
			"request": auditRequest,
		}
		for k, v := range extra {
			info[k] = v
		}
		auditInfo, _ := json.Marshal(info)
		// Hacky: fetch agentic_memory.id from summary API on first audit call.
		// TODO: Remove once IDs are available directly in CfnConfig global map.
		ensureAuditResourceIDs()
		auditResID := AgentMemoryID
		if auditResID == "" {
			auditResID = agentID
		}
		auditEvt := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMASAgent,
			ResourceIdentifier:      agentID,
			AuditType:               audit.AuditTypeAgentMemoryOperation,
			AuditResourceIdentifier: auditResID,
			AuditInformation:        datatypes.JSON(auditInfo),
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if status == "FAILED" {
			if errVal, ok := info["error"]; ok {
				errStr := fmt.Sprintf("%v", errVal)
				auditEvt.AuditExtraInformation = &errStr
			}
		}
		if auditErr := a.db.CreateAuditEvent(auditEvt); auditErr != nil {
			log.Errorf("failed to create audit event: %v", auditErr)
		}
	}

	// Forward the request via the memory proxy client
	if a.memoryProxyClient == nil {
		createAudit("FAILED", map[string]interface{}{"error": "memory proxy client is not configured"})
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "memory proxy client is not configured",
		})
	}

	method := strings.ToUpper(strings.TrimSpace(req.Payload.HTTPRequestType))
	proxyResp, err := a.memoryProxyClient.Do(r.Context(), method, targetURL, requestBody, headers)
	if err != nil {
		createAudit("FAILED", map[string]interface{}{"error": err.Error()})
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to memory provider: %v", err),
		})
	}
	defer proxyResp.Body.Close()

	// Read and parse response body
	respBody, err := io.ReadAll(proxyResp.Body)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read memory provider response: %v", err)
		createAudit("FAILED", map[string]interface{}{"error": errMsg})
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

	// Audit: single entry with request + response
	createAudit("SUCCESS", map[string]interface{}{
		"http_status": proxyResp.StatusCode,
		"response":    respJSON,
	})

	// Return the response with 200 status (the actual HTTP status from the memory provider is in the response body)
	return eh.RespondWithJSON(w, http.StatusOK, response)
}
