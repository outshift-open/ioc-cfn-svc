// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

// Package task provides the task scheduling infrastructure for cfn-svc,
// including CE name to endpoint mapping and cron expression parsing.
package task

import "strings"

const (
	EndpointDistillation = "/api/knowledge-mgmt/distillation"
	EndpointExtraction   = "/api/knowledge-mgmt/extraction"
)

// GetEndpointForCE returns the endpoint path for a given CE name.
// Returns empty string if no endpoint mapping exists.
//
// TODO: This is a temporary hack using pattern matching on CE names.
// Replace with proper CE type/capability-based routing once CE metadata is standardized.
// Current patterns:
//   - Name contains "distillation" → /api/knowledge-mgmt/distillation (Implemented)
//   - Name contains "extraction" or "knowledge" → /api/knowledge-mgmt/extraction (Implemented)
func GetEndpointForCE(ceName string) string {
	nameLower := strings.ToLower(ceName)

	// Implemented: Handles cognition distillation tasks
	if strings.Contains(nameLower, "distillation") {
		return EndpointDistillation
	}

	// Implemented: Handles OTEL span extraction and knowledge ingestion
	if strings.Contains(nameLower, "extraction") || strings.Contains(nameLower, "knowledge") {
		return EndpointExtraction
	}

	// No matching pattern found
	return ""
}
