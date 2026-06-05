package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

// logSharedMemoryAudit creates an audit event for shared-memory operations.
// It resolves the shared memory ID from the typed config (falling back to
// masID) and logs any audit-creation errors without propagating them.
func (a *App) logSharedMemoryAudit(operationID, workspaceID, masID, auditType, status string, errMsg *string) {
	log := getLogger()

	auditResID := getSharedMemoryID(workspaceID, masID)
	if auditResID == "" {
		auditResID = masID
	}

	info := map[string]string{"status": status}
	if errMsg != nil {
		info["error"] = *errMsg
	}
	auditInfo, _ := json.Marshal(info)

	ev := &audit.Audit{
		OperationID:             &operationID,
		ResourceType:            audit.ResourceTypeMAS,
		ResourceIdentifier:      masID,
		AuditType:               auditType,
		AuditResourceIdentifier: auditResID,
		AuditInformation:        datatypes.JSON(auditInfo),
		AuditExtraInformation:   errMsg,
		CreatedBy:               uuid.Nil,
		LastModifiedBy:          uuid.Nil,
	}
	if auditErr := a.db.CreateAuditEvent(ev); auditErr != nil {
		log.Errorf("failed to create audit event: %v", auditErr)
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

	// TODO: operationID is currently a random UUID; replace with a consistent request ID
	// (e.g. trace ID or correlation ID from the incoming request) once available.
	operationID := uuid.New().String()

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

	extractionResp, err := a.cognitionAgentsClient.SendExtraction(ctx, extractionReq)
	if err != nil {
		log.Errorf("failed to send extraction call, error: %s", err.Error())

		errMsg := err.Error()
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeKnowledgeIngestion, "FAILED", &errMsg)

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

	knowledgeGraphResp, err := a.knowledgeMemSvcClient.UpsertKnowledgeGraph(ctx, memoryProviderReq)
	if err != nil {
		log.Errorf(
			"UpsertKnowledgeGraph failed | workspace=%s mas=%s err=%v",
			workspaceID, masID, err,
		)

		errMsg := err.Error()
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeKnowledgeIngestion, "FAILED", &errMsg)

		return nil, fmt.Errorf("failed to create or update shared memories: %w", err)
	}

	// Upsert RAG chunks into vector DB if present in extraction response
	var vectorStoreMessage *string
	if vectorRecords := transformRagChunksToVectorRecords(workspaceID, masID, extractionResp.RagChunks); len(vectorRecords) > 0 {
		vectorStoreReq := iocmemoryprovider.NewKnowledgeVectorStoreRequest(workspaceID, masID, vectorRecords)
		if vectorResp, vectorErr := a.knowledgeMemSvcClient.UpsertKnowledgeVectors(ctx, vectorStoreReq); vectorErr != nil {
			// Non-fatal: graph upsert already succeeded, log and continue

			log.Errorf(
				"UpsertKnowledgeVectors failed (non-fatal) | workspace=%s mas=%s err=%v",
				workspaceID, masID, vectorErr,
			)

			errMsg := vectorErr.Error()
			a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeKnowledgeIngestion, "FAILED", &errMsg)
		} else if vectorResp != nil {
			vectorStoreMessage = vectorResp.Message
		}
	}

	a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeKnowledgeIngestion, "SUCCESS", nil)

	// Pass through token metadata from cognition engine if available
	var responseMeta *common.TokenUsageMeta
	if extractionResp.Meta != nil {
		responseMeta = &common.TokenUsageMeta{
			Tokens: common.TokenUsage{
				Prompt:     extractionResp.Meta.Tokens.Prompt,
				Completion: extractionResp.Meta.Tokens.Completion,
				Total:      extractionResp.Meta.Tokens.Total,
				Model:      extractionResp.Meta.Tokens.Model,
			},
			LatencyMs: extractionResp.Meta.LatencyMs,
			CostUsd:   extractionResp.Meta.CostUsd,
			Timestamp: extractionResp.Meta.Timestamp,
		}

		// Store token metrics to TimescaleDB (fire-and-forget)
		workspaceUUID, _ := uuid.Parse(workspaceID)
		masUUID, _ := uuid.Parse(masID)
		agentID := ""
		if req.Header != nil && req.Header.AgentID != nil {
			agentID = *req.Header.AgentID
		}
		// Extract CE ID from response metadata
		var ceID *uuid.UUID
		if extractionResp.Meta.CEID != "" {
			if parsed, err := uuid.Parse(extractionResp.Meta.CEID); err == nil {
				ceID = &parsed
			} else {
				log.Warnf("Invalid CE ID in response metadata: %s", extractionResp.Meta.CEID)
			}
		}
		a.storeTokenMetricsAsync(
			workspaceUUID,
			masUUID,
			agentID,
			"ingestion",
			*requestId,
			ceID, // Now passes actual CE ID from response
			&common.TokenUsageMeta{
				Tokens: common.TokenUsage{
					Prompt:     extractionResp.Meta.Tokens.Prompt,
					Completion: extractionResp.Meta.Tokens.Completion,
					Total:      extractionResp.Meta.Tokens.Total,
					Model:      extractionResp.Meta.Tokens.Model,
				},
				LatencyMs: extractionResp.Meta.LatencyMs,
				CostUsd:   extractionResp.Meta.CostUsd,
				Timestamp: extractionResp.Meta.Timestamp,
			},
		)
	}

	resp := &sharedmemory.CreateOrUpdateResponse{
		ResponseID:         knowledgeGraphResp.RequestID,
		Status:             string(knowledgeGraphResp.Status),
		GraphStoreMessage:  knowledgeGraphResp.Message,
		VectorStoreMessage: vectorStoreMessage,
		Meta:               responseMeta,
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

	reasonerResp, err := a.cognitionAgentsClient.SendReasoningEvidence(ctx, &reasoningRequest)
	if err != nil {
		log.Errorf(
			"Failed to process evidence | workspace=%s mas=%s operation_id=%s err=%v",
			workspaceID, masID, operationID, err,
		)

		errMsg := err.Error()
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSharedMemoryOperation, "FAILED", &errMsg)

		return nil, fmt.Errorf("failed to process evidence: %w", err)
	}

	if b, err := json.Marshal(reasonerResp); err == nil {
		log.Debugf("Evidence gathering response: %s", b)
	} else {
		log.Debugf("Evidence gathering response: %+v", reasonerResp)
	}

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

		errMsg := "Insufficient evidence to answer provided user intent"
		a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSharedMemoryOperation, "FAILED", &errMsg)

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

	a.logSharedMemoryAudit(operationID, workspaceID, masID, audit.AuditTypeSharedMemoryOperation, "SUCCESS", nil)

	log.Infof("Fetch shared memories succeeded | workspace=%s mas=%s", workspaceID, masID)

	// Pass through token metadata from cognition engine if available
	var responseMeta *common.TokenUsageMeta
	if reasonerResp.Meta != nil {
		responseMeta = &common.TokenUsageMeta{
			Tokens: common.TokenUsage{
				Prompt:     reasonerResp.Meta.Tokens.Prompt,
				Completion: reasonerResp.Meta.Tokens.Completion,
				Total:      reasonerResp.Meta.Tokens.Total,
				Model:      reasonerResp.Meta.Tokens.Model,
			},
			LatencyMs: reasonerResp.Meta.LatencyMs,
			CostUsd:   reasonerResp.Meta.CostUsd,
			Timestamp: reasonerResp.Meta.Timestamp,
		}

		// Store token metrics to TimescaleDB (fire-and-forget)
		workspaceUUID, _ := uuid.Parse(workspaceID)
		masUUID, _ := uuid.Parse(masID)
		// Extract CE ID from response metadata
		var ceID *uuid.UUID
		if reasonerResp.Meta.CEID != "" {
			if parsed, err := uuid.Parse(reasonerResp.Meta.CEID); err == nil {
				ceID = &parsed
			} else {
				log.Warnf("Invalid CE ID in response metadata: %s", reasonerResp.Meta.CEID)
			}
		}
		a.storeTokenMetricsAsync(
			workspaceUUID,
			masUUID,
			agentID,
			"evidence",
			*requestId,
			ceID, // Now passes actual CE ID from response
			&common.TokenUsageMeta{
				Tokens: common.TokenUsage{
					Prompt:     reasonerResp.Meta.Tokens.Prompt,
					Completion: reasonerResp.Meta.Tokens.Completion,
					Total:      reasonerResp.Meta.Tokens.Total,
					Model:      reasonerResp.Meta.Tokens.Model,
				},
				LatencyMs: reasonerResp.Meta.LatencyMs,
				CostUsd:   reasonerResp.Meta.CostUsd,
				Timestamp: reasonerResp.Meta.Timestamp,
			},
		)
	}

	return &sharedmemory.QueryResponse{
		ResponseID: requestId,
		Message:    common.StrToPtr(message),
		Meta:       responseMeta,
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

// onboardSharedMemoriesVectorStoreHandler godoc
//
// @Summary     Onboards the shared memory vector store.
// @Description Onboards the shared memory vector store for a given MAS. The store is scoped per-MAS.
//
// @Tags        Vector Store
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
// @Router      /api/internal/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/vector-store [post]
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
// @Tags        Vector Store
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
