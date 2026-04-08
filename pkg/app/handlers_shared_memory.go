package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
		Name: "BAAI/bge-small-en-v1.5", // TODO: it's hardcoded now, need to ask extraction service to return this in response
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

func TransformReasonerResponseToRecords(resp *cognitionagentclient.ReasonerCognitionResponse) *iocmemoryprovider.QueryRecords {
	if resp == nil {
		return nil
	}

	records := &iocmemoryprovider.QueryRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{},
	}

	for _, rec := range resp.Records {
		details := rec.Content.Evidence.Details

		// Concepts
		for _, c := range details.Concepts {
			records.Concepts = append(records.Concepts, iocmemoryprovider.ConceptRecord{
				ID:   c.ConceptID,
				Name: c.Name,
			})
		}
	}

	return records
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
func (a *App) createOrUpdateSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Creating or updating shared memories | workspace=%s mas=%s",
		workspaceID, masID,
	)

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

	requestId := reqPayload.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	extractionPayload := reqPayload.Payload

	extractionReq := &cognitionagentclient.ExtractionRequest{
		Header: common.Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
		},
		RequestID: *requestId,
		Payload:   extractionPayload,
	}

	if reqPayload.Header != nil && reqPayload.Header.AgentID != nil {
		extractionReq.Header.AgentID = *reqPayload.Header.AgentID
	}

	extractionResp, err := a.cognitionAgentsClient.SendExtraction(r.Context(), extractionReq)
	if err != nil {
		log.Errorf("failed to send extraction call, error: %s", err.Error())

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to perform knowledge extraction"},
		)
	}

	log.Debugf("Successfully extracted knowledge, response: %+v", extractionResp)

	memoryProviderReq := &iocmemoryprovider.KnowledgeGraphStoreRequest{
		RequestID:    *requestId,
		WkspID:       &workspaceID,
		MasID:        &masID,
		ForceReplace: true,
		Records:      TransformExtractionResponseToRecords(extractionResp),
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

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to create or update shared memories, error: %v", err)},
		)
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
func (a *App) fetchSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"Fetching shared memories | workspace=%s mas=%s",
		workspaceID, masID,
	)

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

		if err := req.ValidateAndApplyDefault(); err != nil {
			return eh.RespondWithJSON(
				w,
				http.StatusBadRequest,
				map[string]string{"error": err.Error()},
			)
		}
	}

	requestId := req.RequestId
	if requestId == nil {
		requestId = common.StrToPtr(uuid.New().String())
	}

	// Convert []map[string]interface{} to []interface{} for the downstream client
	// This maintains compatibility with the reasoning service which expects []interface{}
	var additionalContext []interface{}
	for _, v := range req.AdditionalContext {
		additionalContext = append(additionalContext, v)
	}

	reasoningRequest := cognitionagentclient.ReasoningEvidenceRequest{
		Header: common.Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     *req.Header.AgentID,
		},
		RequestID: requestId,
		Payload: cognitionagentclient.ReasoningEvidencePayload{
			Metadata: cognitionagentclient.ReasoningEvidencePayloadMetadata{
				QueryType: sharedmemory.SearchStrategyConvertMap[*req.SearchStrategy], // TODO: reasoning endpoint need to its request payload to use "search_strategy"
			},
			Intent:            *req.Intent,
			AdditionalContext: additionalContext,
		},
	}
	if req.Header != nil && req.Header.AgentID != nil {
		reasoningRequest.Header.AgentID = *req.Header.AgentID
	}

	reasonerResp, err := a.cognitionAgentsClient.SendReasoningEvidence(r.Context(), &reasoningRequest)
	if err != nil {
		log.Errorf(
			"SendReasoningEvidence failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "unable to extract concepts from provided message"},
		)
	}

	log.Infof("reasoner response: %+v", reasonerResp)

	conceptsFromReasonerResp := TransformReasonerResponseToRecords(reasonerResp)
	if len(conceptsFromReasonerResp.Concepts) == 0 {
		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("no relevant entities found from user message")},
		)
	}

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()

	memoryProviderReq := &iocmemoryprovider.KnowledgeGraphQueryRequest{
		RequestID: *requestId,
		WkspID:    &workspaceID,
		MasID:     &masID,
		Records:   *conceptsFromReasonerResp,
		//QueryCriteria: req.QueryCriteria,
	}

	queryFns := map[string]func(
		context.Context,
		*iocmemoryprovider.KnowledgeGraphQueryRequest,
	) (*iocmemoryprovider.KnowledgeGraphQueryResponse, error){
		iocmemoryprovider.QueryTypePath:      a.knowledgeMemSvcClient.QueryKnowledgeGraphPath,
		iocmemoryprovider.QueryTypeNeighbour: a.knowledgeMemSvcClient.QueryKnowledgeGraphNeighbor,
		iocmemoryprovider.QueryTypeConcept:   a.knowledgeMemSvcClient.QueryKnowledgeGraphConcept,
	}

	// TODO: not sure if we allow users to specify query type, hence always query "concept" for now
	queryFn, _ := queryFns[iocmemoryprovider.QueryTypeConcept]

	knowledgeGraphResp, err := queryFn(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"Knowledge graph query failed | type=%s workspace=%s mas=%s err=%v",
			iocmemoryprovider.QueryTypeConcept, workspaceID, masID, err,
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

		return eh.RespondWithJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("failed to fetch shared memories: %v", err)},
		)
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

	records := make([]sharedmemory.QueryResponseRecord, 0, len(knowledgeGraphResp.Records))
	for _, r := range knowledgeGraphResp.Records {
		records = append(records, mapKGRecordToQueryRecord(r))
	}

	resp := sharedmemory.QueryResponse{
		ResponseID: knowledgeGraphResp.RequestID,
		Status:     string(knowledgeGraphResp.Status),
		Message:    knowledgeGraphResp.Message,
		Records:    records,
	}

	log.Infof(
		"Shared memories query succeeded | status=%s records=%d",
		resp.Status,
		len(resp.Records),
	)

	return eh.RespondWithJSON(w, http.StatusOK, resp)
}
