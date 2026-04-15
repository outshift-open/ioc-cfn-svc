package app

import (
	"net/http"

	api "github.com/cisco-eti/ioc-cfn-svc/pkg/generated/api"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

const (
	apiPrefix      = "/api"
	internalPrefix = "/api/internal"
)

func (a *App) initializeRoutes() http.Handler {
	log := getLogger()

	// Create chi router for OpenAPI-generated handlers
	chiRouter := chi.NewRouter()

	// Custom logging middleware for chi
	chiRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("incoming request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	// Mount OpenAPI-generated handlers
	adapter := newOpenAPIAdapter(a)
	chiHandler := api.HandlerFromMux(adapter, chiRouter)

	// Create easyhttp router for internal endpoints and backward compatibility
	rtr := easyhttp.NewRouter()

	// Mount chi-generated public API routes (schema-first endpoints)
	rtr.HandleHTTP(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/", chiHandler)
	rtr.HandleHTTP(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/", chiHandler)
	rtr.HandleHTTP(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/", chiHandler)

	// NEW endpoints from main (not yet in OpenAPI spec - using easyhttp temporarily)
	// TODO: Add these to docs/openapi.yaml and migrate to schema-first
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/shared-memories/vector-store", a.onboardSharedMemoriesVectorStoreHandler)
	rtr.Delete(internalPrefix+"/workspaces/{workspaceId}/shared-memories/vector-store/{store_id}", a.deleteSharedMemoriesVectorStoreHandler)
	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors", a.cognitionAgentsSharedMemoriesVectorsUpsertHandler)
	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors/search", a.cognitionAgentsSharedMemoriesVectorsSearchHandler)

	// Internal diagnostic endpoints
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)
	rtr.Get(internalPrefix+"/diagnostics/info", a.diagnosticsInfoHandler)
	rtr.Get(internalPrefix+"/diagnostics/metrics", a.diagnosticsMetricsHandler)
	rtr.Get(internalPrefix+"/diagnostics/loggers", a.diagnosticsLoggersHandler)
	rtr.Put(internalPrefix+"/diagnostics/loggers", a.diagnosticsSetLoggersHandler)

	// Internal graph endpoints (not in OpenAPI spec)
	rtr.Get(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/neighbors/{conceptId}", a.getNeighborsByIdHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/concepts/by_ids", a.fetchConceptsByIdsHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/paths", a.fetchPathsByIdsHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/concepts/similarity-search", a.conceptSimilaritySearchHandler)

	// Internal audit events
	rtr.Get(internalPrefix+"/mgmt/audit", a.listAuditEventsHandler)
	rtr.Get(internalPrefix+"/mgmt/audit/{eventId}", a.getAuditEventHandler)

	// Serve OpenAPI spec
	rtr.HandleHTTP("/openapi.yaml", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/openapi.yaml")
	}))

	// Swagger UI (schema-first)
	rtr.HandleHTTP("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/openapi.yaml"),
	))

	return rtr
}
