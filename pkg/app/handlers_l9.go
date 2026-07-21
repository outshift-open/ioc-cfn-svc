package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/outshift-open/ioc-cfn-svc/pkg/audit"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

const (
	l9MessagesPath = "/api/l9/messages"
)

var (
	defaultCEURL = "http://localhost:9004" // Fallback CE URL for testing
)

// Valid subkinds per L9 kind specification
var validSubkinds = map[l9.Kind][]string{
	l9.KindKnowledge:   {"query", "distillation", "extraction", "feedback"},
	l9.KindCommit:      {"converged", "resolved", "abort"},
	l9.KindIntent:      {"coordinator-assignment", "mission"},
	l9.KindExchange:    {"team-formation"},
	l9.KindContingency: {"negotiation"},
}

// l9Handler handles L9 protocol messages with content-based routing.
// All routing information comes from the L9 message itself (participants.groups).
// Extracts workspace-id and mas-id from message, finds matching CE by kind/subkind/subprotocol,
// and forwards to CE's /api/l9/messages endpoint. Returns the CE's L9 response.
// @Summary Process and route L9 message to Cognition Engine
// @Description Content-based routing: extracts workspace/MAS from L9 message participants.groups, selects CE by kind/subkind, forwards to CE's /api/l9/messages endpoint
// @Tags l9
// @Accept json
// @Produce json
// @Param message body l9.L9 true "L9 protocol message with header (kind, subkind, participants) and payload"
// @Success 200 {object} l9.L9 "L9 response from Cognition Engine"
// @Failure 400 {object} map[string]string "Invalid L9 message format or missing required fields"
// @Failure 404 {object} map[string]string "Workspace/MAS not found or no CE handles this kind/subkind"
// @Failure 500 {object} map[string]string "Internal error or CE forwarding failed"
// @Failure 502 {object} map[string]string "CE unreachable or returned error"
// @Failure 503 {object} map[string]string "Target CE is disabled"
// @Router /api/l9/messages [post]
func (a *App) l9Handler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	msg, err := parseL9Request(r)
	if err != nil {
		log.Errorf("l9Handler: failed to parse request: %v", err)
		return respondError(w, http.StatusBadRequest, err.Error())
	}

	if err := validateKindSubkindCombination(msg); err != nil {
		log.Errorf("l9Handler: invalid kind/subkind: %v", err)
		return respondError(w, http.StatusBadRequest, err.Error())
	}

	routingInfo, err := extractRoutingInfo(msg)
	if err != nil {
		log.Errorf("l9Handler: failed to extract routing info: %v", err)
		return respondError(w, http.StatusBadRequest, err.Error())
	}

	log.Infof("l9Handler: workspace=%s, mas=%s, kind=%s, subkind=%v, subprotocol=%s",
		routingInfo.workspaceID, routingInfo.masID,
		msg.Header.Kind, msg.Header.Subkind, msg.Header.Subprotocol)

	ceURL, targetCE, err := a.resolveTargetCEURL(routingInfo, msg)
	if err != nil {
		l9e := err.(*l9Error)
		a.recordL9AuditWithStatus(msg, routingInfo, "failed", l9e.message)
		return respondError(w, l9e.statusCode, l9e.Error())
	}

	response, l9err := forwardToCE(ceURL, msg)
	if l9err != nil {
		log.Errorf("l9Handler: CE forwarding failed: %v", l9err)
		a.recordL9AuditWithStatus(msg, routingInfo, "failed", l9err.message)
		return respondError(w, l9err.statusCode, l9err.message)
	}

	if targetCE != nil {
		log.Infof("l9Handler: successfully routed to CE %s", targetCE.ID)
	} else {
		log.Infof("l9Handler: successfully routed to fallback CE at %s", ceURL)
	}

	// Record L9 audit event for the request message
	a.recordL9AuditWithStatus(msg, routingInfo, "success", "")

	return eh.RespondWithJSON(w, http.StatusOK, response)
}

// routingInfo contains extracted identifiers from L9 message
type routingInfo struct {
	workspaceID string
	masID       string
}

// l9Error represents a handler error with HTTP status code
type l9Error struct {
	message    string
	statusCode int
}

func (e *l9Error) Error() string {
	return e.message
}

// parseL9Request reads and validates the L9 message from HTTP request
func parseL9Request(r *http.Request) (*l9.L9, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	var msg l9.L9
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("invalid L9 message format: %w", err)
	}

	return &msg, nil
}

