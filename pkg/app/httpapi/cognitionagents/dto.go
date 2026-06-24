// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

// Package cognitionagents provides DTOs for the cognition agents API handler.
//
// NOTE: Struct fields and JSON tags may change as the API evolves.
package cognitionagents

import (
	"encoding/json"

	"github.com/outshift-open/ioc-cfn-svc/pkg/common"
)

//////////////////////////////////////////////////////////////////

// RequestType represents the type of memory operation
type RequestType string

// RequestType constants for memory operations
const (
	ReqTypeKnowledgeVectorsUpsert RequestType = "KNOWLEDGE_VECTORS_UPSERT"
	ReqTypeKnowledgeVectorsQuery  RequestType = "KNOWLEDGE_VECTORS_QUERY"
)

// SharedMemoryVectorsRequest represents a request for shared memory vector operations.
// The Body field contains the raw JSON payload as defined in:
// https://github.com/outshift-open/ioc-cfn-svc/blob/main/pkg/providers/memory/ioc/schema.go
//
// Example JSON structure:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "request_type": "KNOWLEDGE_VECTORS_UPSERT" or "KNOWLEDGE_VECTORS_QUERY",
//	  "request_body": {
//	    // Raw JSON payload matching KnowledgeVectorStoreRequest or KnowledgeVectorQueryRequest
//	    // from iocmemoryprovider schema
//	  }
//	}
type SharedMemoryVectorsRequest struct {
	Header common.Header    `json:"header"`
	Type   RequestType      `json:"request_type"`
	Body   *json.RawMessage `json:"request_body"`
}

// SharedMemoryVectorsResponse represents a response from shared memory vector operations.
// The Results field contains the raw JSON response returned from iocmemoryprovider.
//
// Example JSON structure for success:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "results": {
//	    // Raw JSON response matching KnowledgeVectorStoreResponse or KnowledgeVectorQueryResponse
//	    // from iocmemoryprovider schema
//	  }
//	}
//
// Example JSON structure for error:
//
//	{
//	  "header": {
//	    "workspace_id": "workspace-123",
//	    "mas_id": "mas-456"
//	  },
//	  "error": {
//	    "message": "Error description",
//	    "detail": { "additional": "error details" }
//	  }
//	}
type SharedMemoryVectorsResponse struct {
	Header  common.Header       `json:"header"`
	Error   *common.ErrorDetail `json:"error,omitempty"`
	Results *json.RawMessage    `json:"results,omitempty"`
}
