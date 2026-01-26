package easyhttp

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

var serverTimeout = 120 * time.Second

type EasyServer struct {
	*http.Server
}

func NewServer(port int, handler http.Handler) *EasyServer {
	return &EasyServer{Server: &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}}
}

func (es *EasyServer) Start() error {
	err := es.Server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (es *EasyServer) Stop() error {
	log.Infof("shutting down web server on port [%s]", es.Server.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), serverTimeout)
	defer cancel()
	return es.Server.Shutdown(ctx)
}
