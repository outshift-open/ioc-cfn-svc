package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
)

// agentVectorUpsertHandler godoc
//
// @Summary     Upsert vectors for an agent
// @Description Upserts one or more vector records into the MAS store, tagged to a specific agent.
//
// @Tags        Vector Store
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       agentId     path string true "Agent ID"
// @Param       body        body sharedmemory.AgentVectorUpsertRequest true "Upsert request"
//
// @Success     201 {object} map[string]string "Upserted successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Vector store not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/vectors [post]
func (a *App) agentVectorUpsertHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	agentID := eh.PathParam(r, "agentId")

	log.Infof("Agent vector upsert | workspace=%s mas=%s agent=%s", workspaceID, masID, agentID)

	var req sharedmemory.AgentVectorUpsertRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		}
	}

	if len(req.Records) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "records is required and must not be empty"})
	}

	requestID := req.RequestID
	if requestID == nil {
		requestID = common.StrToPtr(uuid.New().String())
	}

	records := make([]iocmemoryprovider.KnowledgeVectorStoreRequestRecord, 0, len(req.Records))
	for i, rec := range req.Records {
		id := rec.ID
		if id == "" {
			id = uuid.New().String()
		} else {
			// Validate that provided ID is a valid UUID
			if _, err := uuid.Parse(id); err != nil {
				return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("records[%d].id must be a valid UUID, got %q", i, id)})
			}
		}
		records = append(records, iocmemoryprovider.KnowledgeVectorStoreRequestRecord{
			ID:        id,
			Content:   rec.Content,
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{Data: rec.Embedding.Data},
			Metadata:  rec.Metadata,
		})
	}

	providerReq := iocmemoryprovider.NewKnowledgeVectorStoreRequest(workspaceID, masID, records)
	providerReq.AgentID = &agentID

	resp, err := a.knowledgeMemSvcClient.UpsertKnowledgeVectors(ctx, providerReq)
	if err != nil {
		log.Errorf("Agent vector upsert failed | workspace=%s mas=%s agent=%s err=%v", workspaceID, masID, agentID, err)
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("vector store not found for workspace=%s mas=%s", workspaceID, masID)})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to upsert vectors: %v", err)})
	}

	return eh.RespondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"request_id": requestID,
		"status":     resp.Status,
		"message":    resp.Message,
	})
}

// agentVectorDeleteHandler godoc
//
// @Summary     Delete a vector for an agent
// @Description Soft- or hard-deletes a single vector record owned by a specific agent.
//
// @Tags        Vector Store
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       agentId     path string true "Agent ID"
// @Param       body        body sharedmemory.AgentVectorDeleteRequest true "Delete request"
//
// @Success     200 {object} map[string]string "Deleted successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Vector store or record not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/vectors [delete]
func (a *App) agentVectorDeleteHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	agentID := eh.PathParam(r, "agentId")

	log.Infof("Agent vector delete | workspace=%s mas=%s agent=%s", workspaceID, masID, agentID)

	var req sharedmemory.AgentVectorDeleteRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		}
	}

	if req.ID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	requestID := req.RequestID
	if requestID == nil {
		requestID = common.StrToPtr(uuid.New().String())
	}

	softDelete := true
	if req.SoftDelete != nil {
		softDelete = *req.SoftDelete
	}

	providerReq := iocmemoryprovider.NewKnowledgeVectorDeleteRequest(workspaceID, masID, req.ID, softDelete)
	providerReq.AgentID = &agentID

	resp, err := a.knowledgeMemSvcClient.DeleteKnowledgeVectors(ctx, providerReq)
	if err != nil {
		log.Errorf("Agent vector delete failed | workspace=%s mas=%s agent=%s err=%v", workspaceID, masID, agentID, err)
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("vector %s not found", req.ID)})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to delete vector: %v", err)})
	}

	return eh.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"request_id": requestID,
		"status":     resp.Status,
		"message":    resp.Message,
	})
}

