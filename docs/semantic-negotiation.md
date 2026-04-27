# Semantic Negotiation via CFN

The CFN acts as a proxy between a MAS client and the Cognition Engine server
([`semantic_negotiation`](https://github.com/cisco-eti/ioc-cfn-cognitive-agents/tree/main/semantic_negotiation)).
The client drives the negotiation loop turn-by-turn; CFN wraps each request in the
required SSTP envelope and forwards it to the negotiation server.

## Architecture

```text
MAS Client
    │
    │  POST /semantic-negotiation/start
    │  POST /semantic-negotiation/decide  (repeated)
    ▼
CFN (ioc-cfn-svc)
    │  wraps in SSTPNegotiateMessage envelope
    │
    │  POST /api/semantic-negotiation/negotiate/initiate
    │  POST /api/semantic-negotiation/negotiate/decide          (repeated)
    ▼
Cognition Engine Server (Semantic Negotiation)
```

## Endpoints

| CFN Route | Method | Proxies to |
| --- | --- | --- |
| `/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/start` | POST | `POST /api/semantic-negotiation/negotiate/initiate` |
| `/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-negotiation/decide` | POST | `POST /api/semantic-negotiation/negotiate/decide` |

---

## Step 1 — Start a negotiation session

### Request

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-negotiation/start \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-test-001",
    "content_text": "Negotiate a software project contract covering timeline, budget, and support terms.",
    "agents": [
      {"id": "agent-a", "name": "Agent A"},
      {"id": "agent-b", "name": "Agent B"},
      {"id": "agent-c", "name": "Agent C"}
    ],
    "n_steps": 20
  }'
```

**Fields:**

| Field | Required | Description |
| --- | --- | --- |
| `session_id` | yes | Unique identifier for this negotiation session |
| `content_text` | yes | Natural-language description of what is being negotiated |
| `agents` | yes | List of participating agents (`id` + `name`) |
| `n_steps` | no | Maximum SAO rounds before timeout (default: 20) |

### Response

The Cognition Engine discovers issues and options from `content_text` and returns the
first round's messages inside `envelope`.

```json
{
  "status": "initiated",
  "envelope": {
    "status": "initiated",
    "session_id": "sess-test-001",
    "round": 1,
    "issues": ["timeline", "budget", "support terms"],
    "options_per_issue": {
      "timeline": ["3 months for delivery", "6 months for delivery", "9 months for delivery", "12 months for delivery"],
      "budget": ["$10,000", "$25,000", "$50,000", "$100,000"],
      "support terms": ["No support after project delivery", "3 months of support post-delivery", "6 months of ongoing support", "12 months of ongoing support"]
    },
    "messages": [
      {
        "kind": "negotiate",
        "version": "0",
        "message_id": "0ba39775-9d79-5fe8-b641-ca01c081a0bb",
        "dt_created": "2026-04-23T17:23:06.036542+00:00",
        "origin": { "actor_id": "negotiation-server", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "issues": ["timeline", "budget", "support terms"],
          "options_per_issue": { "...": "..." }
        },
        "payload": {
          "action": "respond",
          "participant_id": "server",
          "next_proposer_id": "agent-a",
          "proposer_id": "server",
          "round": 1,
          "n_steps": 20,
          "current_offer": {
            "timeline": "3 months for delivery",
            "budget": "$50,000",
            "support terms": "12 months of ongoing support"
          },
          "allowed_actions": ["accept", "reject", "counter_offer"]
        },
        "payload_hash": "...",
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] }
      }
    ]
  }
}
```

Use `envelope.messages[0].payload` to understand the current offer and which agent
should propose next (`next_proposer_id`), then construct your `agent_replies` for
Step 2.

---

## Step 2 — Advance the negotiation

POST agent replies as full `SSTPNegotiateMessage` objects in `agent_replies`.
The `agent_replies` field carries raw SSTP envelopes verbatim — **not** simplified
`{agent_id, action, offer}` dicts.

Each reply's `semantic_context.sao_response` encodes the agent's decision:

| `response` | Meaning | `outcome` |
| --- | --- | --- |
| `0` | ACCEPT_OFFER | The accepted offer dict |
| `1` + `outcome` dict | REJECT_OFFER + counter-offer | The counter-offer dict |
| `1` + `outcome: null` | REJECT_OFFER (hard reject) | `null` |

### Example — round 1 (mixed responses)

`next_proposer_id` is `agent-a`, so agent-a counter-offers while the others respond:

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-negotiation/decide \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-test-001",
    "agent_replies": [
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-a", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": {
            "response": 1,
            "outcome": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" }
          }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": {
          "action": "counter_offer", "round": 1, "participant_id": "agent-a",
          "offer": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" }
        }
      },
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-b", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": { "response": 1, "outcome": null }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": { "action": "reject", "round": 1, "participant_id": "agent-b" }
      },
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-c", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": {
            "response": 0,
            "outcome": { "timeline": "3 months for delivery", "budget": "$50,000", "support terms": "12 months of ongoing support" }
          }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": { "action": "accept", "round": 1, "participant_id": "agent-c" }
      }
    ]
  }'
```

