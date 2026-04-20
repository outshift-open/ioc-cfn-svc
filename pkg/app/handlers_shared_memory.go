package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}

func transformExtractionConcepts(src []cognitionagentclient.Concept) []iocmemoryprovider.Concept {
	if len(src) == 0 {
		return nil
	}

	out := make([]iocmemoryprovider.Concept, 0, len(src))

	for _, c := range src {
		// Preserve empty string vs nil semantics
		var desc *string
		if c.Description != "" {
			d := jsonEscapeString(c.Description)
			desc = &d
		}

		out = append(out, iocmemoryprovider.Concept{
			ID:          c.ID,
			Name:        c.Name,
			Description: desc,
			Attributes:  transformConceptAttributes(c.Attributes),
			Embeddings:  transformConceptEmbedding(c.Attributes),
		})
	}

	return out
}

func transformConceptAttributes(attrs cognitionagentclient.ConceptAttributes) map[string]interface{} {
	out := make(map[string]interface{})

	// Required / known fields
	out["concept_type"] = attrs.ConceptType

	// Extra attributes
	for k, v := range attrs.Extra {
		out[k] = v
	}

	return out
}

func transformConceptEmbedding(attrs cognitionagentclient.ConceptAttributes) *iocmemoryprovider.EmbeddingConfig {
	if len(attrs.Embedding) == 0 || len(attrs.Embedding[0]) == 0 {
		return nil
	}

	return &iocmemoryprovider.EmbeddingConfig{
		Name: "ibm-granite/granite-embedding-30m-english", // TODO: it's hardcoded now, need to ask extraction service to return this in response
		Data: attrs.Embedding[0],
	}
}

func transformExtractionRelations(src []cognitionagentclient.Relation) []iocmemoryprovider.Relation {
	if len(src) == 0 {
		return nil
	}

	out := make([]iocmemoryprovider.Relation, 0, len(src))

	for _, r := range src {
		out = append(out, iocmemoryprovider.Relation{
			ID:         r.ID,
			Relation:   r.Relationship,
			NodeIDs:    r.NodeIDs,
			Attributes: r.Attributes,
		})
	}

	return out
}

func TransformExtractionResponseToRecords(resp *cognitionagentclient.KnowledgeCognitionResponse) *iocmemoryprovider.Records {
	if resp == nil {
		return nil
	}

	return &iocmemoryprovider.Records{
		Concepts:  transformExtractionConcepts(resp.Concepts),
		Relations: transformExtractionRelations(resp.Relations),
	}
}

