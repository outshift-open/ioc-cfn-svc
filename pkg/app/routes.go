package app

import (
	"net/http"

	_ "github.com/cisco-eti/ioc-cfn-svc/docs"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

const (
	apiPrefix      = "/api"
	internalPrefix = "/api/internal"
)

func (a *App) initializeRoutes() http.Handler {
	rtr := easyhttp.NewRouter()

	// custom middleware - log all incoming requests
	rtr.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("incoming request: %s %s", r.Method, r.URL.Path)
			h.ServeHTTP(w, r)
		})
	})

	// TKF standard diagnostic endpoints
	rtr.Get(internalPrefix+"/diagnostics/info", a.diagnosticsInfoHandler)
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)
	rtr.Get(internalPrefix+"/diagnostics/loggers", a.diagnosticsLoggersHandler)
	rtr.Post(internalPrefix+"/diagnostics/loggers", a.diagnosticsSetLoggersHandler)

	// cfn endpoints
	rtr.Get(apiPrefix+"/cfn/dummy", a.getCfnDummyHandler)

	// shared memories
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories", a.upsertSharedMemoriesHandler)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{systemId}/shared-memories/query", a.fetchSharedMemoriesHandler)

	// audit events (internal API)
	rtr.Post(internalPrefix+"/audit-events", a.createAuditEventHandler)
	rtr.Get(internalPrefix+"/audit-events", a.listAuditEventsHandler)
	rtr.Get(internalPrefix+"/audit-events/{eventId}", a.getAuditEventHandler)
	rtr.Delete(internalPrefix+"/audit-events/{eventId}", a.deleteAuditEventHandler)

	// Swagger UI + spec
	rtr.HandleHTTP("/docs/", httpSwagger.WrapHandler)

	return rtr
}
