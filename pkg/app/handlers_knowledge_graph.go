package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// validateWorkspaceAndMAS checks that the given workspaceID and masID both exist
func validateWorkspaceAndMAS(workspaceID, masID string) error {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	workspaces, ok := CfnConfig["workspaces"].([]interface{})
	if !ok {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	for _, ws := range workspaces {
		wsMap, ok := ws.(map[string]interface{})
		if !ok {
			continue
		}
		if wsMap["workspace_id"] != workspaceID {
			continue
		}
		masList, ok := wsMap["multi_agentic_systems"].([]interface{})
		if !ok {
			return fmt.Errorf("multi-agentic system %s not found", masID)
		}
		for _, m := range masList {
			masMap, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if masMap["id"] == masID {
				return nil
			}
		}
		return fmt.Errorf("multi-agentic system %s not found", masID)
	}

	return fmt.Errorf("workspace %s not found", workspaceID)
}

// fetchKnowledgeGraphHandler godoc
//
// @Summary     Fetch knowledge graph for a MAS
// @Description Returns all nodes and edges in the knowledge graph for the given MAS.
// @Tags        knowledge-graph
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
//
// @Success     200 {object} map[string]interface{} "Knowledge graph data"
// @Failure     404 {object} map[string]string "Workspace or MAS not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/mgmt/workspaces/{workspaceId}/multi-agentic-systems/{masId}/knowledge-graph [get]
func (a *App) fetchKnowledgeGraphHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof("Fetching knowledge graph | workspace=%s mas=%s", workspaceID, masID)

	if err := validateWorkspaceAndMAS(workspaceID, masID); err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	body, statusCode, err := a.knowledgeMemSvcClient.FetchKnowledgeGraph(r.Context(), masID)
	if err != nil {
		log.Errorf("failed to fetch knowledge graph | workspace=%s mas=%s err=%v", workspaceID, masID, err)
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch knowledge graph",
		})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to parse knowledge graph response",
		})
	}

	return eh.RespondWithJSON(w, statusCode, result)
}
