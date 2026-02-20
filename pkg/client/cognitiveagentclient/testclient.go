package cognitiveagentclient

// RunTestClient is a smoke-test helper that sends a sample request to a
// Cognitive Agent API endpoint and prints the response to stdout.
//
// Supported endpoint values:
//
//   - "otel"     — POST /api/_otel      (sample OpenTelemetry spans)
//   - "general"  — POST /api/_general   (general knowledge cognition request)
//   - "reasoner" — POST /api/_reasoner  (reasoner evidence request with intent query)
//   - "all"      — runs all three endpoints sequentially
//
// Usage (from anywhere in the codebase):
//
//   import "github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitiveagentclient"
//
//   cognitiveagentclient.RunTestClient("http://localhost:8000", "all")
//   cognitiveagentclient.RunTestClient("http://localhost:8000", "otel")
//   cognitiveagentclient.RunTestClient("http://localhost:8000", "reasoner")
//
// Set the base URL to the running Cognitive Agent service address.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// RunTestClient exercises the specified cognitive agent API endpoint.
// Pass "all" to run every endpoint, or one of "otel", "general", "reasoner".
func RunTestClient(baseURL string, endpoint string) {
	fmt.Printf("Cognitive Agent Test Client — base URL: %s, endpoint: %s\n\n", baseURL, endpoint)

	client := New(baseURL, 30*time.Second)
	ctx := context.Background()

	switch endpoint {
	case "otel":
		testSendOtelSpans(ctx, client)
	case "general":
		testSendGeneral(ctx, client)
	case "reasoner":
		testSendReasonerEvidence(ctx, client)
	case "all":
		testSendOtelSpans(ctx, client)
		testSendGeneral(ctx, client)
		testSendReasonerEvidence(ctx, client)
	default:
		fmt.Printf("unknown endpoint %q: use \"otel\", \"general\", \"reasoner\", or \"all\"\n", endpoint)
	}
}

// ---------------------------------------------------------------------------
// Per-endpoint test helpers
// ---------------------------------------------------------------------------

// testSendOtelSpans sends a sample OtelSpan to POST /api/_otel.
func testSendOtelSpans(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/_otel ===")

	spans := []OtelSpan{
		{
			MASID:        "sample-mas-id",
			WorkspaceID:  "sample-workspace-id",
			TraceID:      "162b29522a339e6b1acb21b8041dcda5",
			SpanID:       "2b6a701a27797f5c",
			ParentSpanID: "",
			SpanName:     "farm_agent.build_graph.agent",
			ServiceName:  "corto.farm_agent",
			SpanAttributes: map[string]string{
				"agent_id":             "farm_agent.build_graph",
				"gen_ai.request.model": "gpt-4",
			},
			Duration: 21346166,
		},
	}

	resp, err := client.SendOtelSpans(ctx, spans)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printKnowledgeCognitionResponse(resp)
}

// testSendGeneral sends a sample GeneralRequest to POST /api/_general.
func testSendGeneral(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/_general ===")

	requests := []GeneralRequest{
		{
			KnowledgeCognitionRequestID: "30cbc343f7a41aa2c91bdffc08b99f1f",
			MASID:                       "sample-mas-id",
			WorkspaceID:                 "sample-workspace-id",
		},
	}

	resp, err := client.SendGeneral(ctx, requests)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printJSON("GeneralCognitionResponse", resp)
}

// testSendReasonerEvidence sends a sample ReasonerRequest to POST /api/_reasoner.
func testSendReasonerEvidence(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/_reasoner ===")

	request := &ReasonerRequest{
		MASID:             "sample-mas-id",
		WorkspaceID:       "sample-workspace-id",
		Intent:            "what does the orchestrator do?",
		AdditionalContext: []interface{}{},
		Meta:              map[string]interface{}{},
	}

	resp, err := client.SendReasonerEvidence(ctx, request)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printJSON("ReasonerCognitionResponse", resp)
}

// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

// printKnowledgeCognitionResponse prints a structured summary of the _otel response.
func printKnowledgeCognitionResponse(resp *KnowledgeCognitionResponse) {
	fmt.Printf("  Request ID: %s\n", resp.KnowledgeCognitionRequestID)
	fmt.Printf("  Descriptor: %s\n", resp.Descriptor)
	fmt.Printf("  Concepts (%d):\n", len(resp.Concepts))
	for _, c := range resp.Concepts {
		fmt.Printf("    - %s (%s): %s\n", c.Name, c.Type, c.Description)
	}
	fmt.Printf("  Relations (%d):\n", len(resp.Relations))
	for _, r := range resp.Relations {
		fmt.Printf("    - %s: %v\n", r.Relationship, r.NodeIDs)
	}
	fmt.Printf("  Meta: %d records, %d concepts, %d relations\n",
		resp.Meta.RecordsProcessed, resp.Meta.ConceptsExtracted, resp.Meta.RelationsExtracted)
}

// printJSON pretty-prints any response as indented JSON with a label.
func printJSON(label string, v interface{}) {
	raw, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		fmt.Printf("  %s: failed to marshal: %v\n", label, err)
		return
	}
	fmt.Printf("  %s: %s\n", label, string(raw))
}
