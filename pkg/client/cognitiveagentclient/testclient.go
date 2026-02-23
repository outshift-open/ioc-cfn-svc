package cognitiveagentclient

// RunTestClient is a smoke-test helper that sends sample requests to the
// Cognitive Agent API endpoints and prints the responses to stdout.
//
// Usage (from anywhere in the codebase):
//
//   import "github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitiveagentclient"
//
//   cognitiveagentclient.RunTestClient("http://localhost:8000")
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

// RunTestClient exercises all cognitive agent API endpoints.
func RunTestClient(baseURL string) {
	fmt.Printf("Cognitive Agent Test Client — base URL: %s\n\n", baseURL)

	client := New(baseURL, 30*time.Second)
	ctx := context.Background()

	testSendExtraction(ctx, client)
	testSendReasoningEvidence(ctx, client)
	testSendSemanticNegotiation(ctx, client)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testSendExtraction sends a sample extraction request to POST /api/knowledge-mgmt/extraction.
func testSendExtraction(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/knowledge-mgmt/extraction ===")

	req := &ExtractionRequest{
		Header: Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		Payload: ExtractionPayload{
			Metadata: ExtractionPayloadMetadata{
				Format: "observe-sdk-otel",
			},
			Data: []ExtractionDataRecord{
				{
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
			},
		},
	}

	resp, err := client.SendExtraction(ctx, req)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printExtractionResponse(resp)
}

// testSendReasoningEvidence sends a sample reasoning evidence request to POST /api/knowledge-mgmt/reasoning/evidence.
func testSendReasoningEvidence(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/knowledge-mgmt/reasoning/evidence ===")

	req := &ReasoningEvidenceRequest{
		Header: Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		Payload: ReasoningEvidencePayload{
			Metadata: ReasoningEvidencePayloadMetadata{
				QueryType: "Semantic Graph Traversal",
			},
			Intent:            "what does the orchestrator do?",
			AdditionalContext: []interface{}{},
		},
	}

	resp, err := client.SendReasoningEvidence(ctx, req)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printJSON("ReasonerCognitionResponse", resp)
}

// testSendSemanticNegotiation sends a sample semantic negotiation request to POST /api/semantic-negotiation.
func testSendSemanticNegotiation(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/semantic-negotiation ===")

	req := &SemanticNegotiationRequest{
		Header: Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		Payload: SemanticNegotiationPayload{},
	}

	resp, err := client.SendSemanticNegotiation(ctx, req)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printJSON("SemanticNegotiationResponse", resp)
}

// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

// printExtractionResponse prints a structured summary of the extraction response.
func printExtractionResponse(resp *KnowledgeCognitionResponse) {
	fmt.Printf("  Response ID: %s\n", resp.ResponseID)
	fmt.Printf("  Header: workspace=%s, mas=%s, agent=%s\n",
		resp.Header.WorkspaceID, resp.Header.MASID, resp.Header.AgentID)
	if resp.Error != nil {
		fmt.Printf("  Error: %s\n", resp.Error.Message)
		return
	}
	fmt.Printf("  Descriptor: %s\n", resp.Descriptor)
	fmt.Printf("  Concepts (%d):\n", len(resp.Concepts))
	for _, c := range resp.Concepts {
		fmt.Printf("    - %s (%s): %s\n", c.Name, c.Type, c.Description)
	}
	fmt.Printf("  Relations (%d):\n", len(resp.Relations))
	for _, r := range resp.Relations {
		fmt.Printf("    - %s: %v\n", r.Relationship, r.NodeIDs)
	}
	fmt.Printf("  Metadata: %d records, %d concepts, %d relations\n",
		resp.Metadata.RecordsProcessed, resp.Metadata.ConceptsExtracted, resp.Metadata.RelationsExtracted)
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
