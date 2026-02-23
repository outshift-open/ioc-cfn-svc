package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ReasonerCognitionRequest mirrors the Python model of "ReasonerCognitionRequest" inside the Evidence Gathering Agent
type ReasonerCognitionRequest struct {
	ReasonerCognitionRequestID string           `json:"reasoner_cognition_request_id,omitempty"`
	Intent                     string           `json:"intent"`
	Records                    []map[string]any `json:"records,omitempty"`
	Meta                       map[string]any   `json:"meta,omitempty"`
}

// ReasonerCognitionResponse mirrors the Python model of "ReasonerCognitionResponse" inside the Evidence Gathering Agent
type ReasonerCognitionResponse struct {
	Status                     string            `json:"status"`
	ReasonerCognitionRequestID string            `json:"reasoner_cognition_request_id"`
	Records                    []KnowledgeRecord `json:"records"`
	Meta                       map[string]any    `json:"meta,omitempty"`
}

// KnowledgeRecord mirrors the Python model of "TKFKnowledgeRecord" inside the Evidence Gathering Agent
type KnowledgeRecord struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Content map[string]any `json:"content"`
}

// InvokeReasoningEvidence builds the request payload and performs the POST call to the Evidence Gathering Agent: https://github.com/cisco-eti/ioc-cfn-cognitive-agents/blob/main/evidence-gathering-agent/app/api/routes.py#L18-L20
func InvokeReasoningEvidence(
	ctx context.Context,
	endpoint string,
	intent string,
	records []map[string]any,
	meta map[string]any,
) (*ReasonerCognitionResponse, error) {
	reqPayload := ReasonerCognitionRequest{
		//ReasonerCognitionRequestID: "", TODO: get the request ID from somewhere
		Intent:  intent,
		Records: records,
		Meta:    meta,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reasoner request: %w", err)
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
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("evidence gathering agent POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("evidence gathering agent returned status %d", resp.StatusCode)
	}

	var reasonerResp ReasonerCognitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&reasonerResp); err != nil {
		return nil, fmt.Errorf("failed to decode evidence gathering agent response: %w", err)
	}

	return &reasonerResp, nil
}
