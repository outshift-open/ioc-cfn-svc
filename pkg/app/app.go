package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	mem0client "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/mem0"
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
	gitCommitSHA  string
	gitCommitTime string
	gitBranch     string
	Cfg          config.Config
	server       *easyhttp.EasyServer

	readyForRequests *atomic.Bool
	stopChan         chan struct{}

	// integrated clients
	db         client.Database
	s3         client.S3
	mem0Client *mem0client.Client
}

func New(buildVersion, gitCommitSHA, gitCommitTime, gitBranch string) (*App, error) {
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

	// Initialise the mem0 Agentic Memory Client (optional — runs without it if not configured)
	var mem0 *mem0client.Client
	mem0APIKey := os.Getenv("MEM0_API_KEY")
	if mem0APIKey != "" {
		mem0Cfg := mem0client.DefaultClientConfig()
		mem0Cfg.APIKey = mem0APIKey // sourced from environment, never hardcoded
		if u := os.Getenv("MEM0_BASE_URL"); u != "" {
			mem0Cfg.BaseURL = u
		}
		if orgID := os.Getenv("MEM0_ORG_ID"); orgID != "" {
			mem0Cfg.OrgID = orgID
		}
		if projID := os.Getenv("MEM0_PROJECT_ID"); projID != "" {
			mem0Cfg.ProjectID = projID
		}

		var mem0Err error
		mem0, mem0Err = mem0client.NewClient(mem0Cfg)
		if mem0Err != nil {
			log.Warnf("mem0 agentic memory client init failed (memory operations will be unavailable): %v", mem0Err)
		} else {
			log.Infof("mem0 agentic memory client initialised successfully")
		}
	} else {
		log.Infof("MEM0_API_KEY not set — agentic memory client disabled")
	}

	a := &App{
		buildVersion:     buildVersion,
		gitCommitSHA:     gitCommitSHA,
		gitCommitTime:    gitCommitTime,
		gitBranch:        gitBranch,
		Cfg:              *cfg,
		readyForRequests: &atomic.Bool{},
		stopChan:         make(chan struct{}),
		db:               db,
		s3:               s3,
		mem0Client:       mem0,
	}

	rtr := a.initializeRoutes()
	a.server = easyhttp.NewServer(a.Cfg.AppPort, rtr)

	a.registerOnStartup()
	return a, nil
}

// registerOnStartup calls home to mgmt plane to register this service.
func (a *App) registerOnStartup() {
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	cfnID := a.Cfg.CfnID // UUID generated on app startup
	cfnName := getEnvOrDefault("CFN_NAME", "cfn-local")

	if mgmtURL == "" {
		log.Fatalf("MGMT_URL not set")
	}

	if cfnID == "" || cfnName == "" {
		log.Fatalf("CFN_ID or CFN_NAME not set")
	}

	registerURL := mgmtURL + "/api/cognitive-fabric-nodes/register"
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
		"Accept":       "application/json",
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Fatalf("CFN registration failed: status=%d, response=%v", resp.StatusCode, result)
	}
	log.Infof("CFN registered successfully, response=%v", result)

	// Start periodic heartbeat
	go a.startHeartbeat(mgmtURL, cfnID)
}

// startHeartbeat sends periodic heartbeat to mgmt plane to keep CFN status active.
// It runs in a goroutine and sends PUT requests at the configured interval (default 29s).
// The heartbeat stops when the app's stopChan is closed during shutdown.
func (a *App) startHeartbeat(mgmtURL, cfnID string) {
	// Build heartbeat endpoint URL
	heartbeatURL := mgmtURL + "/api/cognitive-fabric-nodes/" + cfnID + "/heartbeat"

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
		"Accept": "application/json",
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
			if resp.StatusCode == http.StatusOK {
				log.Info("heartbeat successful")
				log.Debugf("heartbeat successful, url=%s, status=%d", heartbeatURL, resp.StatusCode)
			} else {
				log.Errorf("heartbeat failed, status=%d", resp.StatusCode)
			}
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
