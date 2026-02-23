package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type IngestionPayload struct {
	Metadata map[string]any  `json:"metadata,omitempty"`
	Data     json.RawMessage `json:"data"`
}

type IngestionEnvelopeRequest struct {
	Header    Header           `json:"header"`
	RequestID string           `json:"request_id,omitempty"`
	Payload   IngestionPayload `json:"payload"`
}

func ExtractEntitiesAndRelationsBatch(
	ctx context.Context,
	endpoint string,

// header / scope
	workspaceID string,
	masID string,
	agentID string,

// request
	requestID string,
	format string, // observe-sdk-otel | openclaw
	otelTraceJSON []byte,
	saveOutput bool,
) (map[string]any, error) {

	if workspaceID == "" || masID == "" {
		return nil, fmt.Errorf("workspaceID and masID are required")
	}

	if format == "" {
		return nil, fmt.Errorf("metadata.format is required")
	}

	// Build envelope request
	reqPayload := IngestionEnvelopeRequest{
		Header: Header{
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     agentID,
		},
		RequestID: requestID,
		Payload: IngestionPayload{
			Metadata: map[string]any{
				"format": format,
			},
			Data: json.RawMessage(otelTraceJSON),
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ingestion request: %w", err)
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	q := u.Query()
	q.Set("save_output", fmt.Sprintf("%t", saveOutput))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u.String(),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("extraction batch POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode extraction response: %w", err)
	}

	return result, nil
}
