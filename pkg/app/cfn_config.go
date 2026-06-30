// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import "strings"

// CfnConfigPayload is the typed representation of the config blob received from the management plane.
// Only fields that cfn-svc actually reads are declared; json.Unmarshal ignores the rest.
type CfnConfigPayload struct {
	ConfigVersion    int64             `json:"config_version"`
	CfnConfig        map[string]any    `json:"cfn_config,omitempty"`
	Workspaces       []WorkspaceConfig `json:"workspaces"`
	MemoryProviders  []MemProviderCfg  `json:"memory_providers"`
	CognitionEngines []EngineCfg       `json:"cognition_engines"` // Top-level CE list
}

type WorkspaceConfig struct {
	ID                  string   `json:"workspace_id"`
	WorkspaceName       string   `json:"workspace_name,omitempty"`
	MultiAgenticSystems []MASCfg `json:"multi_agentic_systems"`
}

type MASCfg struct {
	ID               string         `json:"id"`
	WorkspaceID      string         `json:"workspace_id,omitempty"`
	Name             string         `json:"name,omitempty"`
	Description      string         `json:"description,omitempty"`
	SharedMemory     *MemoryCfg     `json:"shared_memory"`
	Agents           []AgentCfg     `json:"agents"`
	Config           map[string]any `json:"config,omitempty"`
	CognitionEngines []MASEngineCfg `json:"cognition_engines"` // CEs associated with this MAS
}

// MASEngineCfg represents a CE's association with a MAS, including per-MAS config overrides.
// The MASConfig map can contain a "schedule" field (cron expression) to enable scheduled tasks
// for this CE within this MAS. If no schedule is present, no task is created.
type MASEngineCfg struct {
	ID        string         `json:"id"`
	Name      string         `json:"name,omitempty"`
	MASConfig map[string]any `json:"mas_config,omitempty"` // Per-MAS config overrides (can include "schedule")
}

type AgentIdentityCfg struct {
	Type        string            `json:"type"`
	Identifiers map[string]string `json:"identifiers"`
}

type AgentCfg struct {
	AgentID       string            `json:"agent_id"`
	Name          string            `json:"name,omitempty"`
	URL           string            `json:"url,omitempty"`
	Identity      *AgentIdentityCfg `json:"identity,omitempty"`
	AgenticMemory *MemoryCfg        `json:"agentic_memory"`
}

type MemoryCfg struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Config  *MemConnConfig `json:"config"`
}

type MemConnConfig struct {
	URL  string      `json:"url"`
	Auth *AuthConfig `json:"auth"`
}

type AuthConfig struct {
	Type        string     `json:"type"`
	Credentials *AuthCreds `json:"credentials"`
}

type AuthCreds struct {
	APIKey      string `json:"api_key"`
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`
}

type MemProviderCfg struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled"`
	Config      *MemConnConfig `json:"config"`
}

