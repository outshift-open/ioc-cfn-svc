// Package cognitionengine provides DTOs for the cognition engine registration API.
package cognitionengine

// RegisterRequest represents a Cognition Engine registration request from CE to CFN.
// The CE sends this payload to CFN's /api/cognition-engines/register endpoint,
// and CFN forwards it (with added CFN context) to the management plane.
//
// Example JSON:
//
//	{
//	  "name": "Knowledge Management CE",
//	  "type": "knowledge_management",
//	  "url": "http://ce-host:9004",
//	  "version": "1.0.0",
//	  "capabilities": ["ingestion", "retrieval"],
//	  "metrics": ["kb.documents.indexed", "kb.search.latency_ms"]
//	}
type RegisterRequest struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	URL          string                 `json:"url"`
	Version      string                 `json:"version"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Metrics      []string               `json:"metrics,omitempty"`
	Auth         map[string]interface{} `json:"auth,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	MASConfig    map[string]interface{} `json:"mas_config,omitempty"`
}

// RegisterResponse represents the response returned to the CE after successful registration.
// This is the same response format that the management plane returns to CFN.
//
// Example JSON:
//
//	{
//	  "ce_id": "ce-123",
//	  "cfn_id": "cfn-456",
//	  "name": "Knowledge Management CE",
//	  "version": "1.0.0",
//	  "type": "knowledge_management",
//	  "status": "online",
//	  "created": true
//	}
type RegisterResponse struct {
	CEID    string `json:"ce_id"`
	CFNID   string `json:"cfn_id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Created bool   `json:"created"`
}

// HeartbeatResponse represents the response returned to the CE after a successful heartbeat.
// The management plane uses this to indicate the CE's current status and last seen timestamp.
//
// Example JSON:
//
//	{
//	  "status": "online",
//	  "last_seen": "2026-05-21T10:30:00Z"
//	}
type HeartbeatResponse struct {
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
}
