package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
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
	SearchMetrics   string    `json:"search_metrics,omitempty"`
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

// @Summary     Concept similarity search
// @Description Finds concept nodes nearest to a query embedding vector using HNSW index.
// @Tags        Graph Store
// @Accept      json
// @Produce     json
// @Param       workspaceId        path     string                          true  "Workspace ID"
// @Param       masId              path     string                          true  "Multi-Agentic System ID"
// @Param       include_embeddings query    bool                            false "Include raw embedding vectors in results (debug only)"
// @Param       body               body     conceptSimilaritySearchRequest  true  "Similarity search request"
// @Success     200                {object} conceptSimilaritySearchResponse
// @Failure     400                {object} map[string]string "Invalid request"
// @Failure     404                {object} map[string]string "Graph not found"
// @Failure     500                {object} map[string]string "Internal server error"
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/concepts/similarity-search [post]
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
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			errMsg := fmt.Sprintf("graph not found for workspace=%s mas=%s", workspaceID, masID)
			return eh.RespondWithJSON(w, http.StatusNotFound, conceptSimilaritySearchResponse{
				Header:     conceptSimilaritySearchResponseHeader{WorkspaceID: workspaceID, MasID: masID, AgentID: agentID},
				ResponseID: requestID,
				Status:     "not found",
				Error:      &errMsg,
			})
		}
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

// @Summary     Get neighbors by concept ID
// @Description Returns the neighboring concepts of a given concept in the knowledge graph.
// @Tags        Graph Store
// @Produce     json
// @Param       workspaceId path     string true "Workspace ID"
// @Param       masId       path     string true "Multi-Agentic System ID"
// @Param       conceptId   path     string true "Concept ID"
// @Success     200         {object} sharedmemory.NeighborsResponse
// @Failure     500         {object} map[string]string "Internal server error"
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/neighbors/{conceptId} [get]
func (a *App) getNeighborsByIdHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")
	conceptID := eh.PathParam(r, "conceptId")

	log.Infof(
		"Querying neighbors | workspace=%s mas=%s concept_id=%s",
		workspaceID, masID, conceptID,
	)

	req := &iocmemoryprovider.KnowledgeGraphQueryRequest{
		RequestID: uuid.New().String(),
		WkspID:    &workspaceID,
		MasID:     &masID,
		Records: iocmemoryprovider.QueryRecords{
			Concepts: []iocmemoryprovider.ConceptRecord{{ID: conceptID}},
		},
		QueryCriteria: iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
			iocmemoryprovider.QueryTypeNeighbour, nil, nil,
		),
	}

	kgResp, err := a.knowledgeMemSvcClient.QueryKnowledgeGraphNeighbor(ctx, req)
	if err != nil {
		log.Errorf(
			"Failed to fetch neighbors | workspace=%s mas=%s concept_id=%s err=%v",
			workspaceID, masID, conceptID, err,
		)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to fetch concept: %v", err)},
		)
	}

	log.Infof("%d neighbor(s) found for concept %s", len(kgResp.Records), conceptID)

	records := make([]sharedmemory.QueryResponseRecord, 0, len(kgResp.Records))
	for _, r := range kgResp.Records {
		records = append(records, mapKGRecordToQueryRecord(r))
	}

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.NeighborsResponse{Records: records})
}

