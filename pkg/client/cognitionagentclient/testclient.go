package cognitionagentclient

// RunTestClient is a smoke-test helper that sends sample requests to the
// cognition agent API endpoints and prints the responses to stdout.
//
// Usage (from anywhere in the codebase):
//
//   import "github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
//
//   cognitionagentclient.RunTestClient("http://localhost:8000")
//
// Set the base URL to the running cognition agent service address.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// RunTestClient exercises all cognition agent API endpoints.
func RunTestClient(baseURL string) {
	fmt.Printf("cognition agent Test Client — base URL: %s\n\n", baseURL)

	client := New(baseURL, 30*time.Second)
	ctx := context.Background()

	//testSendExtractionOtel(ctx, client)
	testSendReasoningEvidence(ctx, client)
	//testSendSemanticNegotiation(ctx, client)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testSendExtraction sends a sample extraction request to POST /api/knowledge-mgmt/extraction.
func testSendExtractionOtel(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/knowledge-mgmt/extraction with otel trace===")

	req := &ExtractionRequest{
		Header: common.Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		RequestID: uuid.New().String(),
		Payload: ExtractionPayload{
			Metadata: ExtractionPayloadMetadata{
				Format: "observe-sdk-otel",
			},
			Data: json.RawMessage(`[
				{
					"TraceId": "test_trace_001",
					"SpanId": "span_001",
					"ParentSpanId": "",
					"SpanName": "test.agent",
					"SpanKind": "Server",
					"ServiceName": "test.service",
					"SpanAttributes": {
						"agent_id": "test_agent",
						"gen_ai.request.model": "gpt-4o",
						"gen_ai.prompt.0.role": "user",
						"gen_ai.prompt.0.content": "Tell me about testing."
					},
					"Duration": 1000000
				},
				{
					"TraceId": "test_trace_001",
					"SpanId": "span_002",
					"ParentSpanId": "span_001",
					"SpanName": "child.agent",
					"SpanKind": "Client",
					"ServiceName": "test.service",
					"SpanAttributes": {
						"agent_id": "child_agent",
						"gen_ai.request.model": "gpt-4o"
					},
					"Duration": 500000
				}
			]`),
		},
	}

	resp, err := client.SendExtraction(ctx, req)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printExtractionResponse(resp)
}

func testSendExtractionOpenClaw(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/knowledge-mgmt/extraction with openclaw output ===")

	req := &ExtractionRequest{
		Header: common.Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		RequestID: uuid.New().String(),
		Payload: ExtractionPayload{
			Metadata: ExtractionPayloadMetadata{
				Format: "openclaw",
			},
			Data: json.RawMessage(`{
			  "schema": "openclaw-conversation-v1",
			  "extractedAt": "2026-02-21T23:04:25.893Z",
			  "session": {
				"agentId": "main",
				"sessionId": "e94911bc-d738-4847-8632-223e48813533",
				"sessionKey": "agent:main:main",
				"channel": "main",
				"cwd": "/Users/juliavalenti/.openclaw/sandboxes/agent-main-f331f052"
			  },
			  "stats": {
				"totalEntries": 85,
				"turns": 18,
				"toolCallCount": 15,
				"thinkingTurnCount": 18,
				"totalCost": 0.65891565
			  },
			  "data": [
				{
				  "trace_id": "162b29522a339e6b1acb21b8041dcda5",
				  "span_id": "2b6a701a27797f5c",
				  "parent_span_id": "",
				  "span_name": "farm_agent.build_graph.agent",
				  "service_name": "corto.farm_agent",
				  "span_attributes": {
					"agent_id": "farm_agent.build_graph",
					"gen_ai.request.model": "gpt-4"
				  },
				  "duration": 21346166
				}
			  ],
			  "turns": [
				{
				  "index": 0,
				  "timestamp": "2026-02-20T22:24:34.636Z",
				  "model": "claude-sonnet-4-5-20250929",
				  "stopReason": "stop",
				  "usage": {
					"input": 43,
					"output": 479,
					"cacheRead": 37832,
					"cacheWrite": 1180,
					"totalTokens": 39534,
					"cost": {
					  "input": 0.00012900000000000002,
					  "output": 0.0071849999999999995,
					  "cacheRead": 0.0113496,
					  "cacheWrite": 0.004425,
					  "total": 0.0230886
					}
				  },
				  "userMessage": "[Fri 2026-02-20 14:24 PST] Run the shell command 'sleep 30' and then tell me what time it is.",
				  "thinking": "The user wants me to:\n1. Run the shell command 'sleep 30' ...",
				  "toolCalls": [],
				  "response": "I'll run the sleep command and then tell you the time..."
				}
			  ]
			}
			`),
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
		Header: common.Header{
			WorkspaceID: "sample-workspace-id",
			MASID:       "sample-mas-id",
			AgentID:     "sample-agent-id",
		},
		RequestID: common.StrToPtr(uuid.New().String()),
		Payload: ReasoningEvidencePayload{
			Metadata: ReasoningEvidencePayloadMetadata{
				QueryType: "Semantic Graph Traversal",
			},
			Intent:            "what does the concierge_agent do?",
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

// testSendSemanticNegotiationStart sends a sample semantic negotiation start request.
func testSendSemanticNegotiationStart(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/semantic-negotiation/start ===")

	nSteps := 20
	req := &SemanticNegotiationStartRequest{
		SessionID:   "sample-session-id",
		ContentText: "Let's negotiate the task allocation for the project",
		Agents: []SemanticNegotiationAgent{
			{ID: "agent-1", Name: "Alice"},
			{ID: "agent-2", Name: "Bob"},
		},
		NSteps: &nSteps,
	}

	resp, err := client.SendSemanticNegotiationStart(ctx, req, "sample-workspace-id", "sample-mas-id")
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}

	printJSON("SemanticNegotiationResponse", resp)
}

// testSendSemanticNegotiationDecide sends a sample semantic negotiation decide request.
func testSendSemanticNegotiationDecide(ctx context.Context, client *Client) {
	fmt.Println("=== POST /api/semantic-negotiation/decide ===")

	req := &SemanticNegotiationDecideRequest{
		SessionID: "sample-session-id",
		AgentReplies: []SemanticNegotiationAgentReply{
			{
				AgentID: "agent-1",
				Action:  "counter_offer",
				Offer:   map[string]interface{}{"task": "backend", "hours": 40},
			},
		},
	}

	resp, err := client.SendSemanticNegotiationDecide(ctx, req, "sample-workspace-id", "sample-mas-id")
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
