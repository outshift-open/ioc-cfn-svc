package app

type Header struct {
	WorkspaceID string `json:"workspace_id"`
	MASID       string `json:"mas_id"`
	AgentID     string `json:"agent_id,omitempty"`
}
