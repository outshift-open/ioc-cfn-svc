package app

// audit_resource_ids.go — Hacky call to the management plane summary API
// to grab shared_memory.id and agentic_memory.id for audit AuditResourceIdentifier fields.
// This bypasses CfnConfig parsing and directly hits:
//   GET /api/cognition-fabric-nodes/{CfnID}/summary
// The IDs are fetched once on the first audit operation and stored globally.
// TODO: Remove this once the IDs are available in CfnConfig global map.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
)

var (
	// SharedMemoryID is the shared_memory.id extracted from the CFN summary API.
	// Used as AuditResourceIdentifier for SHARED_MEMORY_OPERATION audit events.
	SharedMemoryID string

	// AgentMemoryID is the agent's agentic_memory.id extracted from the CFN summary API.
	// Used as AuditResourceIdentifier for AGENT_MEMORY_OPERATION audit events.
	AgentMemoryID string

	auditIDsOnce sync.Once
)

// ensureAuditResourceIDs fetches SharedMemoryID and AgentMemoryID exactly once
// from the management plane summary API. Safe to call from any handler;
// the first call does the HTTP fetch, subsequent calls are no-ops.
func ensureAuditResourceIDs() {
	auditIDsOnce.Do(func() {
		log := getLogger()

		mgmtURL := os.Getenv("MGMT_URL")
		if mgmtURL == "" {
			mgmtURL = "http://localhost:9000"
		}

		if CfnID == "" {
			log.Warnf("ensureAuditResourceIDs: CfnID is empty, skipping")
			return
		}

		summaryURL := mgmtURL + "/api/cognition-fabric-nodes/" + CfnID + "/summary"
		log.Infof("ensureAuditResourceIDs: fetching summary from %s", summaryURL)

		client := httpclient.New(30 * time.Second)
		resp, err := client.Get(context.Background(), summaryURL, map[string]string{
			"Accept": "application/json",
		})
		if err != nil {
			log.Errorf("ensureAuditResourceIDs: failed to call summary API: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Errorf("ensureAuditResourceIDs: summary API returned status %d", resp.StatusCode)
			return
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Errorf("ensureAuditResourceIDs: failed to decode summary response: %v", err)
			return
		}

		sharedMemID, agentMemID, err := extractMemoryIDs(result)
		if err != nil {
			log.Errorf("ensureAuditResourceIDs: %v", err)
			return
		}

		SharedMemoryID = sharedMemID
		AgentMemoryID = agentMemID
		log.Infof("ensureAuditResourceIDs: SharedMemoryID=%s AgentMemoryID=%s", sharedMemID, agentMemID)
	})
}

// extractMemoryIDs navigates the summary JSON to pull out shared_memory.id and
// the first agent's agentic_memory.id.
// Path: config.workspaces[0].multi_agentic_systems[0].shared_memory.id
// Path: config.workspaces[0].multi_agentic_systems[0].agents[0].agentic_memory.id
func extractMemoryIDs(summary map[string]interface{}) (string, string, error) {
	configMap, ok := summary["config"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("config not found in summary response")
	}

	workspaces, ok := configMap["workspaces"].([]interface{})
	if !ok || len(workspaces) == 0 {
		return "", "", fmt.Errorf("workspaces not found or empty in summary")
	}

	ws, ok := workspaces[0].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("first workspace is not a valid object")
	}

	masList, ok := ws["multi_agentic_systems"].([]interface{})
	if !ok || len(masList) == 0 {
		return "", "", fmt.Errorf("multi_agentic_systems not found or empty")
	}

	mas, ok := masList[0].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("first MAS is not a valid object")
	}

	// Extract shared_memory.id
	var sharedMemID string
	if sm, ok := mas["shared_memory"].(map[string]interface{}); ok {
		if id, ok := sm["id"].(string); ok {
			sharedMemID = id
		}
	}
	if sharedMemID == "" {
		return "", "", fmt.Errorf("shared_memory.id not found in first MAS")
	}

	// Extract first agent's agentic_memory.id
	var agentMemID string
	if agents, ok := mas["agents"].([]interface{}); ok && len(agents) > 0 {
		if agent, ok := agents[0].(map[string]interface{}); ok {
			if am, ok := agent["agentic_memory"].(map[string]interface{}); ok {
				if id, ok := am["id"].(string); ok {
					agentMemID = id
				}
			}
		}
	}
	if agentMemID == "" {
		return sharedMemID, "", fmt.Errorf("agentic_memory.id not found in first agent (shared_memory.id=%s)", sharedMemID)
	}

	return sharedMemID, agentMemID, nil
}
