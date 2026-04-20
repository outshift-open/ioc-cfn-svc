package app

import (
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// docsFS serves static files from the docs/ directory.
var docsFS = http.FileServer(http.Dir("docs"))

const (
	apiPrefix      = "/api"
	internalPrefix = "/api/internal"
)

func (a *App) initializeRoutes() http.Handler {
	log := getLogger()

	rtr := easyhttp.NewRouter()

	// custom middleware - log all incoming requests
	rtr.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("incoming request: %s %s", r.Method, r.URL.Path)
			h.ServeHTTP(w, r)
		})
	})

	// standard diagnostic endpoints
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)
	rtr.Get(internalPrefix+"/diagnostics/info", a.diagnosticsInfoHandler)
	rtr.Get(internalPrefix+"/diagnostics/metrics", a.diagnosticsMetricsHandler)
	rtr.Get(internalPrefix+"/diagnostics/loggers", a.diagnosticsLoggersHandler)
	rtr.Put(internalPrefix+"/diagnostics/loggers", a.diagnosticsSetLoggersHandler)

	// shared memories
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/shared-memories/vector-store", a.onboardSharedMemoriesVectorStoreHandler)
	rtr.Delete(internalPrefix+"/workspaces/{workspaceId}/shared-memories/vector-store/{store_id}", a.deleteSharedMemoriesVectorStoreHandler)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories", a.createOrUpdateSharedMemoriesHandler)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query", a.fetchSharedMemoriesHandler)

	rtr.Get(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/neighbors/{conceptId}", a.getNeighborsByIdHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/concepts/by_ids", a.fetchConceptsByIdsHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/paths", a.fetchPathsByIdsHandler)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/concepts/similarity-search", a.conceptSimilaritySearchHandler)

	// semantic negotiation
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/start", a.startSemanticNegotiationHandler)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/decide", a.decideSemanticNegotiationHandler)

	// remote agent memory operations
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations", a.memoryOperationsHandler)

	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors", a.cognitionAgentsSharedMemoriesVectorsUpsertHandler)
	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors/search", a.cognitionAgentsSharedMemoriesVectorsSearchHandler)

	// audit events (internal API)
	rtr.Get(internalPrefix+"/mgmt/audit", a.listAuditEventsHandler)
	rtr.Get(internalPrefix+"/mgmt/audit/{eventId}", a.getAuditEventHandler)

	// Public Swagger UI — points to post-split swagger.json (public endpoints only)
	rtr.HandleHTTP("/docs/swagger.json", http.StripPrefix("/docs/", docsFS))
	rtr.HandleHTTP("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/docs/swagger.json"),
	))

	// Internal Swagger UI — points to swagger-internal.json (internal endpoints only)
	rtr.HandleHTTP("/docs/internal/swagger-internal.json", http.StripPrefix("/docs/internal/", docsFS))
	rtr.HandleHTTP("/docs/internal/", httpSwagger.Handler(
		httpSwagger.URL("/docs/internal/swagger-internal.json"),
	))

	return rtr
}