// @Summary     Fetch concepts by IDs
// @Description Returns concept details for each of the provided concept IDs.
// @Tags        Graph Store
// @Accept      json
// @Produce     json
// @Param       workspaceId path     string                          true "Workspace ID"
// @Param       masId       path     string                          true "Multi-Agentic System ID"
// @Param       body        body sharedmemory.ConceptsByIdsRequest true "Concept IDs request"
// @Success     200         {object} sharedmemory.ConceptsByIdsResponse
// @Failure     400         {object} map[string]string "Invalid request"
// @Failure     500         {object} map[string]string "Internal server error"
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/concepts/by_ids [post]
func (a *App) fetchConceptsByIdsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var reqBody sharedmemory.ConceptsByIdsRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	log.Infof(
		"Querying concepts | workspace=%s mas=%s concept_ids=%v",
		workspaceID, masID, reqBody.IDs,
	)

	type result struct {
		resp *iocmemoryprovider.KnowledgeGraphQueryResponse
		err  error
	}

	results := make([]result, len(reqBody.IDs))
	var wg sync.WaitGroup
	for i, id := range reqBody.IDs {
		wg.Add(1)
		go func(idx int, conceptID string) {
			defer wg.Done()
			req := &iocmemoryprovider.KnowledgeGraphQueryRequest{
				RequestID: uuid.New().String(),
				WkspID:    &workspaceID,
				MasID:     &masID,
				Records: iocmemoryprovider.QueryRecords{
					Concepts: []iocmemoryprovider.ConceptRecord{{ID: conceptID}},
				},
				QueryCriteria: iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
					iocmemoryprovider.QueryTypeConcept, nil, nil,
				),
			}
			resp, err := a.knowledgeMemSvcClient.QueryKnowledgeGraphConcept(ctx, req)
			results[idx] = result{resp: resp, err: err}
		}(i, id)
	}
	wg.Wait()

	var concepts []sharedmemory.GraphConcept
	for i, r := range results {
		if r.err != nil {
			log.Errorf(
				"Failed to fetch concept %s | workspace=%s mas=%s err=%v",
				reqBody.IDs[i], workspaceID, masID, r.err,
			)
			return eh.RespondWithJSON(
				w,
				http.StatusInternalServerError,
				map[string]string{"error": fmt.Sprintf("failed to fetch concepts by IDs: %v", r.err)},
			)
		}
		for _, rec := range r.resp.Records {
			for _, c := range rec.Concepts {
				conceptType := ""
				if v, ok := c.Attributes["concept_type"]; ok {
					if s, ok := v.(string); ok {
						conceptType = s
					}
				}
				desc := ""
				if c.Description != nil {
					desc = *c.Description
				}
				concepts = append(concepts, sharedmemory.GraphConcept{
					ID:          c.ID,
					Name:        c.Name,
					Type:        conceptType,
					Description: desc,
				})
			}
		}
	}

	log.Infof("%d concept(s) found for %v", len(concepts), reqBody.IDs)

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.ConceptsByIdsResponse{Concepts: concepts})
}