// extractRoutingInfo extracts workspace and MAS IDs from participants.groups
func extractRoutingInfo(msg *l9.L9) (*routingInfo, error) {
	groups := msg.Header.Participants.Groups
	if groups == nil {
		return nil, fmt.Errorf("missing participants.groups in L9 message")
	}

	workspaceID, _ := (*groups)["workspace_id"].(string)
	if workspaceID == "" {
		return nil, fmt.Errorf("missing workspace_id in participants.groups")
	}

	masID, _ := (*groups)["mas_id"].(string)
	if masID == "" {
		return nil, fmt.Errorf("missing mas_id in participants.groups")
	}

	return &routingInfo{
		workspaceID: workspaceID,
		masID:       masID,
	}, nil
}

// resolveTargetCEURL determines the CE URL to use for this message.
// Returns the CE URL, the target CE (nil if using fallback), and any fatal error.
// Uses fallback URL when no CE is found (404), but returns error for other failures.
func (a *App) resolveTargetCEURL(info *routingInfo, msg *l9.L9) (string, *EngineCfg, error) {
	log := getLogger()

	targetCE, l9err := a.findTargetCE(info, msg)

	if l9err != nil {
		// TODO: Use fallback URL for CE-not-found errors for testing now, remove this in production
		if l9err.statusCode == http.StatusNotFound {
			log.Errorf("CRITICAL: No CE found for workspace=%s, mas=%s, kind=%s, subkind=%v, subprotocol=%s - using fallback URL %s",
				info.workspaceID, info.masID, msg.Header.Kind, msg.Header.Subkind, msg.Header.Subprotocol, defaultCEURL)
			return defaultCEURL, nil, nil
		}
		// Fatal errors (config unavailable, disabled CE, etc.)
		return "", nil, l9err
	}

	ceURL := getCEURL(targetCE)
	log.Infof("l9Handler: routing to CE %s (%s) at %s", targetCE.ID, targetCE.Name, ceURL)
	return ceURL, targetCE, nil
}

// findTargetCE locates the appropriate CE to handle this message.
// Returns 404 if no CE found (triggers fallback in resolveTargetCEURL).
// Returns 500/503 for fatal errors (no fallback, handler returns error).
func (a *App) findTargetCE(info *routingInfo, msg *l9.L9) (*EngineCfg, *l9Error) {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	if ParsedConfig == nil {
		return nil, &l9Error{
			message:    "CFN configuration not available",
			statusCode: http.StatusInternalServerError,
		}
	}

	mas := ParsedConfig.FindMAS(info.workspaceID, info.masID)
	if mas == nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("MAS %s not found in workspace %s", info.masID, info.workspaceID),
			statusCode: http.StatusNotFound,
		}
	}

	matchingCEs := findMatchingCEs(mas, msg.Header.Kind, msg.Header.Subkind, msg.Header.Subprotocol)
	if len(matchingCEs) == 0 {
		return nil, &l9Error{
			message:    fmt.Sprintf("no CE found to handle %s/%v/%s messages", msg.Header.Kind, msg.Header.Subkind, msg.Header.Subprotocol),
			statusCode: http.StatusNotFound,
		}
	}

	if len(matchingCEs) > 1 {
		ceIDs := make([]string, len(matchingCEs))
		for i, ce := range matchingCEs {
			ceIDs[i] = ce.ID
		}
		return nil, &l9Error{
			message:    fmt.Sprintf("ambiguous routing: multiple CEs found: %v", ceIDs),
			statusCode: http.StatusInternalServerError,
		}
	}

	targetCE := matchingCEs[0]
	if !targetCE.Enabled {
		return nil, &l9Error{
			message:    fmt.Sprintf("target CE %s is disabled", targetCE.ID),
			statusCode: http.StatusServiceUnavailable,
		}
	}

	return targetCE, nil
}

