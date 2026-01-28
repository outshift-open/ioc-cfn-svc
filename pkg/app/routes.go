package app

import (
	"net/http"
	"path/filepath"

	"github.com/cisco-eti/sre-go-helloworld/pkg/tools/easyhttp"
)

const (
	v1Prefix = "/api/v1"
)

func v1(path string) string {
	return filepath.Join(v1Prefix, path)
}

func (a *App) initializeRoutes() http.Handler {
	rtr := easyhttp.NewRouter()

	// custom middleware
	rtr.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("checking for something in request: [%s]", r.URL.Query().Get("x"))
			h.ServeHTTP(w, r)
		})
	})

	// diagnostic endpoints
	rtr.Get("/healthz", a.healthHandler)
	rtr.Get("/ready", a.readyHandler)
	// {$} for exact match. see https://pkg.go.dev/net/http#hdr-Patterns-ServeMux
	rtr.Get("/{$}", a.healthHandler)

	// foo endpoints
	rtr.Post(v1("/foo"), a.createFooHandler)
	rtr.Get(v1("/foo/{id}"), a.getFooHandler)

	// cfn endpoints
	rtr.Get(v1("/cfn/dummy"), a.getCfnDummyHandler)

	return rtr
}