// @Summary     Fetch paths between two concepts
// @Description Returns ordered paths through the knowledge graph between a source and target concept.
// @Tags        Graph Store
// @Accept      json
// @Produce     json
// @Param       workspaceId path     string                       true "Workspace ID"
// @Param       masId       path     string                       true "Multi-Agentic System ID"
// @Param       body        body sharedmemory.GraphPathsRequest true "Graph paths request"
// @Success     200         {object} sharedmemory.GraphPathsResponse
// @Failure     400         {object} map[string]string "Invalid request"
// @Failure     500         {object} map[string]string "Internal server error"
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/paths [post]
func (a *App) fetchPathsByIdsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var reqBody sharedmemory.GraphPathsRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	log.Infof(
		"Querying path | workspace=%s mas=%s source_id=%s target_id=%s",
		workspaceID, masID, reqBody.SourceID, reqBody.TargetID,
	)

	kgReq := &iocmemoryprovider.KnowledgeGraphQueryRequest{
		RequestID: uuid.New().String(),
		WkspID:    &workspaceID,
		MasID:     &masID,
		Records: iocmemoryprovider.QueryRecords{
			Concepts: []iocmemoryprovider.ConceptRecord{
				{ID: reqBody.SourceID},
				{ID: reqBody.TargetID},
			},
		},
		QueryCriteria: iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
			iocmemoryprovider.QueryTypePath,
			reqBody.MaxDepth,
			common.BoolToPtr(false),
		),
	}

	kgResp, err := a.knowledgeMemSvcClient.QueryKnowledgeGraphPath(ctx, kgReq)
	if err != nil {
		log.Errorf(
			"Failed to fetch paths | workspace=%s mas=%s source=%s target=%s err=%v",
			workspaceID, masID, reqBody.SourceID, reqBody.TargetID, err,
		)
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to fetch paths: %v", err)},
		)
	}

	allowedRelations := make(map[string]struct{}, len(reqBody.Relations))
	for _, rel := range reqBody.Relations {
		allowedRelations[rel] = struct{}{}
	}

	var paths []sharedmemory.Path

	for _, rec := range kgResp.Records {
		// Build concept id->name map
		idToName := make(map[string]string, len(rec.Concepts))
		for _, c := range rec.Concepts {
			idToName[c.ID] = c.Name
		}

		// Filter relationships by allowed relations (if specified)
		var rels []iocmemoryprovider.Relation
		for _, rel := range rec.Relationships {
			if len(rel.NodeIDs) < 2 {
				continue
			}
			if len(allowedRelations) > 0 {
				if _, ok := allowedRelations[rel.Relation]; !ok {
					continue
				}
			}
			rels = append(rels, rel)
		}

		if len(rels) == 0 {
			continue
		}

		// Build adjacency map and in-degree for path chaining
		type adjEntry struct{ rel iocmemoryprovider.Relation }
		fromToRels := make(map[string][]iocmemoryprovider.Relation)
		inDegree := make(map[string]int)
		for _, rel := range rels {
			from, to := rel.NodeIDs[0], rel.NodeIDs[1]
			fromToRels[from] = append(fromToRels[from], rel)
			inDegree[to]++
			if _, exists := inDegree[from]; !exists {
				inDegree[from] = 0
			}
		}

		// Determine start node
		startID := reqBody.SourceID
		if _, ok := fromToRels[startID]; !ok {
			for nid, deg := range inDegree {
				if deg == 0 {
					if _, ok := fromToRels[nid]; ok {
						startID = nid
						break
					}
				}
			}
			if _, ok := fromToRels[startID]; !ok {
				startID = rels[0].NodeIDs[0]
			}
		}

		// Greedy walk to produce ordered relation list
		visitedRelIDs := make(map[string]struct{})
		var orderedRels []iocmemoryprovider.Relation
		current := startID
		for {
			candidates, ok := fromToRels[current]
			if !ok {
				break
			}
			var nextRel *iocmemoryprovider.Relation
			for i := range candidates {
				rid := candidates[i].ID
				if rid == "" {
					rid = fmt.Sprintf("%s->%s->%s", candidates[i].NodeIDs[0], candidates[i].Relation, candidates[i].NodeIDs[1])
				}
				if _, used := visitedRelIDs[rid]; !used {
					visitedRelIDs[rid] = struct{}{}
					nextRel = &candidates[i]
					break
				}
			}
			if nextRel == nil {
				break
			}
			orderedRels = append(orderedRels, *nextRel)
			current = nextRel.NodeIDs[1]
			if current == reqBody.TargetID {
				break
			}
		}
		if len(orderedRels) == 0 {
			orderedRels = rels
		}

		// Build edges and node_ids
		var edges []sharedmemory.PathEdge
		var nodeIDs []string
		for _, rel := range orderedRels {
			fromID, toID := rel.NodeIDs[0], rel.NodeIDs[1]

			fromName := idToName[fromID]
			toName := idToName[toID]
			if attrs, ok := rel.Attributes["source_name"].(string); ok && attrs != "" {
				fromName = attrs
			}
			if attrs, ok := rel.Attributes["target_name"].(string); ok && attrs != "" {
				toName = attrs
			}

			edges = append(edges, sharedmemory.PathEdge{
				FromID:   fromID,
				Relation: rel.Relation,
				ToID:     toID,
				FromName: fromName,
				ToName:   toName,
			})

			if len(nodeIDs) == 0 {
				nodeIDs = append(nodeIDs, fromID, toID)
			} else if nodeIDs[len(nodeIDs)-1] == fromID {
				nodeIDs = append(nodeIDs, toID)
			} else {
				if !containsStr(nodeIDs, fromID) {
					nodeIDs = append(nodeIDs, fromID)
				}
				if !containsStr(nodeIDs, toID) {
					nodeIDs = append(nodeIDs, toID)
				}
			}
		}

		if len(edges) == 0 {
			continue
		}

		// Build symbolic representation
		parts := make([]string, 0, len(edges))
		for _, e := range edges {
			from := e.FromName
			if from == "" {
				from = e.FromID
			}
			to := e.ToName
			if to == "" {
				to = e.ToID
			}
			parts = append(parts, fmt.Sprintf("%s-[%s]->%s", from, e.Relation, to))
		}

		paths = append(paths, sharedmemory.Path{
			NodeIDs:    nodeIDs,
			Edges:      edges,
			PathLength: len(edges),
			Symbolic:   strings.Join(parts, " -> "),
		})

		if reqBody.Limit != nil && *reqBody.Limit > 0 && len(paths) >= *reqBody.Limit {
			break
		}
	}

	log.Infof(
		"%d path(s) found between %s and %s",
		len(paths), reqBody.SourceID, reqBody.TargetID,
	)

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.GraphPathsResponse{
		Status: "success",
		Paths:  paths,
	})
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ── Update graph ─────────────────────────────────────────────────────────────

