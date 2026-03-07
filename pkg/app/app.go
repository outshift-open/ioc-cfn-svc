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
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"go.uber.org/zap"
)

var (
	l    *zap.SugaredLogger
	once sync.Once
)

func getLogger() *zap.SugaredLogger {
	once.Do(func() {
		l = logger.SubPkg("app")
	})
	return l
}

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

const (
	// CognitionEngineKnowledgeManagement is the name for the Knowledge Management Cognition Engine
	CognitionEngineKnowledgeManagement = "Knowledge Management Cognitive Engine"
	// CognitionEngineSemanticNegotiation is the name for the Semantic Negotiation Cognition Engine
	CognitionEngineSemanticNegotiation = "Semantic Negotiation Cognitive Engine"
	// DefaultWorkspaceName is the workspace name to search for when multiple workspaces exist
	DefaultWorkspaceName = "Default Workspace"
)

// getOutboundIP determines the service's outbound IP address by querying network interfaces.
// It prefers the CFN_IP environment variable if set, then falls back to detecting the first
// non-loopback IPv4 address from active network interfaces.
func getOutboundIP() string {
	log := getLogger()
	// Allow explicit override via environment variable (useful for Docker/K8s)
	if ip := os.Getenv("CFN_IP"); ip != "" {
		log.Infof("using CFN_IP from environment: %s", ip)
		return ip
	}

	// Query network interfaces and pick the first non-loopback IPv4 address
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Errorf("failed to query network interfaces: %v", err)
		return "127.0.0.1"
	}

	for _, iface := range interfaces {
		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip if not IPv4 or is loopback
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not IPv4
			}

			log.Infof("detected service IP from interface %s: %s", iface.Name, ip.String())
			return ip.String()
		}
	}

	log.Warnf("no suitable network interface found, falling back to 127.0.0.1")
	return "127.0.0.1"
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

	// integrated clients
	db                client.Database
	s3                client.S3
	memoryProxyClient *httpclient.Client

	knowledgeMemSvcClient *iocmemoryprovider.Client
}

func New(buildVersion, gitCommitSHA, gitCommitTime, gitBranch string) (*App, error) {
	cfg := config.Get()
	log := getLogger()

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

	// Initialise generic HTTP client for memory provider proxying.
	// 5-minute timeout (memory operations involve LLM calls), no retries (prevents duplicate POSTs).
	// Auth credentials come from management plane config per-agent, not env vars.
	memoryCfg := httpclient.DefaultConfig()
	memoryCfg.Timeout = 5 * time.Minute
	memoryCfg.MaxRetries = 0
	memoryCfg.RetryableFunc = func(resp *http.Response, err error) bool { return false }
	memoryProxyClient := httpclient.NewWithConfig(memoryCfg)
	log.Infof("memory proxy HTTP client initialised")

	knowledgeMemURL := getEnvOrDefault("KNOWLEDGE_MEMORY_SVC_URL", "http://localhost:9003")
	log.Infof("knowledge memory service URL: %s", knowledgeMemURL)
	knowledgeMemClient, err := iocmemoryprovider.NewClient(knowledgeMemURL)
	if err != nil {
		log.Fatalf("Failed to create knowledge memory client: %v", err)
	}

	a := &App{
		buildVersion:          buildVersion,
		gitCommitSHA:          gitCommitSHA,
		gitCommitTime:         gitCommitTime,
		gitBranch:             gitBranch,
		Cfg:                   *cfg,
		readyForRequests:      &atomic.Bool{},
		stopChan:              make(chan struct{}),
		db:                    db,
		s3:                    s3,
		memoryProxyClient:     memoryProxyClient,
		knowledgeMemSvcClient: knowledgeMemClient,
	}

	rtr := a.initializeRoutes()
	a.server = easyhttp.NewServer(a.Cfg.AppPort, rtr)

	a.registerOnStartup()
	return a, nil
}

// registerOnStartup calls home to mgmt plane to register this service.
func (a *App) registerOnStartup() {
	log := getLogger()
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

	// Register Cognition Engines
	a.registerCognitionEngines(mgmtURL)

	// Start periodic heartbeat
	go a.startHeartbeat(mgmtURL)
}

