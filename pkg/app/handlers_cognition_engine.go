package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/cognitionengine"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
)

// registerCognitionEngineHandler godoc
// @Summary		Register a Cognition Engine with the management plane
// @Description	Receives a registration request from a Cognition Engine, adds CFN context (cfn_id),
// @Description	and forwards it to the management plane's /api/cognition-engines endpoint.
// @Description
// @Description	**Request example**:
// @Description	```json
// @Description	{
// @Description	  "name": "Knowledge Management CE",
// @Description	  "type": "knowledge_management",
// @Description	  "url": "http://ce-host:9004",
// @Description	  "capabilities": ["ingestion", "retrieval"],
// @Description	  "metrics": ["kb.documents.indexed", "kb.search.latency_ms"]
// @Description	}
// @Description	```
// @Description
// @Description	The CFN service validates the request, injects the CFN ID association,
// @Description	and forwards the enriched payload to the management plane.
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		body	body		cognitionengine.RegisterRequest		true	"CE registration request"
// @Success		200		{object}	cognitionengine.RegisterResponse	"CE registered successfully"
// @Failure		400		{object}	map[string]string	"Invalid request body"
// @Failure		502		{object}	map[string]string	"Failed to forward request to management plane"
// @Failure		503		{object}	map[string]string	"CFN not registered with management plane"
// @Router		/api/cognition-engines [post]
func (a *App) registerCognitionEngineHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	// Parse the CE registration request
	var req cognitionengine.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid JSON body: %v", err),
		})
	}

	// Validate required fields
	if req.Name == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
	}
	if req.Kind == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "kind is required",
		})
	}
	if req.URL == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "url is required",
		})
	}
	if req.Version == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "version is required",
		})
	}

	// Validate URL format
	parsedURL, err := url.Parse(req.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid url format: must be a valid URL with scheme and host (e.g., http://host:port)",
		})
	}

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	// Build the forwarding request with CFN context added
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	registerURL := mgmtURL + "/api/cognition-engines"

	// Add cfn_id to the payload
	// Ensure nil slices become empty arrays in JSON
	capabilities := req.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	metrics := req.Metrics
	if metrics == nil {
		metrics = []string{}
	}

	payload := map[string]any{
		"name":               req.Name,
		"kind":               req.Kind,
		"subkind":            req.Subkind,
		"url":                req.URL,
		"version":            req.Version,
		"cfn_id":             CfnID,
		"mas_auto_associate": req.MASAutoAssociate,
		"capabilities":       capabilities,
		"metrics":            metrics,
	}

	// Add optional fields (use empty dict if nil to match Pydantic default_factory)
	if req.Auth != nil {
		payload["auth"] = req.Auth
	}
	config := req.Config
	if config == nil {
		config = map[string]interface{}{}
	}
	payload["config"] = config

	masConfig := req.MASConfig
	if masConfig == nil {
		masConfig = map[string]interface{}{}
	}
	payload["mas_config"] = masConfig

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to marshal request: %v", err),
		})
	}

	log.Infof("forwarding CE registration to management plane: %s (ce_name=%s, ce_kind=%s, ce_subkind=%s, cfn_id=%s)",
		registerURL, req.Name, req.Kind, req.Subkind, CfnID)

	// Forward the request to the management plane
	client := httpclient.New(30 * time.Second)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}

	ctx := context.Background()
	resp, err := client.Post(ctx, registerURL, requestBody, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read and parse management plane response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	// If management plane returned an error status, forward it to the CE
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errorResp map[string]any
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			errorResp = map[string]any{"raw": string(respBody)}
		}
		log.Errorf("management plane returned error: status=%d, body=%v", resp.StatusCode, errorResp)
		return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
	}

	// Parse successful response
	var mgmtResp cognitionengine.RegisterResponse
	if err := json.Unmarshal(respBody, &mgmtResp); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to parse management plane response: %v", err),
		})
	}

	log.Infof("CE registered successfully: ce_id=%s, cfn_id=%s", mgmtResp.CEID, mgmtResp.CFNID)

	// Trigger immediate config refresh to include the newly registered CE
	// This minimizes the timing gap before the CE can send operations
	go func() {
		time.Sleep(500 * time.Millisecond) // Brief delay for mgmt plane to propagate
		mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
		if err := a.RefreshConfig(mgmtURL); err != nil {
			log.Errorf("failed to refresh config after CE registration: %v", err)
		} else {
			log.Infof("config refreshed after CE registration: ce_id=%s", mgmtResp.CEID)
		}
	}()

	// Return the successful response to the CE
	return eh.RespondWithJSON(w, http.StatusOK, mgmtResp)
}