// createOrUpdateSharedMemoriesHandler godoc
//
// @Summary     Create or update shared memories.
// @Description Creates or updates shared memories with entries (concepts and relations) extracted from the provided trace or OpenClaw output for a given workspace and multi-agentic system.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body sharedmemory.CreateOrUpdateRequest false "Create or update shared memories request"
//
// @Success     201 {object} sharedmemory.CreateOrUpdateResponse "Shared memories successfully created or updated"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories [post]
// createOrUpdateSharedMemoriesCore contains the core business logic for creating or updating shared memories.
// This function is reused by both HTTP and MCP handlers to avoid code duplication.
func (a *App) createOrUpdateSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.CreateOrUpdateRequest) (*sharedmemory.CreateOrUpdateResponse, error) {
	log := getLogger()

	log.Infof(
		"Creating or updating shared memories | workspace=%s mas=%s",
		workspaceID, masID,
	)

	requestId := req.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	extractionPayload := req.Payload

	extractionReq := &cognitionagentclient.ExtractionRequest{
		Header: common.Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
		},
		RequestID: *requestId,
		Payload:   extractionPayload,
	}

	if req.Header != nil && req.Header.AgentID != nil {
		extractionReq.Header.AgentID = *req.Header.AgentID
	}

	// DEBUG: Log the extraction request being sent
	log.Infof("Sending extraction request to cognition agent: %+v", extractionReq)
	log.Infof("Extraction request payload: %s", string(extractionReq.Payload.Data))

	extractionResp, err := a.cognitionAgentsClient.SendExtraction(ctx, extractionReq)
	if err != nil {
		log.Errorf("failed to send extraction call, error: %s", err.Error())
		return nil, fmt.Errorf("unable to perform knowledge extraction: %w", err)
	}

	log.Debugf("Successfully extracted knowledge, response: %+v", extractionResp)

	memoryProviderReq := &iocmemoryprovider.KnowledgeGraphStoreRequest{
		RequestID:    *requestId,
		WkspID:       &workspaceID,
		MasID:        &masID,
		ForceReplace: true,
		Records:      TransformExtractionResponseToRecords(extractionResp),
	}

	// DEBUG: Print the request being sent to Knowledge Memory Service
	if reqJSON, err := json.MarshalIndent(memoryProviderReq, "", "  "); err == nil {
		fmt.Printf("DEBUG: Sending request to Knowledge Memory Service:\n%s\n", string(reqJSON))
	} else {
		log.Errorf("DEBUG: Failed to marshal request for logging: %v", err)
	}

	// TODO: Revisit audit logging for createOrUpdateSharedMemoriesHandler later.
	// // TODO: operationID is currently a random UUID; replace with a consistent request ID
	// // (e.g. trace ID or correlation ID from the incoming request) once available.
	// operationID := uuid.New().String()
	//
	// // Audit: start of knowledge ingestion
	// startAuditInfo, _ := json.Marshal(map[string]string{
	// 	"status": "STARTED",
	// })
	// startAudit := &audit.Audit{
	// 	OperationID:        &operationID,
	// 	ResourceType:       audit.ResourceTypeMAS,
	// 	ResourceIdentifier: masID,
	// 	AuditType:          audit.AuditTypeKnowledgeIngestion,
	// 	// TODO: AuditResourceIdentifier may change to a different identifier if required.
	// 	AuditResourceIdentifier: masID,
	// 	AuditInformation:        datatypes.JSON(startAuditInfo),
	// 	CreatedBy:               uuid.Nil,
	// 	LastModifiedBy:          uuid.Nil,
	// }
	// if err := a.db.CreateAuditEvent(startAudit); err != nil {
	// 	log.Errorf("failed to create start audit event: %v", err)
	// }

	knowledgeGraphResp, err := a.knowledgeMemSvcClient.UpsertKnowledgeGraph(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"UpsertKnowledgeGraph failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)
		// // Audit: end of knowledge ingestion (failure)
		// errMsg := err.Error()
		// endAuditInfo, _ := json.Marshal(map[string]string{
		// 	"status": "FAILED",
		// 	"error":  errMsg,
		// })
		// endAudit := &audit.Audit{
		// 	OperationID:        &operationID,
		// 	ResourceType:       audit.ResourceTypeMemoryProvider,
		// 	ResourceIdentifier: masID,
		// 	AuditType:          audit.AuditTypeKnowledgeIngestion,
		// 	// TODO: AuditResourceIdentifier may change to a different identifier if required.
		// 	AuditResourceIdentifier: masID,
		// 	AuditInformation:        datatypes.JSON(endAuditInfo),
		// 	AuditExtraInformation:   &errMsg,
		// 	CreatedBy:               uuid.Nil,
		// 	LastModifiedBy:          uuid.Nil,
		// }
		// if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
		// 	log.Errorf("failed to create end audit event: %v", auditErr)
		// }

		return nil, fmt.Errorf("failed to create or update shared memories: %w", err)
	}

	// // Audit: end of knowledge ingestion (success)
	// endAuditInfo, _ := json.Marshal(map[string]string{
	// 	"status": "SUCCESS",
	// })
	// endAudit := &audit.Audit{
	// 	OperationID:        &operationID,
	// 	ResourceType:       audit.ResourceTypeMemoryProvider,
	// 	ResourceIdentifier: masID,
	// 	AuditType:          audit.AuditTypeKnowledgeIngestion,
	// 	// TODO: AuditResourceIdentifier may change to a different identifier if required.
	// 	AuditResourceIdentifier: masID,
	// 	AuditInformation:        datatypes.JSON(endAuditInfo),
	// 	CreatedBy:               uuid.Nil,
	// 	LastModifiedBy:          uuid.Nil,
	// }
	// if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
	// 	log.Errorf("failed to create end audit event: %v", auditErr)
	// }

	resp := &sharedmemory.CreateOrUpdateResponse{
		ResponseID: knowledgeGraphResp.RequestID,
		Status:     string(knowledgeGraphResp.Status),
		Message:    knowledgeGraphResp.Message,
	}

	return resp, nil
}

