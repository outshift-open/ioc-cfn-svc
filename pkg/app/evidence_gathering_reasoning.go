package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ReasonerPayload struct {
	Metadata          map[string]any   `json:"metadata,omitempty"`
	Intent            string           `json:"intent"`
	AdditionalContext []map[string]any `json:"additional_context,omitempty"`
}

type ReasonerEnvelopeRequest struct {
	Header    Header          `json:"header"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   ReasonerPayload `json:"payload"`
}

type ReasonerError struct {
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
}

type TKFKnowledgeRecord struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Content map[string]any `json:"content"`
}

type ReasonerEnvelopeResponse struct {
	Header     Header               `json:"header"`
	ResponseID string               `json:"response_id"`
	Error      *ReasonerError       `json:"error,omitempty"`
	Records    []TKFKnowledgeRecord `json:"records,omitempty"`
	Metadata   map[string]any       `json:"metadata,omitempty"`
}

type EvidenceAgentError struct {
	ResponseID string
	Message    string
	Detail     map[string]any
}

func (e EvidenceAgentError) Error() string {
	return e.Message
}

// InvokeReasoningEvidence builds the request payload and performs the POST call to the Evidence Gathering Agent: https://github.com/cisco-eti/ioc-cfn-cognitive-agents/blob/main/evidence-gathering-agent/app/api/routes.py#L18-L20
func InvokeReasoningEvidence(
	ctx context.Context,
	endpoint string,

// header / scope
	workspaceID string,
	masID string,
	agentID string,

// request
	requestID string,
	intent string,
	additionalContext []map[string]any,
	metadata map[string]any,
) (*ReasonerEnvelopeResponse, error) {
	if workspaceID == "" || masID == "" {
		return nil, fmt.Errorf("workspaceID and masID are required")
	}

	if intent == "" {
		return nil, fmt.Errorf("intent must not be empty")
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	reqPayload := ReasonerEnvelopeRequest{
		Header: Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     agentID,
		},
		RequestID: requestID,
		Payload: ReasonerPayload{
			Intent:            intent,
			Metadata:          metadata,
			AdditionalContext: additionalContext,
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("evidence gathering agent POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("evidence gathering agent returned status %d", resp.StatusCode)
	}

	var envResp ReasonerEnvelopeResponse
	if err := json.NewDecoder(resp.Body).Decode(&envResp); err != nil {
		return nil, fmt.Errorf("failed to decode evidence gathering agent response: %w", err)
	}

	if envResp.Error != nil {
		return nil, EvidenceAgentError{
			ResponseID: envResp.ResponseID,
			Message:    envResp.Error.Message,
			Detail:     envResp.Error.Detail,
		}
	}

	return &envResp, nil
}
