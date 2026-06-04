// Package task provides the task scheduling infrastructure for cfn-svc,
// including CE name to endpoint mapping and cron expression parsing.
package task

// ceNameToEndpoint maps CE names to their corresponding endpoint paths.
// The CE name comes from the top-level cognition_engines config.
// When adding support for a new CE type, add its endpoint mapping here.
//
// Implementation Status:
//   - Knowledge Extraction CE:   ✅ Implemented (POST /api/knowledge-mgmt/extraction)
//   - Memory Distillation CE:     ⏳ TODO: CE must implement this endpoint before scheduling works
var ceNameToEndpoint = map[string]string{
	// TODO: CE must implement /api/knowledge-mgmt/runDistillation endpoint
	// This endpoint should accept a TaskExecutionRequest and return 202 Accepted with a TaskExecutionResponse.
	// Until implemented, Memory Distillation CEs with schedules will fail to dispatch.
	"Memory Distillation CE": "/api/knowledge-mgmt/runDistillation",

	// ✅ Implemented: Handles OTEL span extraction and knowledge ingestion
	"Knowledge Extraction CE": "/api/knowledge-mgmt/extraction",
}

// GetEndpointForCE returns the endpoint path for a given CE name.
// Returns empty string if no endpoint mapping exists.
func GetEndpointForCE(ceName string) string {
	return ceNameToEndpoint[ceName]
}
