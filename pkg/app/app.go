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

	"github.com/cisco-eti/ioc-cfn-svc/pkg/app/httpapi/sharedmemory"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/audit"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/config"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/otelreceiver"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
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
	// ParsedConfig is the typed config parsed from the management plane config blob.
	ParsedConfig *CfnConfigPayload
	// ConfigVersion tracks the config version for detecting config changes during heartbeat.
	ConfigVersion int64
	// cfnConfigMutex protects concurrent access to ParsedConfig and ConfigVersion.
	cfnConfigMutex sync.RWMutex
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
	startTime     time.Time
	Cfg           config.Config
	server        *easyhttp.EasyServer

	readyForRequests *atomic.Bool
	stopChan         chan struct{}

	// integrated clients
	db                client.Database
	s3                client.S3
	memoryProxyClient *httpclient.Client

	knowledgeMemSvcClient *iocmemoryprovider.Client
	cognitionAgentsClient *cognitionagentclient.Client

	otelReceiver *otelreceiver.OTLPReceiver
}

func New(buildVersion, gitCommitSHA, gitCommitTime, gitBranch string) (*App, error) {
	cfg := config.Get()
	log := getLogger()

	// Apply pagination config from env/flags (falls back to built-in defaults).
	audit.SetPaginationConfig(cfg.Pagination.DefaultPageSize, cfg.Pagination.MaxPageSize)
	log.Infof("pagination config: defaultPageSize=%d, maxPageSize=%d", audit.DefaultPageSize(), audit.MaxPageSize())

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

	cognitionAgentsURL := getEnvOrDefault("COGNITION_ENGINE_SVC_URL", "http://localhost:9004")
	log.Infof("cognition agents service URL: %s", cognitionAgentsURL)
	cognitionAgentsClient := cognitionagentclient.New(cognitionAgentsURL, 120*time.Second)

	// Build OTel receiver — batch and flush spans to cfn_cp otel_spans table.
	otelBatchSize := 100
	otelFlushInterval := 5 * time.Second

	resolver := func(sessionKey string) string {
		cfnConfigMutex.RLock()
		defer cfnConfigMutex.RUnlock()
		if ParsedConfig == nil {
			return ""
		}
		_, _, agentID := ParsedConfig.FindAgentByURL(sessionKey)
		return agentID
	}

	exp := otelreceiver.NewSpanExporter(db, resolver)

	batchFactory := batchprocessor.NewFactory()
	batchCfg := batchFactory.CreateDefaultConfig().(*batchprocessor.Config)
	batchCfg.SendBatchSize = uint32(otelBatchSize)
	batchCfg.Timeout = otelFlushInterval

	telSet := component.TelemetrySettings{
		Logger:         log.Desugar(),
		TracerProvider: tracenoop.NewTracerProvider(),
		MeterProvider:  metricnoop.NewMeterProvider(),
	}
	batchProc, err := batchFactory.CreateTraces(
		context.Background(),
		processor.Settings{
			ID:                component.NewID(component.MustNewType("batch")),
			TelemetrySettings: telSet,
		},
		batchCfg,
		exp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch processor: %w", err)
	}

	otelRcvr := otelreceiver.New(batchProc)
	log.Infof("OTLP receiver configured on /v1/traces, batch_size=%d, flush_interval=%s",
		otelBatchSize, otelFlushInterval)

	a := &App{
		buildVersion:          buildVersion,
		gitCommitSHA:          gitCommitSHA,
		gitCommitTime:         gitCommitTime,
		gitBranch:             gitBranch,
		startTime:             time.Now(),
		Cfg:                   *cfg,
		readyForRequests:      &atomic.Bool{},
		stopChan:              make(chan struct{}),
		db:                    db,
		s3:                    s3,
		memoryProxyClient:     memoryProxyClient,
		knowledgeMemSvcClient: knowledgeMemClient,
		cognitionAgentsClient: cognitionAgentsClient,
		otelReceiver:          otelRcvr,
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

	registerURL := mgmtURL + "/api/cognition-fabric-nodes/register"
	log.Infof("registering CFN at %s", registerURL)

	body, _ := json.Marshal(map[string]any{
		"name":       cfnName,
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

	// Store id from response globally
	id, ok := result["id"].(string)
	if !ok || id == "" {
		log.Fatalf("registration response missing id")
	}
	CfnID = id

	// Parse config blob into typed struct
	if cfgBlob, ok := result["config"].(map[string]any); ok {
		cfgBytes, _ := json.Marshal(cfgBlob)
		var parsed CfnConfigPayload
		if err := json.Unmarshal(cfgBytes, &parsed); err != nil {
			log.Errorf("failed to parse config into typed struct: %v", err)
		} else {
			cfnConfigMutex.Lock()
			ParsedConfig = &parsed
			ConfigVersion = parsed.ConfigVersion
			cfnConfigMutex.Unlock()
		}
	}

	cfgJSON, _ := json.Marshal(ParsedConfig)
	log.Infof("CFN registered successfully: cfn_id=%s cfn_name=%s ip_address=%s port=%d config_version=%d payload=%s", CfnID, cfnName, appIP, appPort, ConfigVersion, cfgJSON)

	// Start periodic heartbeat
	go a.startHeartbeat(mgmtURL)
}

// RefreshConfig fetches the latest CFN configuration from the management plane
// and updates the global ParsedConfig.
func (a *App) RefreshConfig(mgmtURL string) error {
	log := getLogger()

	cfnURL := mgmtURL + "/api/cognition-fabric-nodes/" + CfnID

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

	if cfgBlob, ok := result["config"].(map[string]any); ok {
		cfgBytes, _ := json.Marshal(cfgBlob)
		var parsed CfnConfigPayload
		if err := json.Unmarshal(cfgBytes, &parsed); err != nil {
			log.Errorf("failed to parse refreshed config into typed struct: %v", err)
			return fmt.Errorf("failed to parse config: %v", err)
		}
		cfnConfigMutex.Lock()
		ParsedConfig = &parsed
		ConfigVersion = parsed.ConfigVersion
		cfnConfigMutex.Unlock()
		log.Infof("CFN Config refreshed, config_version=%d", parsed.ConfigVersion)
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
	heartbeatURL := mgmtURL + "/api/cognition-fabric-nodes/" + CfnID + "/heartbeat"

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

				// Check if config_version has changed
				if newVersion, ok := result["config_version"].(float64); ok {
					cfnConfigMutex.RLock()
					current := ConfigVersion
					cfnConfigMutex.RUnlock()

					remoteVersion := int64(newVersion)
					log.Debugf("heartbeat response received: remote_version=%d local_version=%d", remoteVersion, current)

					if remoteVersion > current {
						log.Infof("config update detected: remote_version=%d local_version=%d - refreshing", remoteVersion, current)
						if err := a.RefreshConfig(mgmtURL); err != nil {
							log.Errorf("failed to refresh config: %v", err)
						}
					} else {
						log.Debugf("config up-to-date: version=%d", current)
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

	a.otelReceiver.Start()

	wg := sync.WaitGroup{}
	wg.Add(1)
	var serverErr error
	go func() {
		defer wg.Done()
		log.Infof("starting the web server on port %d", a.Cfg.AppPort)
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
	log.Info("- stopping OTLP receiver")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err0 := a.otelReceiver.Stop(ctx)
	log.Info("- stopping http server")
	err1 := a.server.Stop()
	log.Info("- closing connection to db")
	err2 := a.db.Close()
	return errors.Join(err0, err1, err2)
}

// CreateOrUpdateSharedMemoriesCore implements the McpService interface.
// This method provides access to the core business logic for creating or updating shared memories.
func (a *App) CreateOrUpdateSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.CreateOrUpdateRequest) (*sharedmemory.CreateOrUpdateResponse, error) {
	return a.createOrUpdateSharedMemoriesCore(ctx, workspaceID, masID, req)
}

// FetchSharedMemoriesCore implements the McpService interface.
// This method provides access to the core business logic for fetching shared memories.
func (a *App) FetchSharedMemoriesCore(ctx context.Context, workspaceID, masID string, req sharedmemory.QueryRequest) (*sharedmemory.QueryResponse, error) {
	return a.fetchSharedMemoriesCore(ctx, workspaceID, masID, req)
}
