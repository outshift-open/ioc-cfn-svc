package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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

var (
	// CfnID is the globally stored CFN identifier returned by the management plane on registration.
	CfnID string
	// CfnConfig is the config blob returned by the management plane on registration.
	CfnConfig map[string]any
	// CfnTimestamp tracks the config timestamp for detecting config changes during heartbeat.
	CfnTimestamp string
	// cfnConfigMutex protects concurrent access to CfnConfig and CfnTimestamp.
	cfnConfigMutex sync.RWMutex
)

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Errorf("failed to determine outbound IP: %v", err)
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

type App struct {
	buildVersion  string
	gitCommitSHA  string
	gitCommitTime string
	gitBranch     string
	Cfg           config.Config
	server        *easyhttp.EasyServer

	readyForRequests *atomic.Bool
	stopChan         chan struct{}

	// integrated client
	db client.Database
	s3 client.S3
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
	}

	rtr := a.initializeRoutes()
	a.server = easyhttp.NewServer(a.Cfg.AppPort, rtr)

	a.registerOnStartup()
	return a, nil
}

// registerOnStartup calls home to mgmt plane to register this service.
func (a *App) registerOnStartup() {
	mgmtURL := getEnvOrDefault("MGMT_URL", "http://localhost:9000")
	cfnName := getEnvOrDefault("CFN_NAME", "cfn-local")
	appIP := getOutboundIP()
	appPortStr := strings.TrimPrefix(a.server.Addr, ":")
	appPort, err := strconv.Atoi(appPortStr)
	if err != nil {
		log.Fatalf("invalid port %q: %v", appPortStr, err)
	}

	if mgmtURL == "" {
		log.Fatalf("MGMT_URL not set")
	}

	if cfnName == "" || appIP == "" || appPortStr == "" {
		log.Fatalf("registration prereqs missing: cfnName=%q appIP=%q appPort=%d", cfnName, appIP, appPort)
	}

	registerURL := mgmtURL + "/api/cognitive-fabric-nodes/register"
	log.Infof("registering CFN at %s", registerURL)

	body, _ := json.Marshal(map[string]any{
		"cfn_name":   cfnName,
		"ip_address": appIP,
		"port":       appPort,
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

	// Store cfn_id from response globally
	id, ok := result["cfn_id"].(string)
	if !ok || id == "" {
		log.Fatalf("registration response missing cfn_id")
	}
	CfnID = id

	// Store config blob and timestamp from response globally
	cfnConfigMutex.Lock()
	if cfgBlob, ok := result["config"].(map[string]any); ok {
		CfnConfig = cfgBlob
		// Extract and store config_timestamp
		if timestamp, ok := cfgBlob["config_timestamp"].(string); ok {
			CfnTimestamp = timestamp
		}
	}
	cfnConfigMutex.Unlock()

	log.Infof("CFN registered successfully: cfn_id=%s cfn_name=%s ip_address=%s port=%d config=%v timestamp=%s", CfnID, cfnName, appIP, appPort, CfnConfig, CfnTimestamp)

	// Start periodic heartbeat
	go a.startHeartbeat(mgmtURL)
}

// RefreshConfig fetches the latest CFN configuration from the management plane
// and updates the global CfnConfig.
func (a *App) RefreshConfig(mgmtURL string) error {
	cfnURL := mgmtURL + "/api/cognitive-fabric-nodes/" + CfnID

	client := httpclient.New(30 * time.Second)
	ctx := context.Background()
	headers := map[string]string{
		"Accept": "application/json",
	}

	resp, err := client.Get(ctx, cfnURL, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Errorf("RefreshConfig failed: status=%d response=%v", resp.StatusCode, result)
		return fmt.Errorf("RefreshConfig failed: status=%d", resp.StatusCode)
	}

	cfnConfigMutex.Lock()
	defer cfnConfigMutex.Unlock()

	if cfgBlob, ok := result["config"].(map[string]any); ok {
		CfnConfig = cfgBlob
		// Update timestamp
		if timestamp, ok := cfgBlob["config_timestamp"].(string); ok {
			CfnTimestamp = timestamp
		}
		log.Infof("CfnConfig refreshed: %v timestamp=%s", CfnConfig, CfnTimestamp)
	} else {
		log.Warnf("RefreshConfig response missing config key")
	}

	return nil
}

// startHeartbeat sends periodic heartbeat to mgmt plane to keep CFN status active.
// It runs in a goroutine and sends PUT requests at the configured interval (default 29s).
// The heartbeat stops when the app's stopChan is closed during shutdown.
func (a *App) startHeartbeat(mgmtURL string) {
	// Build heartbeat endpoint URL using the globally stored CfnID
	heartbeatURL := mgmtURL + "/api/cognitive-fabric-nodes/" + CfnID + "/heartbeat"

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

			if resp.StatusCode == http.StatusOK {
				// Decode response to check for config changes
				var result map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					log.Errorf("failed to decode heartbeat response: %v", err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				// Check if config_timestamp has changed
				if newTimestamp, ok := result["config_timestamp"].(string); ok {
					cfnConfigMutex.RLock()
					currentTimestamp := CfnTimestamp
					cfnConfigMutex.RUnlock()

					// Parse timestamps as time.Time for proper comparison
					newTime, err := time.Parse(time.RFC3339Nano, newTimestamp)
					if err != nil {
						log.Errorf("failed to parse heartbeat timestamp: %v", err)
						continue
					}

					currentTime, err := time.Parse(time.RFC3339Nano, currentTimestamp)
					if err != nil {
						log.Errorf("failed to parse current timestamp: %v", err)
						continue
					}

					// Refresh config if timestamp has changed
					if !newTime.Equal(currentTime) {
						log.Infof("config timestamp changed, refreshing config")
						if err := a.RefreshConfig(mgmtURL); err != nil {
							log.Errorf("failed to refresh config: %v", err)
						}
					}
				}

				log.Info("heartbeat successful")
				log.Debugf("heartbeat successful, url=%s, status=%d", heartbeatURL, resp.StatusCode)
			} else {
				resp.Body.Close()
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