// findMatchingCEs returns CEs that can handle the given kind/subkind/subprotocol.
// A CE matches if:
// 1. Its KindsSubkinds map contains the requested kind, AND
// 2. The subkinds list for that kind contains the requested subkind
//    (empty subkinds list means CE doesn't handle any subkind for this kind), AND
// 3. Either the CE has no Subprotocols defined (subprotocol check is skipped),
//    OR the CE's Subprotocols list contains the requested subprotocol
func findMatchingCEs(mas *MASCfg, kind l9.Kind, subkind interface{}, subprotocol string) []*EngineCfg {
	log := getLogger()
	var matches []*EngineCfg

	kindStr := string(kind)
	subkindStr := getSubkindString(subkind)

	for _, masEngine := range mas.CognitionEngines {
		ce := ParsedConfig.FindCE(masEngine.ID)
		if ce == nil {
			log.Warnf("CE %s listed in MAS but not found in config", masEngine.ID)
			continue
		}

		if !ce.Enabled {
			continue
		}

		// Check if CE handles this kind
		subkinds, hasKind := ce.KindsSubkinds[kindStr]
		if !hasKind {
			continue
		}

		// Check if CE handles this subkind
		// Empty subkinds list means CE doesn't handle any subkind for this kind
		if subkindStr != "" {
			found := false
			for _, sk := range subkinds {
				if sk == subkindStr {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check if CE handles this subprotocol
		// If CE has no Subprotocols defined, skip the subprotocol check (matches any subprotocol)
		// This allows GAT CEs (like CASA) to register without specifying subprotocols
		if len(ce.Subprotocols) > 0 {
			found := false
			for _, sp := range ce.Subprotocols {
				if sp == subprotocol {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		log.Debugf("Found matching CE %s (%s) for kind=%s, subkind=%s, subprotocol=%s",
			ce.ID, ce.Name, kindStr, subkindStr, subprotocol)
		matches = append(matches, ce)
	}

	return matches
}

// getCEURL returns the CE URL, with fallback to localhost for testing
func getCEURL(ce *EngineCfg) string {
	if ce.URL != "" {
		return ce.URL
	}
	getLogger().Warnf("CE %s has no URL configured, using fallback: %s", ce.ID, defaultCEURL)
	return defaultCEURL
}

// forwardToCE sends the L9 message to the CE and returns its response
func forwardToCE(ceBaseURL string, msg *l9.L9) (*l9.L9, *l9Error) {
	endpoint := ceBaseURL + l9MessagesPath

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("failed to marshal message: %v", err),
			statusCode: http.StatusInternalServerError,
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("failed to create request: %v", err),
			statusCode: http.StatusInternalServerError,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("failed to reach CE: %v", err),
			statusCode: http.StatusBadGateway,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("failed to read CE response: %v", err),
			statusCode: http.StatusInternalServerError,
		}
	}

	if resp.StatusCode >= 400 {
		return nil, &l9Error{
			message:    fmt.Sprintf("CE returned error: %s", string(respBody)),
			statusCode: resp.StatusCode,
		}
	}

	var ceResponse l9.L9
	if err := json.Unmarshal(respBody, &ceResponse); err != nil {
		return nil, &l9Error{
			message:    fmt.Sprintf("invalid L9 response from CE: %v", err),
			statusCode: http.StatusInternalServerError,
		}
	}

	return &ceResponse, nil
}

// validateKindSubkindCombination validates kind/subkind combinations per L9 spec
func validateKindSubkindCombination(msg *l9.L9) error {
	subkind := getSubkindString(msg.Header.Subkind)
	if subkind == "" {
		return nil // Empty subkind is valid
	}

	validList, ok := validSubkinds[msg.Header.Kind]
	if !ok {
		return fmt.Errorf("unknown kind: %s", msg.Header.Kind)
	}

	for _, valid := range validList {
		if subkind == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid subkind '%s' for kind=%s (must be one of: %v)",
		subkind, msg.Header.Kind, validList)
}

// getSubkindString safely converts subkind interface{} to string
func getSubkindString(subkind interface{}) string {
	if subkind == nil {
		return ""
	}
	return fmt.Sprintf("%v", subkind)
}

// respondError is a helper to return error responses
func respondError(w http.ResponseWriter, statusCode int, message string) (int, error) {
	return eh.RespondWithJSON(w, statusCode, map[string]string{"error": message})
}

// recordL9AuditWithStatus creates an L9 audit event with status and optional error message.
// L9 events are stored in the dedicated audit_l9 table but exposed via the existing audit API
// using L9_* audit types (e.g., audit_type=L9_COMMIT).
// Skips audit if message lacks required fields (message.id, message.episode).
// Errors are logged but do not fail the request.
func (a *App) recordL9AuditWithStatus(msg *l9.L9, info *routingInfo, status, errMsg string) {
	if a.db == nil {
		return
	}

	event := audit.NewL9AuditEventFromMessage(msg, info.workspaceID, info.masID)
	if event == nil {
		// Message lacks required audit fields (message.id or message.episode)
		return
	}

	event.Status = status
	if errMsg != "" {
		event.ErrorMsg = &errMsg
	}

	if err := a.db.CreateL9AuditEvent(event); err != nil {
		getLogger().Errorf("l9Handler: failed to record audit event: %v", err)
	}
}