// RefreshConfig fetches the latest CFN configuration from the management plane
// and updates the global CfnConfig.
func (a *App) RefreshConfig(mgmtURL string) error {
	log := getLogger()

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
		log.Infof("CFN Config refreshed, timestamp=%s", CfnTimestamp)
	} else {
		log.Warnf("RefreshConfig response missing config key")
	}

	return nil
}

// startHeartbeat sends periodic heartbeat to mgmt plane to keep CFN status active.
// It runs in a goroutine and sends PUT requests at the configured interval (default 29s).
// The heartbeat stops when the app's stopChan is closed during shutdown.
func (a *App) startHeartbeat(mgmtURL string) {
	log := getLogger()

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
						log.Errorf("failed to parse mgmt timestamp %q: %v", newTimestamp, err)
						continue
					}

					currentTime, err := time.Parse(time.RFC3339Nano, currentTimestamp)
					if err != nil {
						log.Errorf("failed to parse local timestamp %q: %v", currentTimestamp, err)
						continue
					}

					log.Debugf("heartbeat response received: mgmt config_timestamp=%s local config_timestamp=%s", newTimestamp, currentTimestamp)

					// Refresh config if server has newer config
					if newTime.After(currentTime) {
						log.Infof("config update detected: mgmt=%s, local=%s - refreshing", newTimestamp, currentTimestamp)
						if err := a.RefreshConfig(mgmtURL); err != nil {
							log.Errorf("failed to refresh config: %v", err)
						}
					} else {
						log.Debugf("config up-to-date: mgmt=%s, local=%s", newTimestamp, currentTimestamp)
					}
				}

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
	log := getLogger()
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
	log := getLogger()

	log.Infof("shutting down %s...", a.Cfg.ServiceName)
	close(a.stopChan)
	log.Info("- stopping http server")
	err1 := a.server.Stop()
	log.Info("- closing connection to db")
	err2 := a.db.Close()
	return errors.Join(err1, err2)
}

// registerCognitionEngines registers the two Cognition Engines with the management plane.
// It registers both Knowledge Management and Semantic Negotiation Cognition Engines using
// the same CE service URL (host:port) from environment variables.
func (a *App) registerCognitionEngines(mgmtURL string) {
	log := getLogger()

	// Get CE configuration from environment
	ceHost := getEnvOrDefault("COGNITION_ENGINE_HOST", "localhost")
	cePortStr := getEnvOrDefault("COGNITION_ENGINE_PORT", "8000")
	ceURL := fmt.Sprintf("%s:%s", ceHost, cePortStr)

	// Fetch workspace ID from management plane
	workspaceID, err := a.getWorkspaceID(mgmtURL)
	if err != nil {
		log.Fatalf("Failed to get workspace ID: %v", err)
	}

	log.Infof("registering cognition engines with workspace_id=%s, ce_url=%s", workspaceID, ceURL)

	// Fetch existing engines to check if they already exist
	existingEngines, err := a.getExistingCognitionEngines(mgmtURL, workspaceID)
	if err != nil {
		log.Warnf("Failed to fetch existing cognition engines: %v. Will attempt registration anyway.", err)
		existingEngines = make(map[string]bool)
	}

	// Register both cognition engines
	ceNames := []string{
		CognitionEngineKnowledgeManagement,
		CognitionEngineSemanticNegotiation,
	}

	for _, ceName := range ceNames {
		if existingEngines[ceName] {
			log.Infof("Cognition engine %q already exists, skipping registration", ceName)
			continue
		}

		if err := a.registerCognitionEngine(mgmtURL, workspaceID, ceName, ceURL); err != nil {
			log.Fatalf("Failed to register %s: %v", ceName, err)
		}
		log.Infof("Successfully registered %s at %s", ceName, ceURL)
	}
}

