// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

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
//	  "kind": "knowledge",
//	  "subkind": "query",
//	  "url": "http://ce-host:9004",
//	  "version": "1.0.0",
//	  "mas_auto_associate": true,
//	  "capabilities": ["ingestion", "retrieval"],
//	  "metrics": ["kb.documents.indexed", "kb.search.latency_ms"]
//	}
type RegisterRequest struct {
	Name             string                 `json:"name"`
	Kind             string                 `json:"kind"`                       // CE kind (e.g., "knowledge", "contingency")
	Subkind          string                 `json:"subkind"`                    // CE subkind (e.g., "distillation", "query", "negotiation")
	URL              string                 `json:"url"`
	Version          string                 `json:"version"`
	MASAutoAssociate bool                   `json:"mas_auto_associate"` // No omitempty - always send (defaults to false)
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metrics          []string               `json:"metrics,omitempty"`
	Auth             map[string]interface{} `json:"auth,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
	MASConfig        map[string]interface{} `json:"mas_config,omitempty"`
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
//	  "kind": "knowledge",
//	  "subkind": "query",
//	  "enabled": true,
//	  "mas_auto_associate": false,
//	  "status": "online",
//	  "created": true
//	}
type RegisterResponse struct {
	CEID             string `json:"ce_id"`
	CFNID            string `json:"cfn_id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Kind             string `json:"kind"`
	Subkind          string `json:"subkind"`
	Enabled          bool   `json:"enabled"`
	MASAutoAssociate bool   `json:"mas_auto_associate"`
	Status           string `json:"status"`
	Created          bool   `json:"created"`
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
//	  "kind": "knowledge",
//	  "subkind": "query",
//	  "url": "http://ce-host:9004",
//	  "enabled": true,
//	  "mas_auto_associate": false,
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
	ID               string                 `json:"id"`
	CFNID            string                 `json:"cfn_id"`
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Kind             string                 `json:"kind"`
	Subkind          string                 `json:"subkind"`
	URL              string                 `json:"url"`
	Enabled          bool                   `json:"enabled"`
	MASAutoAssociate bool                   `json:"mas_auto_associate"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metrics          []string               `json:"metrics,omitempty"`
	Status           string                 `json:"status"`
	LastSeen         *string                `json:"last_seen,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
	MASConfig        map[string]interface{} `json:"mas_config,omitempty"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        *string                `json:"updated_at,omitempty"`
}

// CognitionEngineListItem represents a CE in the list response.
type CognitionEngineListItem struct {
	ID               string                 `json:"id"`
	CFNID            string                 `json:"cfn_id"`
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Kind             string                 `json:"kind"`
	Subkind          string                 `json:"subkind"`
	URL              string                 `json:"url"`
	Enabled          bool                   `json:"enabled"`
	MASAutoAssociate bool                   `json:"mas_auto_associate"`
	Status           string                 `json:"status"`
	LastSeen         *string                `json:"last_seen,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
	MASConfig        map[string]interface{} `json:"mas_config,omitempty"`
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
// Mutable fields: url, enabled, mas_auto_associate, capabilities, metrics, config, mas_config, auth, kind, subkind.
// Immutable fields: cfn_id, version, name. Attempting to update immutable fields will result in a 400 error.
//
// Example JSON:
//
//	{
//	  "enabled": false,
//	  "url": "http://new-host:9004",
//	  "mas_auto_associate": true,
//	  "capabilities": ["ingestion", "retrieval", "search"]
//	}
type PatchRequest struct {
	// Mutable fields
	URL              *string                `json:"url,omitempty"`
	Enabled          *bool                  `json:"enabled,omitempty"`
	MASAutoAssociate *bool                  `json:"mas_auto_associate,omitempty"`
	Kind             *string                `json:"kind,omitempty"`
	Subkind          *string                `json:"subkind,omitempty"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metrics          []string               `json:"metrics,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
	MASConfig        map[string]interface{} `json:"mas_config,omitempty"`
	Auth             map[string]interface{} `json:"auth,omitempty"`

	// Immutable fields - included to trigger validation error if provided
	CFNID   *string `json:"cfn_id,omitempty"`
	Version *string `json:"version,omitempty"`
	Name    *string `json:"name,omitempty"`
}

// MASCEConfigResponse returns the per-MAS config override for a specific CE.
// This is the resolved mas_config from the mas_cognition_engines join table,
// not the CE-wide factory defaults from the cognition_engines table.
//
// Example JSON:
//
//	{
//	  "ce_id": "7c8650d9-...",
//	  "mas_id": "550e8400-...",
//	  "workspace_id": "660e8400-...",
//	  "mas_config": {
//	    "retry_max_attempts": 3,
//	    "validation_score_intervention": 0.6
//	  }
//	}
type MASCEConfigResponse struct {
	CEID        string                 `json:"ce_id"`
	MASID       string                 `json:"mas_id"`
	WorkspaceID string                 `json:"workspace_id"`
	MASConfig   map[string]interface{} `json:"mas_config"`
}
