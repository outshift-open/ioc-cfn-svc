# Semantic Alignment via Cognition Fabric Node (CFN)

The CFN acts as a proxy between a MAS client and the Cognition Engine server
([`semantic_alignment`](https://github.com/outshift-open/ioc-cfn-cognition-engines/tree/main/semantic_alignment)).
The client drives the negotiation loop turn-by-turn; CFN wraps each request in the
required SSTP envelope and forwards it to the negotiation server.

## Architecture

```text
MAS Client
    │
    │  POST /semantic-alignment/start
    │  POST /semantic-alignment/decide  (repeated)
    ▼
CFN (ioc-cfn-svc)
    │  wraps in SSTPNegotiateMessage envelope
    │
    │  POST /api/semantic-alignment/negotiate/initiate
    │  POST /api/semantic-alignment/negotiate/decide          (repeated)
    ▼
Cognition Engine Server (Semantic Alignment)
```

## Endpoints

| CFN Route | Method | Proxies to |
| --- | --- | --- |
| `/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/start` | POST | `POST /api/semantic-alignment/negotiate/initiate` |
| `/api/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/decide` | POST | `POST /api/semantic-alignment/negotiate/decide` |

---

## Step 1 — Start a negotiation session

### Request

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-alignment/start \
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
  "session_id": "sess-test-001",
  "issues": [
    "timeline",
    "budget",
    "support terms"
  ],
  "options_per_issue": {
    "budget": [
      "$10,000",
      "$50,000",
      "$100,000",
      "$200,000"
    ],
    "support terms": [
      "Email support only",
      "Email and chat support",
      "24/7 support with dedicated representative",
      "Support for 6 months after project completion"
    ],
    "timeline": [
      "3 months",
      "6 months",
      "9 months",
      "12 months"
    ]
  },
  "n_steps": 20,
  "round": 1,
  "messages": [
    {
      "dt_created": "2026-04-27T22:25:30.625912+00:00",
      "kind": "negotiate",
      "message_id": "e4f7a3e0-e4d5-559b-ac3a-769b8a803182",
      "origin": {
        "actor_id": "negotiation-server",
        "attestation": null,
        "tenant_id": "sess-test-001"
      },
      "payload": {
        "action": "respond",
        "allowed_actions": [
          "accept",
          "reject",
          "counter_offer"
        ],
        "current_offer": {
          "budget": "$10,000",
          "support terms": "Email support only",
          "timeline": "12 months"
        },
        "n_steps": 20,
        "next_proposer_id": "agent-a",
        "participant_id": "server",
        "proposer_id": "server",
        "round": 1
      },
      "payload_hash": "efb09dd314c1694107816b2dc0e85421459f2e291153eb25f3dded394997cc27",
      "semantic_context": {
        "encoding": "json",
        "issues": [
          "timeline",
          "budget",
          "support terms"
        ],
        "options_per_issue": {
          "budget": [
            "$10,000",
            "$50,000",
            "$100,000",
            "$200,000"
          ],
          "support terms": [
            "Email support only",
            "Email and chat support",
            "24/7 support with dedicated representative",
            "Support for 6 months after project completion"
          ],
          "timeline": [
            "3 months",
            "6 months",
            "9 months",
            "12 months"
          ]
        },
        "schema_id": "urn:ioc:schema:negotiate:negmas-sao:v1",
        "schema_version": "1.0",
        "session_id": "sess-test-001"
      },
      "version": "0"
    }
  ]
}
```

Use `messages[0].payload` to understand the current offer and which agent should
propose next (`next_proposer_id`), then construct your `agent_replies` for Step 2.

---

## Step 2 — Advance the negotiation

For each message in `messages`, forward it to the corresponding agent (identified by
`payload.participant_id`) and collect its reply. Submit all replies together in
`agent_replies`. CFN wraps each reply in the required SSTP envelope before forwarding
to the negotiation server — callers never deal with SSTP.

**`AgentReply` fields:**

| Field | Required | Description |
| --- | --- | --- |
| `participant_id` | yes | ID of the agent replying (matches `payload.participant_id` in the message) |
| `action` | yes | One of `"accept"`, `"reject"`, or `"counter_offer"` |
| `offer` | when `action` is `"counter_offer"` | Proposed option per issue: `{"issue_id": "option_label"}` |

### Example — round 1 (mixed responses)

`next_proposer_id` is `agent-a`, so agent-a counter-offers while the others respond:

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-alignment/decide \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-test-001",
    "agent_replies": [
      {
        "participant_id": "agent-a",
        "action": "counter_offer",
        "offer": { "timeline": "6 months", "budget": "$25,000", "support terms": "3 months of support post-delivery" }
      },
      {
        "participant_id": "agent-b",
        "action": "reject"
      },
      {
        "participant_id": "agent-c",
        "action": "accept"
      }
    ]
  }'
```

Example response:

```json
{
    "session_id": "sess-test-001",
    "status": "ongoing",
    "round": 3,
    "messages": [
        {
            "version": "0",
            "message_id": "000b3ce7-028a-5340-b665-58914c0a9ed9",
            "dt_created": "2026-04-27T22:32:20.905155+00:00",
            "origin": {
                "actor_id": "negotiation-server",
                "tenant_id": "sess-test-001",
                "attestation": null
            },
            "semantic_context": {
                "schema_id": "urn:ioc:schema:negotiate:negmas-sao:v1",
                "schema_version": "1.0",
                "encoding": "json",
                "session_id": "sess-test-001",
                "issues": [
                    "timeline",
                    "budget",
                    "support terms"
                ],
                "options_per_issue": {
                    "timeline": [
                        "3 months",
                        "6 months",
                        "9 months",
                        "12 months"
                    ],
                    "budget": [
                        "$10,000",
                        "$50,000",
                        "$100,000",
                        "$200,000"
                    ],
                    "support terms": [
                        "Email support only",
                        "Email and chat support",
                        "24/7 support with dedicated representative",
                        "Support for 6 months after project completion"
                    ]
                }
            },
            "payload_hash": "5028e819b50e8da33d640c2a2413650b472c4ad1c830e5b6fde36dba5471e42a",
            "payload": {
                "action": "respond",
                "participant_id": "server",
                "next_proposer_id": "agent-b",
                "round": 2,
                "n_steps": 20,
                "allowed_actions": [
                    "accept",
                    "reject",
                    "counter_offer"
                ],
                "current_offer": {
                    "timeline": "6 months",
                    "budget": "$25,000",
                    "support terms": "3 months of support post-delivery"
                },
                "proposer_id": "agent-a"
            },
            "kind": "negotiate"
        }
    ]
}
```
### Example — accept all (terminal round)

When all agents accept the standing offer the negotiation concludes:

```bash
curl -X POST http://localhost:<cfn-port>/api/workspaces/<workspaceId>/multi-agentic-systems/<masId>/semantic-alignment/decide \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-test-001",
    "agent_replies": [
      { "participant_id": "agent-a", "action": "accept" },
      { "participant_id": "agent-b", "action": "accept" },
      { "participant_id": "agent-c", "action": "accept" }
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
CFN automatically persists the agreement to shared memory (format `semneg`) so it
can be retrieved later via the shared memory API.

```json
{
  "session_id": "sess-test-001",
  "status": "agreed",
  "round": 2,
  "final_result": {
    "confidence_score": 1,
    "dt_created": "2026-04-27T22:34:02.814903+00:00",
    "kind": "commit",
    "logical_clock": {
      "type": "lamport",
      "value": 2
    },
    "merge_strategy": "add",
    "message_id": "sess-test-001",
    "origin": {
      "actor_id": "negotiation-server",
      "attestation": null,
      "tenant_id": "sess-test-001"
    },
    "parent_ids": [
      "sess-test-001"
    ],
    "payload": {
      "session_id": "sess-test-001",
      "status": "agreed",
      "total_rounds": 2,
      "trace": {
        "broken": false,
        "rounds": [
          {
            "decisions": [
              {
                "action": "counter_offer",
                "offer": {
                  "budget": "$25,000",
                  "support terms": "3 months of support post-delivery",
                  "timeline": "6 months"
                },
                "participant_id": "agent-a"
              },
              {
                "action": "reject",
                "offer": null,
                "participant_id": "agent-b"
              },
              {
                "action": "accept",
                "offer": null,
                "participant_id": "agent-c"
              }
            ],
            "next_proposer_id": "agent-a",
            "offer": {
              "budget": "$10,000",
              "support terms": "Email support only",
              "timeline": "12 months"
            },
            "proposer_id": "server",
            "round": 1
          },
          {
            "decisions": [
              {
                "action": "accept",
                "offer": null,
                "participant_id": "agent-a"
              },
              {
                "action": "accept",
                "offer": null,
                "participant_id": "agent-b"
              },
              {
                "action": "accept",
                "offer": null,
                "participant_id": "agent-c"
              }
            ],
            "next_proposer_id": "agent-b",
            "offer": {
              "budget": "$25,000",
              "support terms": "3 months of support post-delivery",
              "timeline": "6 months"
            },
            "proposer_id": "agent-a",
            "round": 2
          }
        ],
        "sstp_message_trace": null,
        "timedout": false
      }
    },
    "payload_hash": "0000000000000000000000000000000000000000000000000000000000000000",
    "payload_refs": [],
    "policy_labels": {
      "propagation": "restricted",
      "retention_policy": "default",
      "sensitivity": "internal"
    },
    "provenance": {
      "sources": [],
      "transforms": []
    },
    "risk_score": 0,
    "semantic_context": {
      "agents_negotiating": [
        "agent-a",
        "agent-b",
        "agent-c"
      ],
      "content_text": "Negotiate a software project contract covering timeline, budget, and support terms.",
      "encoding": "json",
      "error_message": null,
      "final_agreement": [
        {
          "chosen_option": "6 months",
          "issue_id": "timeline"
        },
        {
          "chosen_option": "$25,000",
          "issue_id": "budget"
        },
        {
          "chosen_option": "3 months of support post-delivery",
          "issue_id": "support terms"
        }
      ],
      "issues": [
        "timeline",
        "budget",
        "support terms"
      ],
      "options_per_issue": {
        "budget": [
          "$10,000",
          "$50,000",
          "$100,000",
          "$200,000"
        ],
        "support terms": [
          "Email support only",
          "Email and chat support",
          "24/7 support with dedicated representative",
          "Support for 6 months after project completion"
        ],
        "timeline": [
          "3 months",
          "6 months",
          "9 months",
          "12 months"
        ]
      },
      "outcome": "agreement",
      "schema_id": "urn:ioc:schema:negotiate:commit:v1",
      "schema_version": "1.0",
      "session_id": "sess-test-001"
    },
    "state_object_id": "sess-test-001",
    "ttl_seconds": 86400,
    "version": "0"
  },
  "shared_memory": {
    "persisted": true
  }
}
```
