package semanticnegotiation

import (
	"encoding/json"
)

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
	SessionID string `json:"session_id"`

	// ContentText is the negotiation prompt/context used to initialize the session.
	ContentText string `json:"content_text"`

	// Agents is the list of participating agents (minimum 2).
	Agents []Agent `json:"agents"`

	// NSteps is the maximum number of SAO rounds.
	// If omitted, the service computes a budget from negotiation complexity.
	NSteps *int `json:"n_steps,omitempty"`
}

// AgentDecision is one participant's decision within a SAO round.
type AgentDecision struct {
	// ParticipantID is the ID of the participant who made this decision.
	ParticipantID string `json:"participant_id"`

	// Action is one of "accept", "reject", or "counter_offer".
	Action string `json:"action"`

	// Offer is the proposed option per issue when Action is "counter_offer".
	// Shape: {issue_id: option_label}
	Offer map[string]string `json:"offer,omitempty"`
}

// RoundOffer is the offer produced in a single SAO round.
type RoundOffer struct {
	// Round is the 1-based round number.
	Round int `json:"round"`

	// ProposerID is the ID of the participant who made this proposal.
	ProposerID string `json:"proposer_id"`

	// NextProposerID is the ID of the participant who will propose in the next round.
	// Omitted on the final round.
	NextProposerID *string `json:"next_proposer_id,omitempty"`

	// Offer is the proposed option per issue. Shape: {issue_id: option_label}
	Offer map[string]string `json:"offer"`

	// Decisions contains each participant's response to this round's offer.
	Decisions []AgentDecision `json:"decisions,omitempty"`
}

// NegotiationOutcome is the agreed value for a single issue.
type NegotiationOutcome struct {
	IssueID       string `json:"issue_id"`
	ChosenOption  string `json:"chosen_option"`
}

// NegotiationTrace is the full pre-computed SAO trace returned by /start.
type NegotiationTrace struct {
	// Rounds contains all SAO rounds in order (1-based).
	Rounds []RoundOffer `json:"rounds"`

	// FinalAgreement is the agreed option per issue, if any.
	FinalAgreement []NegotiationOutcome `json:"final_agreement,omitempty"`

	// Timedout indicates whether the SAO exhausted its step budget.
	Timedout bool `json:"timedout"`

	// Broken indicates whether a participant explicitly broke off.
	Broken bool `json:"broken"`
}

// StartResponse is the response from the /start endpoint.
type StartResponse struct {
	// Status is "initiated", "ongoing", "agreed", "broken", or "timeout".
	Status string `json:"status"`

	// SessionID echoes the session identifier.
	SessionID string `json:"session_id"`

	// Issues is the list of negotiable issues discovered from content_text.
	Issues []string `json:"issues,omitempty"`

	// OptionsPerIssue lists the candidate options for each issue.
	OptionsPerIssue map[string][]string `json:"options_per_issue,omitempty"`

	// NSteps is the SAO round budget for this session.
	NSteps int `json:"n_steps,omitempty"`

	// Round is the current round number (1-based).
	Round int `json:"round,omitempty"`

	// Messages contains the first round's messages to dispatch to agents.
	// Forward each message to the corresponding agent's callback endpoint,
	// collect their replies, and submit via /decide.
	Messages []json.RawMessage `json:"messages,omitempty"`

	// CurrentRound is the first offer in the trace (SSTP envelope path only).
	CurrentRound *RoundOffer `json:"current_round,omitempty"`

	// TotalRounds is the total SAO rounds in the pre-computed trace (SSTP envelope path only).
	TotalRounds int `json:"total_rounds,omitempty"`

	// Trace is the complete pre-computed SAO trace (SSTP envelope path only).
	Trace *NegotiationTrace `json:"trace,omitempty"`
}

// AgentReply is one agent's reply to a negotiation round message.
type AgentReply struct {
	// ParticipantID is the ID of the agent replying.
	ParticipantID string `json:"participant_id"`

	// Action is one of "accept", "reject", or "counter_offer".
	Action string `json:"action"`

	// Offer is the proposed option per issue when Action is "counter_offer".
	// Shape: {issue_id: option_label}
	Offer map[string]string `json:"offer,omitempty"`
}

// DecideRequest is the request body to advance an existing negotiation session.
type DecideRequest struct {
	// SessionID is the session identifier from the /start response.
	SessionID string `json:"session_id"`

	// AgentReplies contains each agent's reply to the current round.
	AgentReplies []AgentReply `json:"agent_replies"`
}

// DecideResponse is the response from the /decide endpoint.
type DecideResponse struct {
	// SessionID echoes the session identifier.
	SessionID string `json:"session_id"`

	// Status is "ongoing", "agreed", "broken", or "timeout".
	Status string `json:"status"`

	// Round is the round number that was just evaluated.
	Round int `json:"round"`

	// Messages contains the next round's messages to dispatch to agents.
	// Present only when Status is "ongoing". Forward these to each agent's
	// callback endpoint, collect their replies, and submit via /decide again.
	Messages []json.RawMessage `json:"messages,omitempty"`

	// FinalResult holds the terminal negotiation envelope when Status is
	// "agreed", "broken", or "timeout".
	FinalResult map[string]interface{} `json:"final_result,omitempty"`
}
