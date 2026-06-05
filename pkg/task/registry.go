// Package task provides the task scheduling infrastructure for cfn-svc,
// including CE name to endpoint mapping and cron expression parsing.
package task

import "strings"

// GetEndpointForCE returns the endpoint path for a given CE name.
// Returns empty string if no endpoint mapping exists.
//
// TODO: This is a temporary hack using pattern matching on CE names.
// Replace with proper CE type/capability-based routing once CE metadata is standardized.
// Current patterns:
//   - Name contains "distillation" → /api/knowledge-mgmt/runDistillation (⏳ Not implemented yet)
//   - Name contains "extraction" or "knowledge" → /api/knowledge-mgmt/extraction (✅ Implemented)
func GetEndpointForCE(ceName string) string {
	nameLower := strings.ToLower(ceName)

	// TODO: CE must implement /api/knowledge-mgmt/runDistillation endpoint
	// This endpoint should accept a TaskExecutionRequest and return 202 Accepted with a TaskExecutionResponse.
	if strings.Contains(nameLower, "distillation") {
		return "/api/knowledge-mgmt/runDistillation"
	}

	if strings.Contains(nameLower, "negotiation") {
		return "/api/knowledge-mgmt/runNegotiation"
	}

	// ✅ Implemented: Handles OTEL span extraction and knowledge ingestion
	if strings.Contains(nameLower, "extraction") || strings.Contains(nameLower, "knowledge") {
		return "/api/knowledge-mgmt/extraction"
	}

	// No matching pattern found
	return ""
}