// createOrUpdateSharedMemoriesHandler handles HTTP requests for creating or updating shared memories.
// It parses the HTTP request and delegates to the core business logic function.
func (a *App) createOrUpdateSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var reqPayload sharedmemory.CreateOrUpdateRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Call core business logic
	resp, err := a.createOrUpdateSharedMemoriesCore(ctx, workspaceID, masID, reqPayload)
	if err != nil {
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": err.Error()},
		)
	}

	return eh.RespondWithJSON(w, http.StatusCreated, resp)
}

func mapKGRecordToQueryRecord(r iocmemoryprovider.KnowledgeGraphQueryResponseRecord) sharedmemory.QueryResponseRecord {

	out := sharedmemory.QueryResponseRecord{}

	for _, c := range r.Concepts {
		out.Concepts = append(out.Concepts, sharedmemory.QueryConcept{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Attributes:  c.Attributes,
			Tags:        c.Tags,
		})
	}

	for _, rel := range r.Relationships {
		out.Relationships = append(out.Relationships, sharedmemory.QueryRelation{
			ID:         rel.ID,
			Relation:   rel.Relation,
			NodeIDs:    rel.NodeIDs,
			Attributes: rel.Attributes,
		})
	}

	return out
}

// fetchSharedMemoriesHandler godoc
//
// @Summary     Fetch shared memories
// @Description Queries shared memories for a given workspace and multi-agentic system using a graph path query.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body sharedmemory.QueryRequest true "Query request"
//
// @Success     200 {object} sharedmemory.QueryResponse  "Query executed successfully"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query [post]
// fetchSharedMemoriesCore contains the core business logic for fetching shared memories
// This function is reused by both HTTP and MCP handlers to avoid code duplication.
func (a *App) fetchSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.QueryRequest) (*sharedmemory.QueryResponse, error) {
	log := getLogger()

	// Validate and apply defaults
	if err := req.ValidateAndApplyDefault(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	requestId := req.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	agentID := ""
	if req.Header != nil && req.Header.AgentID != nil {
		agentID = *req.Header.AgentID
	}

	log.Infof(
		"Fetching shared memories | workspace=%s mas=%s request_id=%s agent_id=%s",
		workspaceID, masID, *requestId, agentID,
	)

	reasoningRequest := cognitionagentclient.ReasoningEvidenceRequest{
		Header: common.Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     agentID,
		},
		RequestID: requestId,
		Payload: cognitionagentclient.ReasoningEvidencePayload{
			Intent: *req.Intent,
		},
	}

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()

	// Log the request to cognition agents client
	log.Infof("COGNITION_AGENTS_REQUEST | workspace=%s mas=%s operation_id=%s intent=%s agent_id=%s",
		workspaceID, masID, operationID, *req.Intent, agentID)
	//log.Infof("COGNITION_AGENTS_REQUEST_FULL | request=%+v", reasoningRequest)

	reasonerResp, err := a.cognitionAgentsClient.SendReasoningEvidence(ctx, &reasoningRequest)
	if err != nil {
		log.Errorf(
			"Failed to process evidence | workspace=%s mas=%s operation_id=%s err=%v",
			workspaceID, masID, operationID, err,
		)

		// Audit: shared memory query (failure)
		errMsg := err.Error()
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		// Hacky: fetch shared_memory.id from summary API on first audit call.
		// TODO: Remove once IDs are available directly in CfnConfig global map.
		ensureAuditResourceIDs()
		auditResID := SharedMemoryID
		if auditResID == "" {
			auditResID = masID
		}
		endAudit := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMAS,
			ResourceIdentifier:      masID,
			AuditType:               audit.AuditTypeSharedMemoryOperation,
			AuditResourceIdentifier: auditResID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create audit event: %v", auditErr)
		}

		return nil, fmt.Errorf("failed to process evidence: %w", err)
	}

	// Log the response from cognition agents client
	log.Infof("COGNITION_AGENTS_RESPONSE | workspace=%s mas=%s operation_id=%s records_count=%d",
		workspaceID, masID, operationID, len(reasonerResp.Records))
	//log.Infof("COGNITION_AGENTS_RESPONSE_FULL | response=%+v", reasonerResp)

	// Extract evidence fields from first record
	var evidenceStatus, finalResponse string
	if len(reasonerResp.Records) > 0 {
		ev := reasonerResp.Records[0].Content.Evidence
		evidenceStatus = ev.Status
		finalResponse = ev.FinalResponse
	}

	const insufficientEvidenceMsg = "The evidence does not support an answer to this question."
	if finalResponse == insufficientEvidenceMsg {
		log.Errorf(
			"Insufficient evidence to answer user intent | workspace=%s mas=%s",
			workspaceID, masID,
		)

		// Audit: shared memory query (insufficient evidence)
		errMsg := "Insufficient evidence to answer provided user intent"
		endAuditInfo, _ := json.Marshal(map[string]string{
			"status": "FAILED",
			"error":  errMsg,
		})
		ensureAuditResourceIDs()
		auditResID := SharedMemoryID
		if auditResID == "" {
			auditResID = masID
		}
		endAudit := &audit.Audit{
			OperationID:             &operationID,
			ResourceType:            audit.ResourceTypeMAS,
			ResourceIdentifier:      masID,
			AuditType:               audit.AuditTypeSharedMemoryOperation,
			AuditResourceIdentifier: auditResID,
			AuditInformation:        datatypes.JSON(endAuditInfo),
			AuditExtraInformation:   &errMsg,
			CreatedBy:               uuid.Nil,
			LastModifiedBy:          uuid.Nil,
		}
		if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
			log.Errorf("failed to create audit event: %v", auditErr)
		}

		return nil, fmt.Errorf("insufficient evidence to answer provided user intent")
	}

	var message string
	switch {
	case finalResponse != "":
		message = finalResponse
	case evidenceStatus != "":
		message = fmt.Sprintf("Evidence status: %s", evidenceStatus)
	default:
		message = "evidence processed"
	}

	// Audit: shared memory query (success)
	endAuditInfo, _ := json.Marshal(map[string]string{
		"status": "SUCCESS",
	})
	// Hacky: fetch shared_memory.id from summary API on first audit call.
	// TODO: Remove once IDs are available directly in CfnConfig global map.
	ensureAuditResourceIDs()
	successAuditResID := SharedMemoryID
	if successAuditResID == "" {
		successAuditResID = masID
	}
	endAudit := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      masID,
		AuditType:               audit.AuditTypeSharedMemoryOperation,
		AuditResourceIdentifier: successAuditResID,
		AuditInformation:        datatypes.JSON(endAuditInfo),
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(endAudit); auditErr != nil {
		log.Errorf("failed to create audit event: %v", auditErr)
	}

	log.Infof("Fetch shared memories succeeded | workspace=%s mas=%s", workspaceID, masID)

	return &sharedmemory.QueryResponse{
		ResponseID: requestId,
		Message:    common.StrToPtr(message),
	}, nil
}

