package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app"
	mcpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/mcp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"github.com/joho/godotenv"
	"github.com/namsral/flag"
)

var buildVersion = "dev"
var gitCommitSHA = "unknown"
var gitCommitTime = "unknown"
var gitBranch = "unknown"

// @title		CFN Service API
// @version		1.0
// @description	IoC Cognition Fabric Node service — shared memory routing and memory operations proxy.
// @BasePath	/
func main() {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()
	// Parse flags BEFORE logger is used
	flag.Parse()
	logger.Init()
	log := logger.Default()
	defer log.Sync()

	log.Infof("starting and running service [%s] commit=[%s] time=[%s] branch=[%s]", buildVersion, gitCommitSHA, gitCommitTime, gitBranch)

	config.Log()

	a, err := app.New(buildVersion, gitCommitSHA, gitCommitTime, gitBranch)
	if err != nil {
		log.Fatalf("failed to create app: %v", err)
	}

	// Start both HTTP and MCP servers concurrently with proper error handling
	var wg sync.WaitGroup
	errorChan := make(chan error, 2)

	// Start HTTP REST server
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("starting HTTP server on port %d", a.Cfg.AppPort)
		if err := a.Run(); err != nil {
			log.Errorf("HTTP server error: %v", err)
			errorChan <- fmt.Errorf("HTTP server failed: %w", err)
		}
	}()

	// Start MCP server for AI tool integration
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("MCP server panic: %v", r)
				errorChan <- fmt.Errorf("MCP server panic: %v", r)
			}
		}()
		cfg := mcpclient.ServerConfigs(a)
		log.Infof("starting MCP server on port %d", a.Cfg.McpPort)
		mcpclient.RunServer(cfg) // This calls log.Fatalf on error, so we need to catch panics
	}()

	// Wait for either shutdown signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Info("received shutdown signal")
	case err := <-errorChan:
		log.Errorf("server error: %v", err)
	}

	log.Info("shutting down...")
	if err := a.Stop(); err != nil {
		log.Errorf("shutdown error: %v", err)
	}

	// Wait for goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("all servers stopped")
	case <-time.After(3 * time.Second):
		log.Warn("timeout waiting for servers to stop")
	}
	log.Info("shutdown complete")
}
