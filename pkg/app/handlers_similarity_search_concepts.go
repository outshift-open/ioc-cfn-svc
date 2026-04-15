package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
)

type conceptSimilaritySearchHeader struct {
	AgentID *string `json:"agent_id,omitempty"`
}

type conceptSimilaritySearchPayload struct {
	EmbeddedText    string    `json:"embedded_text,omitempty"`
	EmbeddingVector []float64 `json:"embedding_vector"`
	TopK            int       `json:"top_k,omitempty"`
	SearchMetrics   string    `json:"search_metric,omitempty"`
}

type conceptSimilaritySearchRequest struct {
	Header    *conceptSimilaritySearchHeader `json:"header,omitempty"`
	RequestID *string                        `json:"request_id,omitempty"`
	Payload   conceptSimilaritySearchPayload `json:"payload"`
}

type conceptSimilaritySearchResponseHeader struct {
	WorkspaceID string `json:"workspace_id"`
	MasID       string `json:"mas_id"`
	AgentID     string `json:"agent_id,omitempty"`
}

type conceptSimilaritySearchResult struct {
	Score           float64   `json:"score"`
	ConceptID       string    `json:"concept_id"`
	ConceptName     string    `json:"concept_name"`
	EmbeddingVector []float64 `json:"embedding_vector,omitempty"`
}

type conceptSimilaritySearchResponse struct {
	Header     conceptSimilaritySearchResponseHeader `json:"header"`
	ResponseID *string                               `json:"response_id,omitempty"`
	Status     string                                `json:"status"`
	Results    []conceptSimilaritySearchResult       `json:"results,omitempty"`
	Error      *string                               `json:"error"`
}

func (a *App) conceptSimilaritySearchHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var req conceptSimilaritySearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
	}

	if len(req.Payload.EmbeddingVector) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "payload.embedding_vector is required"})
	}

	requestID := req.RequestID
	if requestID == nil {
		requestID = common.StrToPtr(uuid.New().String())
	}

	agentID := ""
	if req.Header != nil && req.Header.AgentID != nil {
		agentID = *req.Header.AgentID
	}

	topK := req.Payload.TopK
	if topK <= 0 {
		topK = 10
	}

	metric := req.Payload.SearchMetrics
	if metric == "" {
		metric = "l2"
	}

	log.Infof(
		"Concept similarity search | workspace=%s mas=%s request_id=%s agent_id=%s metric=%s top_k=%d",
		workspaceID, masID, *requestID, agentID, metric, topK,
	)

	searchReq := &iocmemoryprovider.KnowledgeGraphSimilaritySearchRequest{
		RequestID: *requestID,
		MasID:     common.StrToPtr(masID),
		WkspID:    common.StrToPtr(workspaceID),
		Embedding: req.Payload.EmbeddingVector,
		Limit:     topK,
		Metric:    metric,
	}

	includeEmbeddings := r.URL.Query().Get("include_embeddings") == "true"

	searchResp, err := a.knowledgeMemSvcClient.SimilaritySearchConcepts(r.Context(), searchReq, includeEmbeddings)
	if err != nil {
		log.Errorf("Concept similarity search failed | workspace=%s mas=%s err=%v", workspaceID, masID, err)
		errMsg := fmt.Sprintf("similarity search failed: %v", err)
		return eh.RespondWithJSON(w, http.StatusInternalServerError, conceptSimilaritySearchResponse{
			Header:     conceptSimilaritySearchResponseHeader{WorkspaceID: workspaceID, MasID: masID, AgentID: agentID},
			ResponseID: requestID,
			Status:     "failure",
			Error:      &errMsg,
		})
	}

	results := make([]conceptSimilaritySearchResult, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		results = append(results, conceptSimilaritySearchResult{
			Score:           r.Score,
			ConceptID:       r.ConceptID,
			ConceptName:     r.ConceptName,
			EmbeddingVector: r.EmbeddingVector,
		})
	}

	return eh.RespondWithJSON(w, http.StatusOK, conceptSimilaritySearchResponse{
		Header:     conceptSimilaritySearchResponseHeader{WorkspaceID: workspaceID, MasID: masID, AgentID: agentID},
		ResponseID: requestID,
		Status:     "success",
		Results:    results,
		Error:      nil,
	})
}