// cognitionEngineHeartbeatHandler godoc
// @Summary		Proxy CE heartbeat to management plane
// @Description	Receives heartbeat requests from a Cognition Engine and forwards them to the management plane.
// @Description	The management plane updates the CE's last_seen timestamp and status (offline → online if applicable).
// @Description
// @Description	CEs should call this endpoint every 30 seconds to maintain their online status.
// @Description
// @Description	**Response example**:
// @Description	```json
// @Description	{
// @Description	  "status": "online",
// @Description	  "last_seen": "2026-05-21T10:30:00Z"
// @Description	}
// @Description	```
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		ceId	path		string							true	"Cognition Engine ID"
// @Success		200		{object}	cognitionengine.HeartbeatResponse	"Heartbeat acknowledged"
// @Failure		404		{object}	map[string]string				"CE not found"
// @Failure		502		{object}	map[string]string				"Failed to forward heartbeat to management plane"
// @Failure		503		{object}	map[string]string				"CFN not registered with management plane"
// @Router		/api/cognition-engines/{ceId}/heartbeat [put]
func (a *App) cognitionEngineHeartbeatHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	ceID := eh.PathParam(r, "ceId")
	if ceID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id is required",
		})
	}

	// Validate ceID is a valid UUID
	if _, err := uuid.Parse(ceID); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid ce_id format: must be a valid UUID"),
		})
	}

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	// Validate CE exists in CFN config
	cfnConfigMutex.RLock()
	if ParsedConfig == nil {
		cfnConfigMutex.RUnlock()
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN config not yet loaded",
		})
	}
	ce := ParsedConfig.FindCE(ceID)
	cfnConfigMutex.RUnlock()

	if ce == nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("cognition engine %s not found in this CFN - it may take up to 30s after registration for operations to be available", ceID),
		})
	}

	// Build the heartbeat URL for management plane
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	heartbeatURL := fmt.Sprintf("%s/api/cognition-engines/%s/heartbeat", mgmtURL, ceID)

	log.Debugf("forwarding CE heartbeat to management plane: %s (ce_id=%s)", heartbeatURL, ceID)

	// Forward the heartbeat to the management plane (PUT with no body)
	client := httpclient.New(10 * time.Second)
	headers := map[string]string{
		"Accept": "application/json",
	}

	ctx := context.Background()
	resp, err := client.Put(ctx, heartbeatURL, nil, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward heartbeat to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read management plane response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	// If management plane returned an error status, forward it to the CE
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]any
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			errorResp = map[string]any{"raw": string(respBody)}
		}
		log.Errorf("management plane heartbeat failed: status=%d, body=%v", resp.StatusCode, errorResp)
		return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
	}

	// Parse successful response
	var hbResp cognitionengine.HeartbeatResponse
	if err := json.Unmarshal(respBody, &hbResp); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to parse management plane response: %v", err),
		})
	}

	log.Debugf("CE heartbeat acknowledged: ce_id=%s, status=%s", ceID, hbResp.Status)

	// Return the successful response to the CE
	return eh.RespondWithJSON(w, http.StatusOK, hbResp)
}

// listCognitionEnginesHandler godoc
// @Summary		List Cognition Engines
// @Description	List cognition engines, optionally filtered by cfn_id and/or status.
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		cfn_id	query		string								false	"Filter by CFN ID"
// @Param		status	query		string								false	"Filter by status (online/offline)"
// @Success		200		{object}	cognitionengine.CognitionEngineList	"List of cognition engines"
// @Failure		502		{object}	map[string]string					"Failed to forward request to management plane"
// @Failure		503		{object}	map[string]string					"CFN not registered with management plane"
// @Router		/api/cognition-engines [get]
func (a *App) listCognitionEnginesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	// Build query string from request
	queryParams := r.URL.Query()
	cfnIDParam := queryParams.Get("cfn_id")
	statusParam := queryParams.Get("status")

	// Build the list URL for management plane
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	listURL := fmt.Sprintf("%s/api/cognition-engines", mgmtURL)

	// Add query parameters if provided
	if cfnIDParam != "" || statusParam != "" {
		listURL += "?"
		if cfnIDParam != "" {
			listURL += fmt.Sprintf("cfn_id=%s", url.QueryEscape(cfnIDParam))
		}
		if statusParam != "" {
			if cfnIDParam != "" {
				listURL += "&"
			}
			listURL += fmt.Sprintf("status=%s", url.QueryEscape(statusParam))
		}
	}

	log.Debugf("forwarding CE list request to management plane: %s", listURL)

	// Forward the request to the management plane
	client := httpclient.New(30 * time.Second)
	headers := map[string]string{
		"Accept": "application/json",
	}

	ctx := context.Background()
	resp, err := client.Get(ctx, listURL, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read management plane response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	// If management plane returned an error status, forward it
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]any
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			errorResp = map[string]any{"raw": string(respBody)}
		}
		log.Errorf("management plane list failed: status=%d, body=%v", resp.StatusCode, errorResp)
		return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
	}

	// Parse successful response
	var listResp cognitionengine.CognitionEngineList
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to parse management plane response: %v", err),
		})
	}

	log.Debugf("CE list retrieved: total=%d", listResp.Total)

	// Return the successful response
	return eh.RespondWithJSON(w, http.StatusOK, listResp)
}