func (a *App) fetchSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	var req sharedmemory.QueryRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			log.Errorf("invalid JSON body: %s", err)
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	// Call core business logic
	response, err := a.fetchSharedMemoriesCore(r.Context(), workspaceID, masID, req)
	if err != nil {
		if strings.Contains(err.Error(), "insufficient evidence") {
			return eh.RespondWithJSON(
				w,
				http.StatusNotFound,
				map[string]string{"error": err.Error()},
			)
		}
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": err.Error()},
		)
	}

	return eh.RespondWithJSON(w, http.StatusOK, response)
}

// Get neighbors by concept ID, returns the neighboring concepts of a given concept in the knowledge graph.
// Internal API - not exposed in public Swagger documentation.
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

// Fetch concepts by IDs, returns concept details for each of the provided concept IDs.
// Internal API - not exposed in public Swagger documentation.
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

//	@Summary	Fetch paths between two concepts, returns ordered paths through the knowledge graph between a source and target concept.
//
// Internal API - not exposed in public Swagger documentation.
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

// onboardSharedMemoriesVectorStoreHandler godoc
//
// @Summary     Onboards the shared memory vector store.
// @Description Onboards the shared memory vector store.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       body        body sharedmemory.OnboardVectorStoreRequest true "Onboard vector store request"
//
// @Success     201 {object} sharedmemory.OnboardVectorStoreResponse "Vector Store successfully onboarded"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/shared-memories/vector-store [post]
func (a *App) onboardSharedMemoriesVectorStoreHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	// only workspace is used for vector store onboarding
	workspaceID := eh.PathParam(r, "workspaceId")

	log.Infof(
		"onboarding shared memory store | workspace=%s",
		workspaceID,
	)

	var reqPayload sharedmemory.OnboardVectorStoreRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	requestId := reqPayload.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	memoryProviderReq := &iocmemoryprovider.KnowledgeVectorStoreOnboardRequest{
		RequestID: *requestId,
		WkspID:    workspaceID,
	}

	response, err := a.knowledgeMemSvcClient.OnboardKnowledgeVectorStore(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"OnboardKnowledgeVectorStore failed | workspace=%s err=%v",
			workspaceID, err,
		)
		if response != nil {
			responseJSON, _ := json.Marshal(response)
			log.Infof(
				"OnboardKnowledgeVectorStore response | workspace=%s response=%s",
				workspaceID, string(responseJSON),
			)
		}
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to onboard knowledge vector store, error: %v", err)},
		)
	}

	resp := &sharedmemory.OnboardVectorStoreResponse{
		ResponseID: requestId,
		Status:     string(response.Status),
		Message:    response.Message,
		StoreId:    &workspaceID,
	}

	return eh.RespondWithJSON(w, http.StatusCreated, resp)
}

