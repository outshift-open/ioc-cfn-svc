package app

import (
	"encoding/json"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// getCfnDummyHandler godoc
// @Summary		Get CFN dummy data
// @Description	Returns mock CFN data
// @Tags			cfn
// @Produce		json
// @Success		200	{object}	interface{}
// @Router			/api/v1/cfn/dummy [get]
func (a *App) getCfnDummyHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "cfn dummy response",
	})
}

// upsertSharedMemoriesHandler godoc
// @Summary		Upsert shared memories
// @Description	Upserts shared memory entries for a given workspace and multi-agentic system
// @Tags			shared-memories
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		systemId		path		string								true	"Multi-Agentic System ID"
// @Param		body			body		sharedmemory.SharedMemoryUpsertRequest	true	"Upsert request"
// @Success		201				{object}	sharedmemory.SharedMemoryUpsertResponse
// @Failure		400				{object}	map[string]string
// @Failure		500				{object}	map[string]string
// @Router		/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories [post]
func (a *App) upsertSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	// TODO: validate workspaceId and systemId path params
	//workspaceID := eh.PathParam(r, "workspaceId")
	//systemID := eh.PathParam(r, "systemId")

	var req sharedmemory.SharedMemoryUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	// TODO: persist shared memory for (workspaceID, systemID)
	// For now, return a lightweight mock response

	response := sharedmemory.SharedMemoryUpsertResponse{
		Status:  "success",
		Message: "shared memories upserted successfully",
	}

	return eh.RespondWithJSON(w, http.StatusCreated, response)
}

// fetchSharedMemoriesHandler godoc
// @Summary		Fetch shared memories
// @Description	Fetches shared memory entries for a given workspace and multi-agentic system
// @Tags			shared-memories
// @Accept		json
// @Produce		json
// @Param		workspaceId	path		string								true	"Workspace ID"
// @Param		systemId		path		string								true	"Multi-Agentic System ID"
// @Param		body			body		sharedmemory.SharedMemoryQueryRequest	true	"Query request"
// @Success		200				{object}	sharedmemory.SharedMemoryQueryResponse
// @Failure		400				{object}	map[string]string
// @Failure		500				{object}	map[string]string
// @Router		/api/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories/query [post]
func (a *App) fetchSharedMemoriesHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	workspaceID := eh.PathParam(r, "workspaceId")
	systemID := eh.PathParam(r, "systemId")

	var req sharedmemory.SharedMemoryQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
	}

	_ = workspaceID
	_ = systemID
	_ = req
	// TODO: query shared memories for (workspaceID, systemId)

	return eh.RespondWithJSON(w, http.StatusOK, sharedmemory.SharedMemoryQueryResponse{})
}
