# Cognitive Agent Client

Go client for the **Cognitive Agents API** with built-in retries and exponential backoff.

> **Note:** API endpoint paths and Go struct fields/JSON tags in this package may change
> as the upstream Cognitive Agents API evolves. Update structs and paths when the API
> contract is modified.

---

## Overview

```
Your App  ──▶  cognitiveagentclient.Client  ──▶  httpclient.Client  ──▶  Cognitive Agents Service
```

| Endpoint                                        | Method                  | Description                                           |
| ----------------------------------------------- | ----------------------- | ----------------------------------------------------- |
| `POST /api/knowledge-mgmt/extraction`           | `SendExtraction`        | Ingest agent telemetry → extract concepts & relations  |
| `POST /api/knowledge-mgmt/reasoning/evidence`   | `SendReasoningEvidence` | Reasoning evidence request with an intent query        |
| `POST /api/semantic-negotiation`                | `SendSemanticNegotiation` | Semantic negotiation request (TBD)                   |

---

## Quick Start

```go
import "github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitiveagentclient"

client := cognitiveagentclient.New("http://localhost:8000", 30*time.Second)

resp, err := client.SendExtraction(ctx, &cognitiveagentclient.ExtractionRequest{
    Header: cognitiveagentclient.Header{
        WorkspaceID: "ws-001",
        MASID:       "mas-001",
    },
    Payload: cognitiveagentclient.ExtractionPayload{
        Metadata: cognitiveagentclient.ExtractionPayloadMetadata{Format: "observe-sdk-otel"},
        Data:     records,
    },
})
```

## Smoke Test

```go
cognitiveagentclient.RunTestClient("http://localhost:8000")
```

---

## Files

| File                      | Description                                    |
| ------------------------- | ---------------------------------------------- |
| `cognitiveagentclient.go` | Client, request/response types, and API methods |
| `testclient.go`           | Smoke-test helper with sample data              |
| `README.md`               | This document                                   |

## TODO

- [ ] Add audit CRUD operations for cognitive agent API calls.
