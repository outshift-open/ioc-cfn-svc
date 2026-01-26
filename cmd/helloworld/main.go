package main

import (
	stderrors "errors"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/go-errors/errors"

	"github.com/cisco-eti/sre-go-helloworld/pkg/app"
	"github.com/cisco-eti/sre-go-helloworld/pkg/config"
	"github.com/cisco-eti/sre-go-helloworld/pkg/metric"
	"github.com/cisco-eti/sre-go-helloworld/pkg/tools/easyhttp"
	"github.com/cisco-eti/sre-go-helloworld/pkg/tools/logger"
)

// NOTE: this is set at build time to git ref using compile flag. see Makefile
var buildVersion = "<unknown>"

var log = logger.Default()

// @title			Template API
// @version		1.0
// @termsOfService	http://swagger.io/terms/
// @license.name	Apache 2.0
// @BasePath		/
func main() {
	err := run()
	if err != nil {
		log.Fatalf("%s", err)
		// TODO: if dev env, hang process to make debug easier: time.Sleep(time.Hour)
	}
}

func run() error {
	log.Infof("starting helloworld [%s]", buildVersion)
	config.Log()
	defer log.Sync()

	a, err := app.New(buildVersion)
	if err != nil {
		return errors.Errorf("app: %+v", err)
	}

	wg := &sync.WaitGroup{}
	var serverError error

	// start other long running daemons/servers/background tasks here and add to
	// wait group

	wg.Add(1)
	go func() {
		log.Infof("running application http server on port [%d]...", a.Cfg.AppPort)
		serverError = a.Run() // blocks
		wg.Done()
	}()

	wg.Add(1)
	metricServer := easyhttp.NewServer(a.Cfg.MetricsPort, metric.Handler())
	go func() {
		log.Infof("running metric http server on port [%d]...", a.Cfg.MetricsPort)
		err := metricServer.Start()
		if err != nil {
			log.Warnf("error running metric server: %s", err)
		}
		wg.Done()
	}()

	// listen for C-c interrupt
	interruptListener := make(chan os.Signal, 1)
	signal.Notify(interruptListener, syscall.SIGINT, syscall.SIGTERM)
	<-interruptListener // blocks until interrupt
	log.Infof("graceful shutdown signal received")

	// after interrupt, close/flush/cleanup/cancel
	logger.ErrorWrap(metricServer.Stop)
	logger.ErrorWrap(a.Stop)
	wg.Wait() // blocks until all services are stopped
	log.Info("shut down")
	return stderrors.Join(serverError)
}
