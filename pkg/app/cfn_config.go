package app

// CfnConfigPayload is the typed representation of the config blob received from the management plane.
// Only fields that cfn-svc actually reads are declared; json.Unmarshal ignores the rest.
type CfnConfigPayload struct {
	ConfigVersion   int64              `json:"config_version"`
	Workspaces      []WorkspaceConfig  `json:"workspaces"`
	MemoryProviders []MemProviderCfg   `json:"memory_providers"`
}

type WorkspaceConfig struct {
	ID                  string      `json:"workspace_id"`
	MultiAgenticSystems []MASCfg    `json:"multi_agentic_systems"`
	CognitionEngines    []EngineCfg `json:"cognition_engines"`
}

type MASCfg struct {
	ID           string     `json:"id"`
	SharedMemory *MemoryCfg `json:"shared_memory"`
	Agents       []AgentCfg `json:"agents"`
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
	Name   string         `json:"name"`
	Config *MemConnConfig `json:"config"`
}

type EngineCfg struct {
	Name   string         `json:"name"`
	Config *MemConnConfig `json:"config"`
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

// FindAgentByURL returns the workspace, MAS, and agent IDs for the agent whose
// Identity.Identifiers["url"] matches the given session key (e.g. "main::agents::planner").
// Returns empty strings if not found.
func (c *CfnConfigPayload) FindAgentByURL(sessionKey string) (workspaceID, masID, agentID string) {
	for _, ws := range c.Workspaces {
		for _, mas := range ws.MultiAgenticSystems {
			for _, agent := range mas.Agents {
				if agent.Identity == nil {
					continue
				}
				if url, ok := agent.Identity.Identifiers["url"]; ok && url == sessionKey {
					return ws.ID, mas.ID, agent.AgentID
				}
			}
		}
	}
	return "", "", ""
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