type EngineCfg struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	URL              string                 `json:"url"`
	Kind             string                 `json:"kind"`    // New: CE kind (e.g., "knowledge", "contingency")
	Subkind          string                 `json:"subkind"` // New: CE subkind (e.g., "distillation", "query", "negotiation")
	Enabled          bool                   `json:"enabled"`
	Status           string                 `json:"status,omitempty"`
	LastSeen         string                 `json:"last_seen,omitempty"` // New: ISO 8601 timestamp
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metrics          []string               `json:"metrics,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
	MASConfig        map[string]interface{} `json:"mas_config,omitempty"`         // Default MAS config template
	MASAutoAssociate bool                   `json:"mas_auto_associate,omitempty"` // New: auto-associate with new MAS
	Auth             *AuthConfig            `json:"auth,omitempty"`               // Decrypted auth from mgmt plane
}

// FindCE locates a Cognition Engine by CE ID in the top-level cognition_engines list.
func (c *CfnConfigPayload) FindCE(ceID string) *EngineCfg {
	for i := range c.CognitionEngines {
		if c.CognitionEngines[i].ID == ceID {
			return &c.CognitionEngines[i]
		}
	}
	return nil
}

// IsCEAssociatedWithMAS checks if a CE is associated with any MAS.
// Returns true if at least one MAS has this CE in its cognition_engines list.
func (c *CfnConfigPayload) IsCEAssociatedWithMAS(ceID string) bool {
	for _, ws := range c.Workspaces {
		for _, mas := range ws.MultiAgenticSystems {
			for _, ce := range mas.CognitionEngines {
				if ce.ID == ceID {
					return true
				}
			}
		}
	}
	return false
}

// FindMASConfigForCE returns the MAS-specific config for a CE within a given MAS.
// Returns nil if the CE is not associated with the MAS or if no MAS config exists.
func (c *CfnConfigPayload) FindMASConfigForCE(workspaceID, masID, ceID string) map[string]any {
	mas := c.FindMAS(workspaceID, masID)
	if mas == nil {
		return nil
	}
	for _, ce := range mas.CognitionEngines {
		if ce.ID == ceID {
			return ce.MASConfig
		}
	}
	return nil
}

// FindMAS locates a MAS by workspace and MAS ID.
func (c *CfnConfigPayload) FindMAS(workspaceID, masID string) *MASCfg {
	for i := range c.Workspaces {
		if c.Workspaces[i].ID != workspaceID {
			continue
		}
		for j := range c.Workspaces[i].MultiAgenticSystems {
			if c.Workspaces[i].MultiAgenticSystems[j].ID == masID {
				return &c.Workspaces[i].MultiAgenticSystems[j]
			}
		}
	}
	return nil
}

// FindAgent locates an agent by workspace, MAS, and agent ID.
func (c *CfnConfigPayload) FindAgent(workspaceID, masID, agentID string) *AgentCfg {
	mas := c.FindMAS(workspaceID, masID)
	if mas == nil {
		return nil
	}
	for i := range mas.Agents {
		if mas.Agents[i].AgentID == agentID {
			return &mas.Agents[i]
		}
	}
	return nil
}

// FindWorkspace locates a workspace by ID.
func (c *CfnConfigPayload) FindWorkspace(workspaceID string) *WorkspaceConfig {
	for i := range c.Workspaces {
		if c.Workspaces[i].ID == workspaceID {
			return &c.Workspaces[i]
		}
	}
	return nil
}

// FindCEsByKind returns all CEs in a MAS that match the given kind and optional subkind.
// If subkind is empty, returns all CEs matching the kind.
func (c *CfnConfigPayload) FindCEsByKind(workspaceID, masID, kind, subkind string) []*EngineCfg {
	var matches []*EngineCfg

	mas := c.FindMAS(workspaceID, masID)
	if mas == nil {
		return matches
	}

	for _, masEngine := range mas.CognitionEngines {
		ce := c.FindCE(masEngine.ID)
		if ce == nil || !ce.Enabled {
			continue
		}

		// Match by kind (required)
		if ce.Kind != kind {
			continue
		}

		// Match by subkind if specified
		if subkind != "" && ce.Subkind != "" && ce.Subkind != subkind {
			continue
		}

		matches = append(matches, ce)
	}

	return matches
}

// FindAgentByURL returns the workspace, MAS, and agent IDs for the first agent whose
// any Identity.Identifiers value is a prefix of the given session key.
// Returns empty strings if not found.
func (c *CfnConfigPayload) FindAgentByURL(sessionKey string) (workspaceID, masID, agentID string) {
	for _, ws := range c.Workspaces {
		for _, mas := range ws.MultiAgenticSystems {
			for _, agent := range mas.Agents {
				if agent.Identity == nil {
					continue
				}
				for _, val := range agent.Identity.Identifiers {
					if val != "" && strings.HasPrefix(sessionKey, val) {
						return ws.ID, mas.ID, agent.AgentID
					}
				}
			}
		}
	}
	return "", "", ""
}

// hasRequiredIDs returns true if both workspace and MAS IDs are non-empty.
func hasRequiredIDs(workspaceID, masID string) bool {
	return workspaceID != "" && masID != ""
}

// masExistsInConfig returns true if the workspace/MAS pair is registered in the current CFN config.
func masExistsInConfig(workspaceID, masID string) bool {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()
	if ParsedConfig == nil {
		return false
	}
	return ParsedConfig.FindMAS(workspaceID, masID) != nil
}

// getSharedMemoryID returns the shared memory ID for a given workspace/MAS from ParsedConfig.
func getSharedMemoryID(workspaceID, masID string) string {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	if ParsedConfig == nil {
		return ""
	}
	mas := ParsedConfig.FindMAS(workspaceID, masID)
	if mas != nil && mas.SharedMemory != nil {
		return mas.SharedMemory.ID
	}
	return ""
}

// getAgentMemoryID returns the agentic memory ID for a given workspace/MAS/agent from ParsedConfig.
func getAgentMemoryID(workspaceID, masID, agentID string) string {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	if ParsedConfig == nil {
		return ""
	}
	agent := ParsedConfig.FindAgent(workspaceID, masID, agentID)
	if agent != nil && agent.AgenticMemory != nil {
		return agent.AgenticMemory.ID
	}
	return ""
}