// agentVectorSimilaritySearchHandler godoc
//
// @Summary     Similarity search scoped to an agent
// @Description Performs vector similarity search over embeddings owned by a specific agent within a MAS store.
//
// @Tags        Vector Store
// @Accept      json
// @Produce     json
//
// @Param       workspaceId        path  string true  "Workspace ID"
// @Param       masId              path  string true  "Multi-Agentic System ID"
// @Param       agentId            path  string true  "Agent ID"
// @Param       include_embeddings query bool   false "Include raw embedding vectors in results (debug only)"
// @Param       body               body  sharedmemory.VectorSimilaritySearchRequest true "Similarity search request"
//
// @Success     200 {object} sharedmemory.VectorSimilaritySearchResponse "Search results"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Vector store not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/similarity-search [post]
func (a *App) agentVectorSimilaritySearchHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	agentID := eh.PathParam(r, "agentId")

	log.Infof("Agent vector similarity search | workspace=%s mas=%s agent=%s", workspaceID, masID, agentID)

	var req sharedmemory.VectorSimilaritySearchRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		}
	}

	if len(req.Payload.EmbeddingVector) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "payload.embedding_vector is required"})
	}

	requestID := req.RequestId
	if requestID == nil {
		requestID = common.StrToPtr(uuid.New().String())
	}

	limit := 10
	if req.Payload.TopK != nil && *req.Payload.TopK > 0 {
		limit = *req.Payload.TopK
	}

	metric := "cosine"
	if req.Payload.Metric != nil && *req.Payload.Metric != "" {
		metric = *req.Payload.Metric
	}

	providerReq := &iocmemoryprovider.KnowledgeVectorSimilaritySearchRequest{
		RequestID:      *requestID,
		WkspID:         workspaceID,
		MasID:          masID,
		AgentID:        &agentID,
		Embedding:      req.Payload.EmbeddingVector,
		Limit:          limit,
		Metric:         metric,
		MetadataFilter: req.Payload.Filters,
	}

	includeEmbeddings := r.URL.Query().Get("include_embeddings") == "true"
	response, err := a.knowledgeMemSvcClient.SimilaritySearchVectors(ctx, providerReq, includeEmbeddings)
	if err != nil {
		log.Errorf("Agent similarity search failed | workspace=%s mas=%s agent=%s err=%v", workspaceID, masID, agentID, err)
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("vector store not found for workspace=%s mas=%s", workspaceID, masID)})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to perform similarity search: %v", err)})
	}

	resp := &sharedmemory.VectorSimilaritySearchResponse{
		Header:    req.Header,
		RequestId: requestID,
		Results:   mapVectorSimilarityResults(response.Results),
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

func mapVectorSimilarityResults(src []iocmemoryprovider.KnowledgeVectorSimilaritySearchResult) []sharedmemory.VectorSimilaritySearchResult {
	if len(src) == 0 {
		return nil
	}

	out := make([]sharedmemory.VectorSimilaritySearchResult, 0, len(src))
	for _, r := range src {
		result := sharedmemory.VectorSimilaritySearchResult{
			Score:           r.Score,
			EmbeddedText:    r.Content,
			EmbeddingVector: r.EmbeddingVector,
		}
		if r.Metadata != nil {
			if v, ok := r.Metadata["recorded_at"].(string); ok {
				result.Timestamp = v
			}
			if v, ok := r.Metadata["doc_index"].(float64); ok {
				result.DocIndex = int(v)
			}
			if v, ok := r.Metadata["chunk_index"].(float64); ok {
				result.ChunkIndex = int(v)
			}
			if v, ok := r.Metadata["data_source"].(string); ok {
				result.Domain = v
			}
		}
		out = append(out, result)
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
// @Param       workspaceId 				path string true "Workspace ID"
// @Param       masId       				path string true "Multi-Agentic System ID"
// @Param       include_embeddings 			query bool   alse "Include raw embedding vectors in results (debug only)"
// @Param       body        				body sharedmemory.VectorSimilaritySearchRequest true "Similarity search request"
//
// @Success     200 {object} sharedmemory.VectorSimilaritySearchResponse "Search results"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     404 {object} map[string]string "Vector store not found"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/rag/similarity-search [post]
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

	includeEmbeddings := r.URL.Query().Get("include_embeddings") == "true"
	response, err := a.knowledgeMemSvcClient.SimilaritySearchVectors(ctx, memoryProviderReq, includeEmbeddings)
	if err != nil {
		log.Errorf(
			"SimilaritySearchVectors failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			return eh.RespondWithJSON(
				w,
				http.StatusNotFound,
				map[string]string{"error": fmt.Sprintf("vector store not found for workspace=%s mas=%s", workspaceID, masID)},
			)
		}
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