// getCognitionEngineHandler godoc
// @Summary		Get Cognition Engine
// @Description	Get details of a specific cognition engine by ID.
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		ceId	path		string								true	"Cognition Engine ID"
// @Success		200		{object}	cognitionengine.CognitionEngineDetail	"CE details"
// @Failure		400		{object}	map[string]string					"Invalid CE ID format"
// @Failure		404		{object}	map[string]string					"CE not found"
// @Failure		502		{object}	map[string]string					"Failed to forward request to management plane"
// @Failure		503		{object}	map[string]string					"CFN not registered with management plane"
// @Router		/api/cognition-engines/{ceId} [get]
func (a *App) getCognitionEngineHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	ceID := eh.PathParam(r, "ceId")
	if ceID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id is required",
		})
	}

	// Validate ceID is a valid UUID
	if _, err := uuid.Parse(ceID); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid ce_id format: must be a valid UUID",
		})
	}

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	// Validate CE exists in CFN config
	cfnConfigMutex.RLock()
	if ParsedConfig == nil {
		cfnConfigMutex.RUnlock()
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN config not yet loaded",
		})
	}
	ce := ParsedConfig.FindCE(ceID)
	cfnConfigMutex.RUnlock()

	if ce == nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("cognition engine %s not found in this CFN", ceID),
		})
	}

	// Build the get URL for management plane
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	getURL := fmt.Sprintf("%s/api/cognition-engines/%s", mgmtURL, ceID)

	log.Debugf("forwarding CE get request to management plane: %s", getURL)

	// Forward the request to the management plane
	client := httpclient.New(30 * time.Second)
	headers := map[string]string{
		"Accept": "application/json",
	}

	ctx := context.Background()
	resp, err := client.Get(ctx, getURL, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read management plane response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	// If management plane returned an error status, forward it
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]any
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			errorResp = map[string]any{"raw": string(respBody)}
		}
		log.Errorf("management plane get failed: status=%d, body=%v", resp.StatusCode, errorResp)
		return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
	}

	// Parse successful response
	var detailResp cognitionengine.CognitionEngineDetail
	if err := json.Unmarshal(respBody, &detailResp); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to parse management plane response: %v", err),
		})
	}

	log.Debugf("CE details retrieved: ce_id=%s, name=%s", detailResp.ID, detailResp.Name)

	// Return the successful response
	return eh.RespondWithJSON(w, http.StatusOK, detailResp)
}

// deleteCognitionEngineHandler godoc
// @Summary		Delete Cognition Engine
// @Description	Soft-delete (deregister) a cognition engine by ID.
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		ceId	path		string	true	"Cognition Engine ID"
// @Success		204		"CE deleted successfully"
// @Failure		400		{object}	map[string]string	"Invalid CE ID format"
// @Failure		404		{object}	map[string]string	"CE not found"
// @Failure		502		{object}	map[string]string	"Failed to forward request to management plane"
// @Failure		503		{object}	map[string]string	"CFN not registered with management plane"
// @Router		/api/cognition-engines/{ceId} [delete]
func (a *App) deleteCognitionEngineHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	ceID := eh.PathParam(r, "ceId")
	if ceID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id is required",
		})
	}

	// Validate ceID is a valid UUID
	if _, err := uuid.Parse(ceID); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid ce_id format: must be a valid UUID",
		})
	}

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")

	// Refresh config to get latest MAS-CE associations before delete
	log.Debugf("refreshing config before delete to ensure fresh MAS-CE associations")
	if err := a.RefreshConfig(mgmtURL); err != nil {
		log.Warnf("failed to refresh config before delete: %v - proceeding with cached config", err)
		// Continue with cached config rather than failing the delete
	}

	// Validate CE exists in CFN config
	cfnConfigMutex.RLock()
	if ParsedConfig == nil {
		cfnConfigMutex.RUnlock()
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN config not yet loaded",
		})
	}
	ce := ParsedConfig.FindCE(ceID)

	// Check if CE is associated with any MAS
	hasAssociation := ParsedConfig.IsCEAssociatedWithMAS(ceID)
	cfnConfigMutex.RUnlock()

	if ce == nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("cognition engine %s not found in this CFN", ceID),
		})
	}

	// Block delete if CE is still associated with any MAS
	if hasAssociation {
		return eh.RespondWithJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("cannot delete CE %s: still associated with one or more MAS", ceID),
		})
	}

	// Build the delete URL for management plane
	deleteURL := fmt.Sprintf("%s/api/cognition-engines/%s", mgmtURL, ceID)

	log.Infof("forwarding CE delete request to management plane: %s (ce_id=%s)", deleteURL, ceID)

	// Forward the request to the management plane
	client := httpclient.New(30 * time.Second)
	headers := map[string]string{
		"Accept": "application/json",
	}

	ctx := context.Background()
	resp, err := client.Delete(ctx, deleteURL, nil, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Management plane returns 204 No Content on success
	if resp.StatusCode == http.StatusNoContent {
		log.Infof("CE deleted successfully: ce_id=%s", ceID)
		w.WriteHeader(http.StatusNoContent)
		return http.StatusNoContent, nil
	}

	// Read error response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	var errorResp map[string]any
	if err := json.Unmarshal(respBody, &errorResp); err != nil {
		errorResp = map[string]any{"raw": string(respBody)}
	}
	log.Errorf("management plane delete failed: status=%d, body=%v", resp.StatusCode, errorResp)
	return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
}

