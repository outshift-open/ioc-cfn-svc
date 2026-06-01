package easyhttp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

func serverTimeoutDuration() time.Duration {
	sec, _ := strconv.Atoi(os.Getenv("SERVER_TIMEOUT_SECONDS"))
	if sec <= 0 {
		sec = 120
	}
	return time.Duration(sec) * time.Second
}

type EasyServer struct {
	*http.Server
}

func NewServer(port int, handler http.Handler) *EasyServer {
	return &EasyServer{Server: &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  serverTimeoutDuration(),
		WriteTimeout: serverTimeoutDuration(),
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
	log := getLogger()

	log.Infof("shutting down web server on port [%s]", es.Server.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), serverTimeoutDuration())
	defer cancel()
	return es.Server.Shutdown(ctx)
}
