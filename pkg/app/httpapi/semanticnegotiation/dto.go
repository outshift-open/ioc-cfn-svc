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
//
// For /negotiate/initiate the upstream returns an SSTPNegotiateMessage envelope;
// the full envelope is surfaced under Envelope and the negotiation status is
// extracted into Status for convenience.
//
// For /negotiate/decide the upstream returns a flat JSON object; Status,
// SessionID, Round, Messages, and FinalResult are populated directly.
type Response struct {
	// Status indicates the result of the negotiation step.
	// Possible values: "ongoing", "agreed", "broken", "timeout", "broken" (error).
	Status string `json:"status,omitempty"`

	// SessionID echoes the session identifier from the upstream response.
	SessionID string `json:"session_id,omitempty"`

	// Round is the SAO round number that was evaluated.
	Round *int `json:"round,omitempty"`

	// Messages contains the next round's SSTP messages when Status is "ongoing"
	// (returned by /negotiate/decide).
	Messages []interface{} `json:"messages,omitempty"`

	// FinalResult holds the terminal negotiation envelope when Status is
	// "agreed", "broken", or "timeout" (returned by /negotiate/decide).
	FinalResult map[string]interface{} `json:"final_result,omitempty"`

	// Envelope is the full SSTPNegotiateMessage returned by /negotiate/initiate.
	Envelope map[string]interface{} `json:"envelope,omitempty"`
}
