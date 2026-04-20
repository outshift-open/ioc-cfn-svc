package app

import (
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

// ragChunkNamespace is the UUID v5 namespace for deriving deterministic chunk IDs.
var ragChunkNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // uuid.NameSpaceURL

func transformRagChunksToVectorRecords(wkspID, masID string, chunks []cognitionagentclient.RagChunk) []iocmemoryprovider.KnowledgeVectorStoreRequestRecord {
	if len(chunks) == 0 {
		return nil
	}

	records := make([]iocmemoryprovider.KnowledgeVectorStoreRequestRecord, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Embedding) == 0 || len(chunk.Embedding[0]) == 0 {
			continue
		}

		// Deterministic ID derived from (wksp, mas, source, doc, chunk) so that
		// re-ingesting the same chunk hits ON CONFLICT (id) DO UPDATE instead of inserting a duplicate.
		chunkKey := fmt.Sprintf("%s:%s:%s:%d:%d", wkspID, masID, chunk.Metadata.Domain, chunk.Metadata.DocIndex, chunk.Metadata.ChunkIndex)
		chunkID := uuid.NewSHA1(ragChunkNamespace, []byte(chunkKey)).String()

		metadata := map[string]interface{}{
			"data_source": chunk.Metadata.Domain,
			"recorded_at": chunk.Metadata.Timestamp,
			"doc_index":   chunk.Metadata.DocIndex,
			"chunk_index": chunk.Metadata.ChunkIndex,
		}

		records = append(records, iocmemoryprovider.KnowledgeVectorStoreRequestRecord{
			ID:        chunkID,
			Content:   chunk.Text,
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{Data: chunk.Embedding[0]},
			Metadata:  metadata,
		})
	}

	return records
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

	log.Infof("Successfully extracted knowledge, response: %+v", extractionResp)

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

	// Upsert RAG chunks into vector DB if present in extraction response
	var vectorStoreMessage *string
	if vectorRecords := transformRagChunksToVectorRecords(workspaceID, masID, extractionResp.RagChunks); len(vectorRecords) > 0 {
		vectorStoreReq := iocmemoryprovider.NewKnowledgeVectorStoreRequest(workspaceID, masID, vectorRecords)
		if vectorResp, vectorErr := a.knowledgeMemSvcClient.UpsertKnowledgeVectors(ctx, vectorStoreReq); vectorErr != nil {
			log.Errorf(
				"UpsertKnowledgeVectors failed (non-fatal) | workspace=%s mas=%s err=%v",
				workspaceID, masID, vectorErr,
			)
			// Non-fatal: graph upsert already succeeded, log and continue
		} else if vectorResp != nil {
			vectorStoreMessage = vectorResp.Message
		}
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
		ResponseID:         knowledgeGraphResp.RequestID,
		Status:             string(knowledgeGraphResp.Status),
		GraphStoreMessage:  knowledgeGraphResp.Message,
		VectorStoreMessage: vectorStoreMessage,
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

	reasonerResp, err := a.cognitionAgentsClient.SendReasoningEvidence(r.Context(), &reasoningRequest)
	if err != nil {
		log.Errorf(
			"Failed to process evidence | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
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
			map[string]string{"error": fmt.Sprintf("failed to process evidence: %v", err)},
		)
	}

	log.Debugf("Evidence gathering response: %+v", reasonerResp)

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

		return eh.RespondWithJSON(
			w,
			http.StatusNotFound,
			map[string]string{"error": "Insufficient evidence to answer provided user intent"},
		)
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

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.QueryResponse{
		ResponseID: requestId,
		Message:    common.StrToPtr(message),
	})
}

// onboardSharedMemoriesVectorStoreHandler godoc
//
// @Summary     Onboards the shared memory vector store.
// @Description Onboards the shared memory vector store for a given MAS. The store is scoped per-MAS.
//
// @Tags        shared-memories
// @Accept      json
// @Produce     json
//
// @Param       workspaceId path string true "Workspace ID"
// @Param       masId       path string true "Multi-Agentic System ID"
// @Param       body        body object true "Onboard vector store request"
//
// @Success     201 {object} object "Vector Store successfully onboarded"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/vector-store [post]
func (a *App) onboardSharedMemoriesVectorStoreHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

	workspaceID := eh.PathParam(r, "workspaceId")
	masID := eh.PathParam(r, "masId")

	log.Infof(
		"onboarding shared memory store | workspace=%s mas=%s",
		workspaceID, masID,
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
		MasID:     masID,
	}

	response, err := a.knowledgeMemSvcClient.OnboardKnowledgeVectorStore(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"OnboardKnowledgeVectorStore failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)
		if response != nil {
			responseJSON, _ := json.Marshal(response)
			log.Infof(
				"OnboardKnowledgeVectorStore response | workspace=%s mas=%s response=%s",
				workspaceID, masID, string(responseJSON),
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
		StoreId:    &masID,
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
// @Param       body        body object true "Delete vector store request"
//
// @Success     200 {object} object "Vector Store successfully deleted"
// @Failure     400 {object} map[string]string "Invalid request"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/workspaces/{workspaceId}/shared-memories/vector-store/{store_id} [delete]
func (a *App) deleteSharedMemoriesVectorStoreHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()
	ctx := r.Context()

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
		MasID:     storeID,
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
