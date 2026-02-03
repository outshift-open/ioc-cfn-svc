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

	// custom middleware
	rtr.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("checking for something in request: [%s]", r.URL.Query().Get("x"))
			h.ServeHTTP(w, r)
		})
	})

	// TKF standard diagnostic endpoint
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)

	// cfn endpoints
	rtr.Get(apiPrefix+"/cfn/dummy", a.getCfnDummyHandler)

	return rtr
}
