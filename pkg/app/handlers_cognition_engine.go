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
// @Description	and forwards it to the management plane's /api/cognition-engines/register endpoint.
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
// @Router		/api/cognition-engines/register [post]
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
	if req.Type == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "type is required",
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
	registerURL := mgmtURL + "/api/cognition-engines/register"

	// Add cfn_id to the payload
	payload := map[string]any{
		"name":         req.Name,
		"type":         req.Type,
		"url":          req.URL,
		"version":      req.Version,
		"cfn_id":       CfnID,
		"capabilities": req.Capabilities,
		"metrics":      req.Metrics,
	}

	// Add optional fields if provided
	if req.Auth != nil {
		payload["auth"] = req.Auth
	}
	if req.Config != nil {
		payload["config"] = req.Config
	}
	if req.MASConfig != nil {
		payload["mas_config"] = req.MASConfig
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to marshal request: %v", err),
		})
	}

	log.Infof("forwarding CE registration to management plane: %s (ce_name=%s, ce_type=%s, cfn_id=%s)",
		registerURL, req.Name, req.Type, CfnID)

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
