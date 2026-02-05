package easyhttp

import (
	"fmt"
	"net/http"
	"time"
)

// panicRecoverMiddleware will recover from a panic within a handler to prevent
// the server from completely crashing
func panicRecoverMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				panicMsg := fmt.Sprintf("recovered from panic: [%s]", panicErr)
				log.Error(panicMsg)
				http.Error(w, panicMsg, http.StatusInternalServerError)
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// loggingMiddleware will log details for all incoming requests
func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		uri := r.RequestURI // r.URL.String()
		userAgent := r.UserAgent()

		q := r.URL.Query()
		if q.Get("log") == "false" {
			// don't log requests with ?log=false query param
			// helpful for endpoint like /healthz or /metrics
			log.Debugf("%s %s ->", method, uri)
			h.ServeHTTP(w, r)
			return
		}

		remoteAddr := r.RemoteAddr
		referer := r.Referer()

		log.Infof("%s %s ->", method, uri)

		cw := &cleanResponseWriter{ResponseWriter: w}

		start := time.Now()
		h.ServeHTTP(cw, r)
		dur := time.Since(start)

		responseStatus := fmt.Sprintf("%d - %s", cw.responseStatusCode,
			http.StatusText(cw.responseStatusCode))
		log.Infof("-> [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s]",
			remoteAddr,
			start.Format(time.RFC3339), // unnecessary?
			method,
			uri,
			responseStatus,
			fmt.Sprintf("%db", cw.responseSize),
			dur,
			referer,
			userAgent,
		)
	})
}

type cleanResponseWriter struct {
	http.ResponseWriter
	responseStatusCode int
	responseSize       int
	wroteHeader        bool
}

func (w *cleanResponseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.responseSize += size
	return size, err
}

func (w *cleanResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.responseStatusCode = statusCode
	w.wroteHeader = true
}

/*
// https://github.com/golang/go/issues/65648#issuecomment-2282740908
func main() {
	rtr := http.NewServeMux()
	// ...

	http.ListenAndServe(":8080", BodyOverride(rtr))
}

type BodyOverrider struct {
	http.ResponseWriter

	code     int
	override bool
}

func (b *BodyOverrider) WriteHeader(code int) {
	if b.Header().Get("Content-Type") == "text/plain; charset=utf-8" {
		b.Header().Set("Content-Type", "application/json")

		b.override = true
	}

	b.code = code
	b.ResponseWriter.WriteHeader(code)
}

func (b *BodyOverrider) Write(body []byte) (int, error) {
	if b.override {
		switch b.code {
		case http.StatusNotFound:
			body = []byte(`{"code": "route_not_found"}`)
		case http.StatusMethodNotAllowed:
			body = []byte(`{"code": "method_not_allowed"}`)
		}
	}

	return b.ResponseWriter.Write(body)
}
*/