// patchCognitionEngineHandler godoc
// @Summary		Partially update a Cognition Engine
// @Description	Update mutable fields of a CE: url, enabled, capabilities, metrics, config, mas_config, auth, kind, subkind.
// @Description	Immutable fields (cfn_id, version, name, auto_attach) cannot be updated and will return 400.
// @Tags			cognition-engine
// @Accept		json
// @Produce		json
// @Param		ceId	path		string								true	"Cognition Engine ID"
// @Param		body	body		cognitionengine.PatchRequest		true	"CE patch request"
// @Success		200		{object}	cognitionengine.CognitionEngineDetail	"CE updated successfully"
// @Failure		400		{object}	map[string]string					"Invalid request or attempted to update immutable field"
// @Failure		404		{object}	map[string]string					"CE not found"
// @Failure		502		{object}	map[string]string					"Failed to forward request to management plane"
// @Failure		503		{object}	map[string]string					"CFN not registered with management plane"
// @Router		/api/cognition-engines/{ceId} [patch]
func (a *App) patchCognitionEngineHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	ceID := eh.PathParam(r, "ceId")
	if ceID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id is required",
		})
	}

	// Validate ceID is a valid UUID
	if _, err := uuid.Parse(ceID); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid ce_id format: must be a valid UUID",
		})
	}

	// Parse the patch request
	var req cognitionengine.PatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid JSON body: %v", err),
		})
	}

	// Check if this CFN is registered with the management plane
	if CfnID == "" {
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN not registered with management plane yet",
		})
	}

	// Validate CE exists in CFN config
	cfnConfigMutex.RLock()
	if ParsedConfig == nil {
		cfnConfigMutex.RUnlock()
		return eh.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFN config not yet loaded",
		})
	}
	ce := ParsedConfig.FindCE(ceID)
	cfnConfigMutex.RUnlock()

	if ce == nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("cognition engine %s not found in this CFN", ceID),
		})
	}

	// Build the patch URL for management plane
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	patchURL := fmt.Sprintf("%s/api/cognition-engines/%s", mgmtURL, ceID)

	requestBody, err := json.Marshal(req)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to marshal request: %v", err),
		})
	}

	log.Infof("forwarding CE patch request to management plane: %s (ce_id=%s)", patchURL, ceID)

	// Forward the request to the management plane
	client := httpclient.New(30 * time.Second)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}

	ctx := context.Background()
	resp, err := client.Patch(ctx, patchURL, requestBody, headers)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to forward request to management plane: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read management plane response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to read management plane response: %v", err),
		})
	}

	// If management plane returned an error status, forward it
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]any
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			errorResp = map[string]any{"raw": string(respBody)}
		}
		log.Errorf("management plane patch failed: status=%d, body=%v", resp.StatusCode, errorResp)
		return eh.RespondWithJSON(w, resp.StatusCode, errorResp)
	}

	// Parse successful response
	var detailResp cognitionengine.CognitionEngineDetail
	if err := json.Unmarshal(respBody, &detailResp); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to parse management plane response: %v", err),
		})
	}

	log.Infof("CE patched successfully: ce_id=%s", ceID)

	// Return the successful response
	return eh.RespondWithJSON(w, http.StatusOK, detailResp)
}

