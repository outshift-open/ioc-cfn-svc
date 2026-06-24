// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"net/http"

	"github.com/outshift-open/ioc-cfn-svc/pkg/app/httpapi/cognitionagents"
	"github.com/outshift-open/ioc-cfn-svc/pkg/common"
	iocmemoryprovider "github.com/outshift-open/ioc-cfn-svc/pkg/providers/memory/ioc"
	eh "github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
)

// TODO: Handler logic, API route, and request/response structs may change based on
// core logic implementation and final API design.
// TODO: Add audit CRUD operations for cognition agents memory queries.

// respondWithError creates a standardized error response
func (a *App) respondWithError(w http.ResponseWriter, status int, header common.Header, message string, details map[string]interface{}) (int, error) {
	return eh.RespondWithJSON(w, status, cognitionagents.SharedMemoryVectorsResponse{
		Header: header,
		Error: &common.ErrorDetail{
			Message: message,
			Detail:  details,
		},
	})
}

// cognitionAgentsSharedMemoriesVectorsUpsertHandler godoc
// @Summary		Performs upsert for shared memory vectors
// @Description	Performs upsert for shared memory vectors for a cognition agent
// @Tags			cognition-agents
// @Accept		json
// @Produce		json
// @Param		cfnId	    path		string									true	"CFN ID"
// @Param		body	    body		cognitionagents.SharedMemoryVectorsRequest	    true	"Shared Memory Request"
// @Success		201		    {object}	cognitionagents.SharedMemoryVectorsResponse
// @Failure		400		    {object}	cognitionagents.SharedMemoryVectorsResponse
// @Failure		500		    {object}	cognitionagents.SharedMemoryVectorsResponse
func (a *App) cognitionAgentsSharedMemoriesVectorsUpsertHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	_ = r.PathValue("cfnId") // available for future routing/validation

	var req cognitionagents.SharedMemoryVectorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return a.respondWithError(w, http.StatusBadRequest, common.Header{}, "invalid JSON body",
			map[string]interface{}{"error": err.Error()})
	}

	// Validation required in cfn service
	if req.Header.WorkspaceID == "" || req.Header.MASID == "" {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "workspace_id and mas_id are mandatory", nil)
	}

	// Use the existing knowledge memory service client from App struct
	if a.knowledgeMemSvcClient == nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Knowledge memory service client not initialized", nil)
	}

	if req.Type != cognitionagents.ReqTypeKnowledgeVectorsUpsert {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "Invalid request type",
			map[string]interface{}{"request_type": string(req.Type)})
	}

	// Parse request body directly from json.RawMessage
	var vectorRequest iocmemoryprovider.KnowledgeVectorStoreRequest
	if err := json.Unmarshal(*req.Body, &vectorRequest); err != nil {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "Failed to parse request",
			map[string]interface{}{"error": err.Error()})
	}

	// Call UpsertKnowledgeVectors method
	response, err := a.knowledgeMemSvcClient.UpsertKnowledgeVectors(r.Context(), &vectorRequest)

	// Handle nil response or error
	if response == nil || err != nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Failed request",
			map[string]interface{}{"error": err.Error()})
	}

	// Marshal response to json.RawMessage
	resultsBytes, err := json.Marshal(response)
	if err != nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Failed to marshal response",
			map[string]interface{}{"error": err.Error()})
	}

	return eh.RespondWithJSON(w, http.StatusCreated, cognitionagents.SharedMemoryVectorsResponse{
		Header:  req.Header,
		Results: (*json.RawMessage)(&resultsBytes),
	})
}

// cognitionAgentsSharedMemoriesVectorsSearchHandler godoc
// @Summary		Performs search for shared memory vectors
// @Description	Performs search for shared memory vectors for a cognition agent
// @Tags			cognition-agents
// @Accept		json
// @Produce		json
// @Param		cfnId	    path		string									true	"CFN ID"
// @Param		body	    body		cognitionagents.SharedMemoryVectorsRequest	    true	"Shared Memory Request"
// @Success		200			{object}	cognitionagents.SharedMemoryVectorsResponse
// @Failure		400		    {object}	cognitionagents.SharedMemoryVectorsResponse
// @Failure		500		    {object}	cognitionagents.SharedMemoryVectorsResponse
func (a *App) cognitionAgentsSharedMemoriesVectorsSearchHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	_ = r.PathValue("cfnId") // available for future routing/validation

	var req cognitionagents.SharedMemoryVectorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return a.respondWithError(w, http.StatusBadRequest, common.Header{}, "invalid JSON body",
			map[string]interface{}{"error": err.Error()})
	}

	// Validation required in cfn service
	if req.Header.WorkspaceID == "" || req.Header.MASID == "" {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "workspace_id and mas_id are mandatory", nil)
	}

	// Use the existing knowledge memory service client from App struct
	if a.knowledgeMemSvcClient == nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Knowledge memory service client not initialized", nil)
	}

	if req.Type != cognitionagents.ReqTypeKnowledgeVectorsQuery {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "Invalid request type",
			map[string]interface{}{"request_type": string(req.Type)})
	}

	// Parse request body directly from json.RawMessage
	var vectorRequest iocmemoryprovider.KnowledgeVectorQueryRequest
	if err := json.Unmarshal(*req.Body, &vectorRequest); err != nil {
		return a.respondWithError(w, http.StatusBadRequest, req.Header, "Failed to parse request",
			map[string]interface{}{"error": err.Error()})
	}

	// Call QueryKnowledgeVectors method
	response, err := a.knowledgeMemSvcClient.QueryKnowledgeVectors(r.Context(), &vectorRequest)

	// Handle nil response or error
	if response == nil || err != nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Failed request",
			map[string]interface{}{"error": err.Error()})
	}

	// Marshal response to json.RawMessage
	resultsBytes, err := json.Marshal(response)
	if err != nil {
		return a.respondWithError(w, http.StatusInternalServerError, req.Header, "Failed to marshal response",
			map[string]interface{}{"error": err.Error()})
	}

	return eh.RespondWithJSON(w, http.StatusOK, cognitionagents.SharedMemoryVectorsResponse{
		Header:  req.Header,
		Results: (*json.RawMessage)(&resultsBytes),
	})
}
