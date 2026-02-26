package easyhttp

import (
	"fmt"
	"net/http"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"go.uber.org/zap"
)

var l *zap.SugaredLogger

func getLogger() *zap.SugaredLogger {
	if l == nil {
		l = logger.SubPkg("app")
	}
	return l
}

type Router struct {
	mux         *http.ServeMux
	middlewares []middleware
	wrappedMux  http.Handler
}

type middleware func(http.Handler) http.Handler

// NewRouter initializes a base router with important error, logging, and panic
// recovering middleware
func NewRouter() *Router {
	mux := http.NewServeMux()
	rtr := &Router{
		mux:         mux,
		middlewares: []middleware{},
		// ensures even the 404 and 405 handlers are logged
		wrappedMux: loggingMiddleware(panicRecoverMiddleware(mux)),
	}

	// register the most generic wildcard root path with custom not found handler.
	// all handlers with more specific routes will match before this
	// (not specifying a method applies it to all http methods)
	rtr.Handle("/", func(http.ResponseWriter, *http.Request) (int, error) {
		return http.StatusNotFound, nil
	})

	return rtr
}

func (rtr *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rtr.wrappedMux.ServeHTTP(w, r)
}

// Use registers middleware to be used
func (rtr *Router) Use(mw middleware) {
	rtr.middlewares = append(rtr.middlewares, mw)
}

func (rtr *Router) Get(path string, eh easyHandler) {
	rtr.byMethod(http.MethodGet, path, eh)
}

func (rtr *Router) Post(path string, eh easyHandler) {
	rtr.byMethod(http.MethodPost, path, eh)
}

func (rtr *Router) Put(path string, eh easyHandler) {
	rtr.byMethod(http.MethodPut, path, eh)
}

func (rtr *Router) Patch(path string, eh easyHandler) {
	rtr.byMethod(http.MethodPatch, path, eh)
}

func (rtr *Router) Delete(path string, eh easyHandler) {
	rtr.byMethod(http.MethodDelete, path, eh)
}

func (rtr *Router) Handle(path string, eh easyHandler) {
	rtr.byMethod("", path, eh)
}

// byMethod registers a method + handler for a specific path, applying all
// middleware
func (rtr *Router) byMethod(method, path string, eh easyHandler) {
	h := wrapToEasyHandler(eh)
	for i := len(rtr.middlewares) - 1; i >= 0; i-- {
		h = rtr.middlewares[i](h)
	}
	rtr.mux.Handle(fmt.Sprintf("%s %s", method, path), h)
}

// easyHandler is an http handler method signature that lets you write more
// straightforward functions, directly returning an http status code and
// possible error
type easyHandler func(http.ResponseWriter, *http.Request) (int, error)

// wrapToEasyHandler wraps a given standard http handler with an error handler
// that returns a proper error message/status to the caller
func wrapToEasyHandler(h easyHandler) http.Handler {
	log := getLogger()
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &cleanResponseWriter{ResponseWriter: w}
		code, err := h(cw, r)
		if err == nil { // reverse logic
			if !cw.wroteHeader {
				if code == 0 {
					code = http.StatusOK
				}
				RespondWithJSON(w, code,
					map[string]string{"status": http.StatusText(code)})
			}
			return
		}

		if code >= http.StatusInternalServerError && code != http.StatusNotImplemented {
			// print the stack trace if it's an unexpected 500
			log.Errorf("[%s %s %d] error: %+v", r.Method, r.URL, code, err)
		} else {
			// just print out error message
			log.Infof("[%s %s %d] error: %s", r.Method, r.URL, code, err)
		}

		RespondWithJSON(w, code, map[string]string{"error": err.Error()})
	})
}
