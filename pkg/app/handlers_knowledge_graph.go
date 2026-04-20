package app

import (
	"encoding/json"
	"net/http"

	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// fetchKnowledgeGraphHandler godoc
//
// @Summary     Fetch knowledge graph for a MAS
// @Description Returns all nodes and edges in the knowledge graph for the given MAS.
// @Tags        knowledge-graph
// @Produce     json
//
// @Param       mas_id query string true "Multi-Agentic System ID"
//
// @Success     200 {object} map[string]interface{} "Knowledge graph data"
// @Failure     400 {object} map[string]string "Missing or invalid mas_id"
// @Failure     404 {object} map[string]string "Graph not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/mgmt/knowledge-graph [get]
func (a *App) fetchKnowledgeGraphHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	masID := r.URL.Query().Get("mas_id")
	if masID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "mas_id query parameter is required",
		})
	}

	log.Infof("Fetching knowledge graph | mas_id=%s", masID)

	body, statusCode, err := a.knowledgeMemSvcClient.FetchKnowledgeGraph(r.Context(), masID)
	if err != nil {
		log.Errorf("failed to fetch knowledge graph | mas_id=%s err=%v", masID, err)
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