// deleteSharedMemoriesVectorStoreHandler godoc
//
// @Summary     Deletes the shared memory vector store.
// @Description Deletes the shared memory vector store.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       store_id    path string true "Store ID"
// @Param       body        body sharedmemory.DeleteVectorStoreRequest true "Delete vector store request"
//
// @Success     200 {object} sharedmemory.DeleteVectorStoreResponse "Vector Store successfully deleted"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/workspaces/{workspaceId}/shared-memories/vector-store/{store_id} [delete]
func (a *App) deleteSharedMemoriesVectorStoreHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	// Extract path parameters
	workspaceID := eh.PathParam(r, "workspaceId")
	storeID := eh.PathParam(r, "store_id")

	log.Infof(
		"deleting shared memory store | workspace=%s store_id=%s",
		workspaceID, storeID,
	)

	var reqPayload sharedmemory.DeleteVectorStoreRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil && err != io.EOF {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": "invalid JSON body"},
			)
		}
	}

	requestId := reqPayload.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	memoryProviderReq := &iocmemoryprovider.KnowledgeVectorStoreOnboardDeleteRequest{
		RequestID: *requestId,
		WkspID:    workspaceID,
	}

	response, err := a.knowledgeMemSvcClient.DeleteKnowledgeVectorStore(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"DeleteKnowledgeVectorStore failed | workspace=%s store_id=%s err=%v",
			workspaceID, storeID, err,
		)
		if response != nil {
			responseJSON, _ := json.Marshal(response)
			log.Infof(
				"DeleteKnowledgeVectorStore response | workspace=%s store_id=%s response=%s",
				workspaceID, storeID, string(responseJSON),
			)
		}
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to delete knowledge vector store, error: %v", err)},
		)
	}

	resp := &sharedmemory.DeleteVectorStoreResponse{
		ResponseID: requestId,
		Status:     string(response.Status),
		Message:    response.Message,
		StoreId:    &storeID,
	}

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}

// CreateOrUpdateSharedMemoriesCore implements the McpService interface.
// This method provides access to the core business logic for creating or updating shared memories.
func (a *App) CreateOrUpdateSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.CreateOrUpdateRequest) (*sharedmemory.CreateOrUpdateResponse, error) {
	return a.createOrUpdateSharedMemoriesCore(ctx, workspaceID, masID, req)
}

// FetchSharedMemoriesCore implements the McpService interface.
// This method provides access to the core business logic for fetching shared memories.
func (a *App) FetchSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.QueryRequest) (*sharedmemory.QueryResponse, error) {
	return a.fetchSharedMemoriesCore(ctx, workspaceID, masID, req)
}
