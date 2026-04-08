package semanticnegotiation

// Agent represents a participant in a semantic negotiation session.
type Agent struct {
	// ID is the unique agent identifier.
	ID string `json:"id"`
	// Name is the human-readable agent name.
	Name string `json:"name"`
}

// StartRequest is the request body to initiate a new semantic negotiation session.
type StartRequest struct {
	// SessionID is the client-provided session identifier.
	// Currently assumed globally unique (not scoped by workspace/mas).
	SessionID string `json:"session_id"`

	// ContentText is the negotiation prompt/context used to initialize the session.
	ContentText string `json:"content_text"`

	// Agents is the list of participating agents.
	Agents []Agent `json:"agents"`

	// NSteps is the maximum number of negotiation steps.
	// If omitted, defaults to 20.
	NSteps *int `json:"n_steps,omitempty"`
}

// AgentReply represents a single agent reply used to advance an existing session.
type AgentReply struct {
	// AgentID is the agent identifier (must match one of the initiated agents).
	AgentID string `json:"agent_id"`

	// Action is the agent action.
	// Allowed values: "accept", "reject", "counter_offer"
	Action string `json:"action"`

	// Offer is an optional structured offer payload.
	// Required when Action is "counter_offer".
	Offer map[string]interface{} `json:"offer,omitempty"`
}

// DecideRequest is the request body to advance an existing semantic negotiation session.
type DecideRequest struct {
	// SessionID is the session identifier previously provided to the start endpoint.
	SessionID string `json:"session_id"`

	// AgentReplies are the replies produced by agents since the last step.
	AgentReplies []AgentReply `json:"agent_replies"`
}

// Response is the response from semantic negotiation endpoints.
// The shape is defined by the semantic negotiation library.
type Response struct {
	// Status indicates the result of the negotiation step.
	Status string `json:"status,omitempty"`

	// Message provides additional information about the negotiation state.
	Message string `json:"message,omitempty"`

	// Result contains the pipeline execution result.
	// The structure depends on the semantic negotiation library implementation.
	Result map[string]interface{} `json:"result,omitempty"`
}
