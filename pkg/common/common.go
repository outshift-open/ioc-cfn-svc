package common

type CognitionAgent string

const (
	FormatObserveSDKOTel = "observe-sdk-otel"
	FormatOpenClaw       = "openclaw"
	FormatSemNeg         = "semneg"
	FormatOTelTrace      = "otel-trace"
)

// Header carries routing context for CFN requests and responses.
type Header struct {
	WorkspaceID string `json:"workspace_id"`       // Mandatory
	MASID       string `json:"mas_id"`             // Mandatory
	AgentID     string `json:"agent_id,omitempty"` // Optional
}

// ErrorDetail provides debugging information when an error occurs.
type ErrorDetail struct {
	Message string                 `json:"message"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
}

func StrToPtr(s string) *string {
	return &s
}

func BoolToPtr(b bool) *bool {
	return &b
}

// TokenUsage represents LLM token consumption metrics.
type TokenUsage struct {
	Prompt     int    `json:"prompt"`
	Completion int    `json:"completion"`
	Total      int    `json:"total"`
	Model      string `json:"model"`
}

// TokenUsageMeta contains LLM call metadata including token usage and CE attribution.
type TokenUsageMeta struct {
	Tokens    TokenUsage `json:"tokens"`
	LatencyMs float64    `json:"latency_ms"`
	CostUsd   *float64   `json:"cost_usd,omitempty"`
	Timestamp string     `json:"timestamp"`
	CEID      string     `json:"ce_id,omitempty"` // CE that performed the operation (for metrics attribution)
}
