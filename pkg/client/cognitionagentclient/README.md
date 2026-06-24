# cognition agent Client

Go client for the **cognition agents API** with built-in retries and exponential backoff.

> **Note:** API endpoint paths and Go struct fields/JSON tags in this package may change
> as the upstream cognition agents API evolves. Update structs and paths when the API
> contract is modified.

---

## Overview

```
Your App  ──▶  cognitionagentclient.Client  ──▶  httpclient.Client  ──▶  cognition agents Service
```

| Endpoint                                        | Method                  | Description                                           |
| ----------------------------------------------- | ----------------------- | ----------------------------------------------------- |
| `POST /api/knowledge-mgmt/extraction`           | `SendExtraction`        | Ingest agent telemetry → extract concepts & relations  |
| `POST /api/knowledge-mgmt/reasoning/evidence`   | `SendReasoningEvidence` | Reasoning evidence request with an intent query        |
| `POST /api/semantic-alignment`                  | `SendSemanticAlignment` | Semantic alignment request (TBD)                     |

---

## Quick Start

```go
import "github.com/outshift-open/ioc-cfn-svc/pkg/client/cognitionagentclient"

client := cognitionagentclient.New("http://localhost:8000", 30*time.Second)

resp, err := client.SendExtraction(ctx, &cognitionagentclient.ExtractionRequest{
    Header: cognitionagentclient.Header{
        WorkspaceID: "ws-001",
        MASID:       "mas-001",
    },
    Payload: cognitionagentclient.ExtractionPayload{
        Metadata: cognitionagentclient.ExtractionPayloadMetadata{Format: "observe-sdk-otel"},
        Data:     records,
    },
})
```

## Smoke Test

```go
cognitionagentclient.RunTestClient("http://localhost:8000")
```

---

## Files

| File                      | Description                                    |
|---------------------------| ---------------------------------------------- |
| `cognitionagentclient.go` | Client, request/response types, and API methods |
| `testclient.go`           | Smoke-test helper with sample data              |
| `README.md`               | This document                                   |

## TODO

- [ ] Add audit CRUD operations for cognition agent API calls.
