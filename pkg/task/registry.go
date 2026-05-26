// Package task provides the task scheduling infrastructure for cfn-svc,
// including the task-name-to-endpoint registry and cron expression parsing.
package task

// registry maps each known task_name to its corresponding CE endpoint path.
// When a new scheduled task type is added, register it here.
var registry = map[string]string{
	"distillation-task": "/api/knowledge-mgmt/runDistillation",
	"otel-task":         "/api/knowledge-mgmt/runOtelTask",
}

// LookupEndpoint returns the CE endpoint path for the given task name.
// Returns false if the task name is not registered.
func LookupEndpoint(taskName string) (string, bool) {
	ep, ok := registry[taskName]
	return ep, ok
}

// IsRegistered reports whether the given task name exists in the registry.
func IsRegistered(taskName string) bool {
	_, ok := registry[taskName]
	return ok
}
