package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var log = logger.SubPkg("app")

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

type App struct {
	buildVersion string
	Cfg          config.Config
	server       *easyhttp.EasyServer

	readyForRequests *atomic.Bool
	stopChan         chan struct{}

	// integrated client
	db client.Database
	s3 client.S3
}

func New(buildVersion string) (*App, error) {
	cfg := config.Get()

	var db client.Database
	var s3 client.S3

	var err error
	if cfg.DB.Enabled() {
		db, err = database.New(cfg.DB)
		if err != nil {
			return nil, err
		}
	} else {
		db = client.NewMockDatabase()
	}

	err = db.MigrateUp()
	if err != nil {
		return nil, err
	}

	s3 = client.NewMockS3()
	a := &App{
		buildVersion:     buildVersion,
		Cfg:              *cfg,
		readyForRequests: &atomic.Bool{},
		stopChan:         make(chan struct{}),
		db:               db,
		s3:               s3,
	}

	rtr := a.initializeRoutes()
	a.server = easyhttp.NewServer(a.Cfg.AppPort, rtr)

	a.registerOnStartup()
	return a, nil
}

// registerOnStartup calls home to mgmt plane to register this service.
func (a *App) registerOnStartup() {
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:8000")
	// TODO: revisit - won't need these eventually to connect to mgmt plane
	workspaceID := getEnvOrDefault("WORKSPACE_ID", "workspace-id")
	apiKey := getEnvOrDefault("X_API_KEY", "x-api-key")
	cfnID := a.Cfg.CfnID // UUID generated on app startup
	cfnName := getEnvOrDefault("CFN_NAME", "cfn-local")

	if mgmtURL == "" || workspaceID == "" || apiKey == "" {
		log.Warnf("skipping registration: MGMT_URL, WORKSPACE_ID, or X_API_KEY not set")
		return
	}

	if cfnID == "" || cfnName == "" {
		log.Warnf("skipping registration: CFN_ID or CFN_NAME not set")
		return
	}

	registerURL := mgmtURL + "/api/workspaces/" + workspaceID + "/cognitive-fabric-node/register"
	log.Infof("registering CFN at %s", registerURL)

	body, _ := json.Marshal(map[string]any{
		"cfn_id":     cfnID,
		"cfn_name":   cfnName,
		"cfn_config": map[string]any{},
	})

	client := httpclient.New(30 * time.Second)
	ctx := context.Background()
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    apiKey,
	}

	resp, err := client.Post(ctx, registerURL, body, headers)
	if err != nil {
		log.Fatalf("CFN registration failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("failed to decode registration response: %v", err)
	}
	log.Infof("CFN registered successfully, response=%v", result)

	// Start periodic heartbeat
	go a.startHeartbeat(mgmtURL, workspaceID, cfnID, apiKey)
}

// startHeartbeat sends periodic heartbeat to mgmt plane to keep CFN status active.
// It runs in a goroutine and sends PUT requests at the configured interval (default 29s).
// The heartbeat stops when the app's stopChan is closed during shutdown.
// TODO: revisit - workspaceID and apiKey won't be needed eventually to connect to mgmt plane
func (a *App) startHeartbeat(mgmtURL, workspaceID, cfnID, apiKey string) {
	// Build heartbeat endpoint URL
	heartbeatURL := mgmtURL + "/api/workspaces/" + workspaceID + "/cognitive-fabric-node/" + cfnID + "/heartbeat"

	// Get heartbeat interval from env or use default of 29 seconds
	intervalStr := getEnvOrDefault("HEARTBEAT_INTERVAL_SECONDS", "29")
	interval, err := time.ParseDuration(intervalStr + "s")
	if err != nil {
		interval = 29 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Create HTTP client with 10s timeout for heartbeat requests
	client := httpclient.New(10 * time.Second)
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    apiKey,
	}

	log.Infof("starting heartbeat to %s", heartbeatURL)

	for {
		select {
		case <-a.stopChan:
			// App is shutting down, stop heartbeat
			log.Info("stopping heartbeat")
			return
		case <-ticker.C:
			// Send heartbeat request
			ctx := context.Background()
			resp, err := client.Put(ctx, heartbeatURL, nil, headers)
			if err != nil {
				log.Errorf("heartbeat failed: %v", err)
				continue
			}
			resp.Body.Close()
			log.Infof("heartbeat sent, status=%d", resp.StatusCode)
		}
	}
}

// Run starts the app and serves on the specified addr. this is synchronous and
// blocks
func (a *App) Run() error {
	wg := sync.WaitGroup{}
	wg.Add(1)
	var serverErr error
	go func() {
		defer wg.Done()
		log.Infof("starting the web server")
		serverErr = a.server.Start() // blocks
		a.readyForRequests.Store(false)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("starting a long running background job")
		a.LongRunningBackgroundJob() // blocks
	}()

	a.readyForRequests.Store(true)
	wg.Wait()
	return serverErr
}

// Stop stops the app and closes connections to all resources
func (a *App) Stop() error {
	log.Infof("shutting down %s...", a.Cfg.ServiceName)
	close(a.stopChan)
	log.Info("- stopping http server")
	err1 := a.server.Stop()
	log.Info("- closing connection to db")
	err2 := a.db.Close()
	return errors.Join(err1, err2)
}
