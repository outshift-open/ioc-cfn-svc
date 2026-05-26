package task

var registry = map[string]string{
	"distillation-task": "/api/knowledge-mgmt/runDistillation",
	"otel-task":         "/api/knowledge-mgmt/runOtelTask",
}

func LookupEndpoint(taskName string) (string, bool) {
	ep, ok := registry[taskName]
	return ep, ok
}

func IsRegistered(taskName string) bool {
	_, ok := registry[taskName]
	return ok
}
