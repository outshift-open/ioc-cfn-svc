package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app"
	mcpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/mcp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var buildVersion = "dev"

var log = logger.Default()

// @title			Template API
// @version		1.0
// @BasePath		/
func main() {
	log.Infof("starting and running service [%s]", buildVersion)
	config.Log()
	defer log.Sync()

	a, err := app.New(buildVersion)
	if err != nil {
		log.Fatalf("failed to create app: %v", err)
	}

	// Start server in background
	if os.Getenv("MCP_ENABLED") == "true" {
		go func() {
			cfg := mcpclient.ServerConfigFromEnv()
			log.Infof("MCP server listening on %s", cfg.Addr())
			mcpclient.RunServer(cfg)
		}()
	} else {
		go func() {
			log.Infof("http server listening on port %d", a.Cfg.AppPort)
			if err := a.Run(); err != nil {
				log.Errorf("server error: %v", err)
			}
		}()
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("shutting down...")
	if err := a.Stop(); err != nil {
		log.Errorf("shutdown error: %v", err)
	}
	log.Info("shutdown complete")
}
