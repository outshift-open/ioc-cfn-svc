package app

import (
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
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
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)
	rtr.Get(internalPrefix+"/diagnostics/loggers", a.diagnosticsLoggersHandler)
	rtr.Post(internalPrefix+"/diagnostics/loggers", a.diagnosticsSetLoggersHandler)

	// cfn endpoints
	rtr.Get(apiPrefix+"/cfn/dummy", a.getCfnDummyHandler)

	return rtr
}
