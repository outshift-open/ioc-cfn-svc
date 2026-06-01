// Package cognitionengine provides DTOs for the cognition engine registration API.
package cognitionengine

// RegisterRequest represents a Cognition Engine registration request from CE to CFN.
// The CE sends this payload to CFN's /api/cognition-engines endpoint,
// and CFN forwards it (with added CFN context) to the management plane.
//
// Example JSON:
//
//	{
//	  "name": "Knowledge Management CE",
//	  "type": "knowledge_management",
//	  "url": "http://ce-host:9004",
//	  "version": "1.0.0",
//	  "auto_attach": true,
//	  "capabilities": ["ingestion", "retrieval"],
//	  "metrics": ["kb.documents.indexed", "kb.search.latency_ms"]
//	}
type RegisterRequest struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	URL          string                 `json:"url"`
	Version      string                 `json:"version"`
	AutoAttach   bool                   `json:"auto_attach"` // No omitempty - always send (defaults to false)
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
//	  "enabled": true,
//	  "auto_attach": false,
//	  "status": "online",
//	  "created": true
//	}
type RegisterResponse struct {
	CEID       string `json:"ce_id"`
	CFNID      string `json:"cfn_id"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	AutoAttach bool   `json:"auto_attach"`
	Status     string `json:"status"`
	Created    bool   `json:"created"`
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

// CognitionEngineDetail represents detailed information about a CE.
//
// Example JSON:
//
//	{
//	  "id": "ce-123",
//	  "cfn_id": "cfn-456",
//	  "name": "Knowledge Management CE",
//	  "version": "1.0.0",
//	  "type": "knowledge_management",
//	  "url": "http://ce-host:9004",
//	  "enabled": true,
//	  "auto_attach": false,
//	  "capabilities": ["ingestion", "retrieval"],
//	  "metrics": ["kb.documents.indexed"],
//	  "status": "online",
//	  "last_seen": "2026-05-21T10:30:00Z",
//	  "config": {},
//	  "mas_config": {},
//	  "created_at": "2026-05-21T09:00:00Z",
//	  "updated_at": "2026-05-21T10:30:00Z"
//	}
type CognitionEngineDetail struct {
	ID           string                 `json:"id"`
	CFNID        string                 `json:"cfn_id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Type         string                 `json:"type"`
	URL          string                 `json:"url"`
	Enabled      bool                   `json:"enabled"`
	AutoAttach   bool                   `json:"auto_attach"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Metrics      []string               `json:"metrics,omitempty"`
	Status       string                 `json:"status"`
	LastSeen     *string                `json:"last_seen,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	MASConfig    map[string]interface{} `json:"mas_config,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    *string                `json:"updated_at,omitempty"`
}

// CognitionEngineListItem represents a CE in the list response.
type CognitionEngineListItem struct {
	ID         string                 `json:"id"`
	CFNID      string                 `json:"cfn_id"`
	Name       string                 `json:"name"`
	Version    string                 `json:"version"`
	Type       string                 `json:"type"`
	URL        string                 `json:"url"`
	Enabled    bool                   `json:"enabled"`
	AutoAttach bool                   `json:"auto_attach"`
	Status     string                 `json:"status"`
	LastSeen   *string                `json:"last_seen,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	MASConfig  map[string]interface{} `json:"mas_config,omitempty"`
}

// CognitionEngineList represents a list of CEs.
//
// Example JSON:
//
//	{
//	  "cognition_engines": [...],
//	  "total": 5
//	}
type CognitionEngineList struct {
	CognitionEngines []CognitionEngineListItem `json:"cognition_engines"`
	Total            int                       `json:"total"`
}

// PatchRequest represents a partial update request for a CE.
// Only mutable fields (enabled, capabilities, metrics, config, mas_config, auth) can be updated.
// Attempting to update immutable fields will result in a 400 error from the management plane.
//
// Example JSON:
//
//	{
//	  "enabled": false,
//	  "capabilities": ["ingestion", "retrieval", "search"]
//	}
type PatchRequest struct {
	// Mutable fields
	Enabled      *bool                  `json:"enabled,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Metrics      []string               `json:"metrics,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	MASConfig    map[string]interface{} `json:"mas_config,omitempty"`
	Auth         map[string]interface{} `json:"auth,omitempty"`

	// Immutable fields - included to trigger validation error if provided
	URL        *string `json:"url,omitempty"`
	CFNID      *string `json:"cfn_id,omitempty"`
	Version    *string `json:"version,omitempty"`
	Name       *string `json:"name,omitempty"`
	Type       *string `json:"type,omitempty"`
	AutoAttach *bool   `json:"auto_attach,omitempty"`
}

// MASCEAssociateRequest represents a request to associate a CE with a MAS.
//
// Example JSON:
//
//	{
//	  "ce_id": "550e8400-e29b-41d4-a716-446655440000"
//	}
type MASCEAssociateRequest struct {
	CEID string `json:"ce_id"`
}

// MASCEAssociateResponse represents the response after associating a CE with a MAS.
//
// Example JSON:
//
//	{
//	  "ce_id": "550e8400-e29b-41d4-a716-446655440000",
//	  "mas_id": "e9b5592f-326d-42e3-8bbe-29cf876ebc7c",
//	  "created_at": "2026-06-01T15:30:00Z"
//	}
type MASCEAssociateResponse struct {
	CEID      string `json:"ce_id"`
	MASID     string `json:"mas_id"`
	CreatedAt string `json:"created_at"`
}