type updateGraphHeader struct {
	AgentID string `json:"agent_id,omitempty"`
}

type updateGraphConcept struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

type updateGraphRelation struct {
	ID           string                 `json:"id"`
	NodeIDs      []string               `json:"node_ids"`
	Relationship string                 `json:"relationship"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

type updateGraphRequest struct {
	Header     updateGraphHeader      `json:"header,omitempty"`
	RequestID  *string                `json:"request_id,omitempty"`
	Concepts   []updateGraphConcept   `json:"concepts,omitempty"`
	Relations  []updateGraphRelation  `json:"relations,omitempty"`
	Descriptor string                 `json:"descriptor,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type updateGraphResponse struct {
	Header           updateGraphHeader      `json:"header"`
	ResponseID       string                 `json:"response_id"`
	Error            *string                `json:"error"`
	UpdatedAt        int64                  `json:"updated_at"`
	Descriptor       string                 `json:"descriptor,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ConceptsUpdated  int                    `json:"concepts_updated"`
	RelationsUpdated int                    `json:"relations_updated"`
}

// @Summary     Update knowledge graph
// @Description Adds or updates concepts and relations in an existing knowledge graph for a given workspace and MAS.
// @Tags        Graph Store
// @Accept      json
// @Produce     json
// @Param       workspaceId path     string             true  "Workspace ID"
// @Param       masId       path     string             true  "Multi-Agentic System ID"
// @Param       body        body     updateGraphRequest true  "Update graph request"
// @Success     200         {object} updateGraphResponse
// @Failure     400         {object} map[string]string "Invalid request"
// @Failure     500         {object} map[string]string "Internal server error"
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/update [post]
func (a *App) updateGraphHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var req updateGraphRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
	}

	requestID := uuid.New().String()
	if req.RequestID != nil && *req.RequestID != "" {
		requestID = *req.RequestID
	}

	log.Infof(
		"Update graph | workspace=%s mas=%s agent=%s request_id=%s concepts=%d relations=%d",
		workspaceID, masID, req.Header.AgentID, requestID, len(req.Concepts), len(req.Relations),
	)

	// Map to the provider request format
	concepts := make([]iocmemoryprovider.Concept, 0, len(req.Concepts))
	for _, c := range req.Concepts {
		desc := c.Description
		concepts = append(concepts, iocmemoryprovider.Concept{
			ID:          c.ID,
			Name:        c.Name,
			Description: &desc,
			Attributes:  c.Attributes,
		})
	}

	relations := make([]iocmemoryprovider.Relation, 0, len(req.Relations))
	for _, rel := range req.Relations {
		relations = append(relations, iocmemoryprovider.Relation{
			ID:         rel.ID,
			Relation:   rel.Relationship,
			NodeIDs:    rel.NodeIDs,
			Attributes: rel.Attributes,
		})
	}

	storeReq := iocmemoryprovider.NewKnowledgeGraphStoreRequest()
	storeReq.RequestID = requestID
	storeReq.MasID = common.StrToPtr(masID)
	storeReq.WkspID = common.StrToPtr(workspaceID)
	storeReq.SkipNodeIDCheck = true
	storeReq.Records = &iocmemoryprovider.Records{
		Concepts:  concepts,
		Relations: relations,
	}

	_, err := a.knowledgeMemSvcClient.UpsertKnowledgeGraphUpdate(r.Context(), storeReq)
	if err != nil {
		log.Errorf("Update graph failed | workspace=%s mas=%s err=%v", workspaceID, masID, err)
		if errors.Is(err, iocmemoryprovider.ErrNotFound) {
			errMsg := fmt.Sprintf("graph not found for workspace=%s mas=%s", workspaceID, masID)
			return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{"error": errMsg})
		}

		errMsg := fmt.Sprintf("update graph failed: %v", err)
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": errMsg})
	}

	return eh.RespondWithJSON(w, http.StatusOK, updateGraphResponse{
		Header:           req.Header,
		ResponseID:       requestID,
		Error:            nil,
		UpdatedAt:        time.Now().Unix(),
		Descriptor:       req.Descriptor,
		Metadata:         req.Metadata,
		ConceptsUpdated:  len(req.Concepts),
		RelationsUpdated: len(req.Relations),
	})
}
