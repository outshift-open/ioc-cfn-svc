// Package task provides the task scheduling infrastructure for cfn-svc,
// including CE endpoint mapping and cron expression parsing.
package task

// ceKindToEndpoint maps CE (kind, subkind) combinations to their corresponding endpoint paths.
// When adding support for a new CE type, add its endpoint mapping here.
//
// Implementation Status:
//   - extraction:   ✅ Implemented (POST /api/knowledge-mgmt/extraction)
//   - distillation: ⏳ TODO: CE must implement this endpoint before scheduling works
//
// Example: A CE with kind="knowledge" and subkind="distillation" will be dispatched to
// "/api/knowledge-mgmt/runDistillation" when its scheduled task fires.
var ceKindToEndpoint = map[string]map[string]string{
	"knowledge": {
		// TODO: CE must implement /api/knowledge-mgmt/runDistillation endpoint
		// This endpoint should accept a TaskExecutionRequest and return 202 Accepted with a TaskExecutionResponse.
		// Until implemented, distillation CEs with schedules will fail to dispatch.
		"distillation": "/api/knowledge-mgmt/runDistillation",

		// ✅ Implemented: Handles OTEL span extraction and knowledge ingestion
		"extraction": "/api/knowledge-mgmt/extraction",
	},
}

// GetEndpointForCE returns the endpoint path for a given CE kind and subkind.
// Returns empty string if no endpoint mapping exists for this CE type.
//
// This is called at task dispatch time to determine where to send the TaskExecutionRequest.
func GetEndpointForCE(kind, subkind string) string {
	if subkinds, ok := ceKindToEndpoint[kind]; ok {
		return subkinds[subkind]
	}
	return ""
}