// getExistingCognitionEngines fetches the list of existing cognition engines from the management plane
// and returns a map of engine names for quick lookup.
func (a *App) getExistingCognitionEngines(mgmtURL, workspaceID string) (map[string]bool, error) {
	log := getLogger()

	enginesURL := fmt.Sprintf("%s/api/workspaces/%s/cognition-engines", mgmtURL, workspaceID)
	log.Infof("fetching existing cognition engines from %s", enginesURL)

	httpClient := httpclient.New(30 * time.Second)
	ctx := context.Background()
	headers := map[string]string{
		"Accept": "application/json",
	}

	resp, err := httpClient.Get(ctx, enginesURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cognition engines: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch cognition engines: status=%d", resp.StatusCode)
	}

	var result struct {
		Engines []struct {
			CognitiveEngineID   string         `json:"cognitive_engine_id"`
			WorkspaceID         string         `json:"workspace_id"`
			CognitiveEngineName string         `json:"cognitive_engine_name"`
			Config              map[string]any `json:"config"`
			Enabled             bool           `json:"enabled"`
			CreatedAt           string         `json:"created_at"`
		} `json:"engines"`
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode cognition engines response: %w", err)
	}

	// Build map of existing engine names
	existingEngines := make(map[string]bool)
	if result.Engines != nil {
		for _, engine := range result.Engines {
			existingEngines[engine.CognitiveEngineName] = true
		}
	}

	log.Infof("found %d existing cognition engines", len(existingEngines))
	return existingEngines, nil
}

// getWorkspaceID fetches the workspace ID from the management plane.
// If there's only one workspace, it returns that workspace's ID.
// If there are multiple workspaces, it searches for "Default Workspace".
// If "Default Workspace" is not found among multiple workspaces, it returns an error.
func (a *App) getWorkspaceID(mgmtURL string) (string, error) {
	log := getLogger()

	workspacesURL := mgmtURL + "/api/workspaces"
	log.Infof("fetching workspaces from %s", workspacesURL)

	httpClient := httpclient.New(30 * time.Second)
	ctx := context.Background()
	headers := map[string]string{
		"Accept": "application/json",
	}

	resp, err := httpClient.Get(ctx, workspacesURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to fetch workspaces: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch workspaces: status=%d", resp.StatusCode)
	}

	var result struct {
		Workspaces []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			CfnID string `json:"cfn_id"`
		} `json:"workspaces"`
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode workspaces response: %w", err)
	}

	if len(result.Workspaces) == 0 {
		return "", fmt.Errorf("no workspaces found")
	}

	// If there's only one workspace, use that
	if len(result.Workspaces) == 1 {
		workspaceID := result.Workspaces[0].ID
		log.Infof("found single workspace: id=%s, name=%s", workspaceID, result.Workspaces[0].Name)
		return workspaceID, nil
	}

	// Multiple workspaces: search for "Default Workspace"
	for _, ws := range result.Workspaces {
		if ws.Name == DefaultWorkspaceName {
			log.Infof("found Default Workspace: id=%s", ws.ID)
			return ws.ID, nil
		}
	}

	// "Default Workspace" not found among multiple workspaces
	return "", fmt.Errorf("multiple workspaces found but '%s' not found - cognition engine registration failed", DefaultWorkspaceName)
}

// registerCognitionEngine registers a single Cognition Engine with the management plane.
func (a *App) registerCognitionEngine(mgmtURL, workspaceID, engineName, engineURL string) error {
	log := getLogger()

	registerURL := fmt.Sprintf("%s/api/workspaces/%s/cognition-engines", mgmtURL, workspaceID)
	log.Infof("registering cognition engine %q at %s", engineName, registerURL)

	body, _ := json.Marshal(map[string]any{
		"cognitive_engine_name": engineName,
		"config": map[string]any{
			"url": engineURL,
		},
	})

	httpClient := httpclient.New(30 * time.Second)
	ctx := context.Background()
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}

	resp, err := httpClient.Post(ctx, registerURL, body, headers)
	if err != nil {
		return fmt.Errorf("cognition engine registration failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode registration response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cognition engine registration failed: status=%d, response=%v", resp.StatusCode, result)
	}

	return nil
}