### Example — accept all (terminal round)

When all agents accept the standing offer the negotiation concludes. Set every
agent's `sao_response.response` to `0` and `outcome` to the current offer:

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-negotiation/decide \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-test-001",
    "agent_replies": [
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-a", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": { "response": 0, "outcome": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" } }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": { "action": "accept", "round": 2, "participant_id": "agent-a" }
      },
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-b", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": { "response": 0, "outcome": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" } }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": { "action": "accept", "round": 2, "participant_id": "agent-b" }
      },
      {
        "kind": "negotiate",
        "origin": { "actor_id": "agent-c", "tenant_id": "sess-test-001" },
        "semantic_context": {
          "session_id": "sess-test-001",
          "sao_response": { "response": 0, "outcome": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" } }
        },
        "policy_labels": { "sensitivity": "internal", "propagation": "restricted", "retention_policy": "default" },
        "provenance": { "sources": [], "transforms": [] },
        "payload": { "action": "accept", "round": 2, "participant_id": "agent-c" }
      }
    ]
  }'
```

---

## Responses

### Ongoing

Negotiation continues. Construct new `agent_replies` based on `messages` and call
decide again.

```json
{
  "status": "ongoing",
  "session_id": "sess-test-001",
  "round": 2,
  "messages": [{ "kind": "negotiate", "payload": { "action": "respond", "round": 2, "current_offer": { "..." } } }]
}
```

### Agreed

All agents accepted the same offer. `final_result` is the full `SSTPCommitMessage`.

```json
{
  "status": "agreed",
  "session_id": "sess-test-001",
  "round": 2,
  "final_result": {
    "kind": "commit",
    "semantic_context": {
      "final_agreement": [
        { "issue_id": "timeline",      "chosen_option": "6 months for delivery" },
        { "issue_id": "budget",        "chosen_option": "$25,000" },
        { "issue_id": "support terms", "chosen_option": "3 months of support post-delivery" }
      ],
      "outcome": "agreement"
    },
    "payload": {
      "status": "agreed",
      "total_rounds": 2,
      "trace": {
        "timedout": false,
        "broken": false,
        "rounds": [
          {
            "round": 1,
            "proposer_id": "server",
            "offer": { "timeline": "3 months for delivery", "budget": "$50,000", "support terms": "12 months of ongoing support" },
            "decisions": [
              { "participant_id": "agent-a", "action": "counter_offer", "offer": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" } },
              { "participant_id": "agent-b", "action": "reject", "offer": null },
              { "participant_id": "agent-c", "action": "accept", "offer": null }
            ]
          },
          {
            "round": 2,
            "proposer_id": "agent-a",
            "offer": { "timeline": "6 months for delivery", "budget": "$25,000", "support terms": "3 months of support post-delivery" },
            "decisions": [
              { "participant_id": "agent-a", "action": "accept", "offer": null },
              { "participant_id": "agent-b", "action": "accept", "offer": null },
              { "participant_id": "agent-c", "action": "accept", "offer": null }
            ]
          }
        ]
      }
    }
  }
}
```

### Terminal statuses

| `status` | Meaning |
| --- | --- |
| `agreed` | All agents accepted the same offer. `final_result` is present. |
| `broken` | Negotiation ended without agreement (e.g. all agents hard-rejected). |
| `timeout` | `n_steps` rounds elapsed without agreement. |

---
