package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// l9Handler handles L9 protocol messages between MAS and CE.
// @Summary Process L9 message
// @Description Receives an L9 message, validates MAS-CE association, and returns an L9 response
// @Tags l9
// @Accept json
// @Produce json
// @Param workspaceId path string true "Workspace ID"
// @Param masId path string true "Multi-Agentic System ID"
// @Param ceId path string true "Cognition Engine ID"
// @Param message body l9.L9 true "L9 message"
// @Success 200 {object} l9.L9
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/cognition-engines/{ceId}/l9 [post]
func (a *App) l9Handler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	// Extract path parameters
	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	ceID := eh.PathParam(r, "ceId")

	log.Infof("l9Handler: processing L9 message for workspace=%s, mas=%s, ce=%s", workspaceID, masID, ceID)

	// Validate MAS-CE association (non-blocking for testing)
	cfnConfigMutex.RLock()
	var masConfig map[string]any
	if ParsedConfig != nil {
		masConfig = ParsedConfig.FindMASConfigForCE(workspaceID, masID, ceID)
	}
	cfnConfigMutex.RUnlock()

	if masConfig == nil {
		log.Warnf("l9Handler: CE %s is not associated with MAS %s in workspace %s (allowing for testing)", ceID, masID, workspaceID)
	} else {
		log.Infof("l9Handler: validated MAS-CE association")
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("l9Handler: failed to read request body: %v", err)
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("failed to read request body: %v", err),
		})
	}
	defer r.Body.Close()

	// Parse L9 message
	var l9Msg l9.L9
	if err := json.Unmarshal(body, &l9Msg); err != nil {
		log.Errorf("l9Handler: failed to unmarshal L9 message: %v", err)
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid L9 message format: %v", err),
		})
	}

	log.Infof("l9Handler: received L9 message - protocol=%s, kind=%s, version=%s",
		l9Msg.Header.Protocol,
		l9Msg.Header.Kind,
		l9Msg.Header.Version)

	// Process the L9 message
	// TODO: Add your business logic here to process the incoming message
	// TODO: Determine message direction from l9Msg.Header.Participants
	// TODO: Route to appropriate destination (MAS -> CE or CE -> MAS)
	// For now, we'll echo back the message as a simple response
	responseMsg := l9Msg

	// Send response
	return eh.RespondWithJSON(w, http.StatusOK, responseMsg)
}
