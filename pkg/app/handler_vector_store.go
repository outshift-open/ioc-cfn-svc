package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
)

func mapVectorSimilarityResults(src []iocmemoryprovider.KnowledgeVectorSimilaritySearchResult) []sharedmemory.VectorSimilaritySearchResult {
	if len(src) == 0 {
		return nil
	}

	out := make([]sharedmemory.VectorSimilaritySearchResult, 0, len(src))
	for _, r := range src {
		out = append(out, sharedmemory.VectorSimilaritySearchResult{
			Score:    r.Score,
			ID:       r.ID,
			Content:  r.Content,
			Metadata: r.Metadata,
		})
	}
	return out
}

// vectorSimilaritySearchHandler godoc
//
// @Summary     Search shared memory vectors by similarity
// @Description Performs vector similarity search over document embeddings stored for a given workspace and MAS.
//
// @Tags        Vector Store
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body sharedmemory.VectorSimilaritySearchRequest true "Similarity search request"
//
// @Success     200 {object} sharedmemory.VectorSimilaritySearchResponse "Search results"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/vectors/similarity-search [post]
func (a *App) vectorSimilaritySearchHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Vector similarity search | workspace=%s mas=%s",
		workspaceID, masID,
	)

	var req sharedmemory.VectorSimilaritySearchRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	if len(req.Payload.EmbeddingVector) == 0 {
		return eh.RespondWithJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "payload.embedding_vector is required"},
		)
	}

	requestId := req.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	limit := 10
	if req.Payload.TopK != nil && *req.Payload.TopK > 0 {
		limit = *req.Payload.TopK
	}

	metric := "cosine"
	if req.Payload.Metric != nil && *req.Payload.Metric != "" {
		metric = *req.Payload.Metric
	}

	memoryProviderReq := &iocmemoryprovider.KnowledgeVectorSimilaritySearchRequest{
		RequestID:      *requestId,
		WkspID:         workspaceID,
		MasID:          masID,
		Embedding:      req.Payload.EmbeddingVector,
		Limit:          limit,
		Metric:         metric,
		MetadataFilter: req.Payload.Filters,
	}

	response, err := a.knowledgeMemSvcClient.SimilaritySearchVectors(ctx, memoryProviderReq, false)
	if err != nil {
		log.Errorf(
			"SimilaritySearchVectors failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to perform vector similarity search, error: %v", err)},
		)
	}

	resp := &sharedmemory.VectorSimilaritySearchResponse{
		Header:    req.Header,
		RequestId: requestId,
		Results:   mapVectorSimilarityResults(response.Results),
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}
