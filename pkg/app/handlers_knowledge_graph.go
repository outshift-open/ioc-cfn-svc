// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
)

// validateWorkspaceAndMAS checks that the given workspaceID and masID both exist
func validateWorkspaceAndMAS(workspaceID, masID string) error {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	if ParsedConfig == nil {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	ws := ParsedConfig.FindWorkspace(workspaceID)
	if ws == nil {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	mas := ParsedConfig.FindMAS(workspaceID, masID)
	if mas == nil {
		return fmt.Errorf("multi-agentic system %s not found", masID)
	}

	return nil
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
