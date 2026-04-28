package common

type CognitionAgent string

const (
	FormatObserveSDKOTel = "observe-sdk-otel"
	FormatOpenClaw       = "openclaw"
	FormatSemNeg         = "semneg"
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
