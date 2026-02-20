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

| Step | Endpoint             | Method                 | Description                                      |
| ---- | -------------------- | ---------------------- | ------------------------------------------------ |
| 1    | `POST /api/_otel`    | `SendOtelSpans`        | Ingest OTel spans → extract concepts & relations  |
| 2    | `POST /api/_general` | `SendGeneral`          | General cognition using request ID from step 1    |
| 3    | `POST /api/_reasoner`| `SendReasonerEvidence` | Natural-language intent → reasoner evidence       |

---

## Quick Start

```go
import "github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitiveagentclient"

client := cognitiveagentclient.New("http://localhost:8000", 30*time.Second)

resp, err := client.SendOtelSpans(ctx, spans)          // step 1
genResp, err := client.SendGeneral(ctx, requests)       // step 2
resResp, err := client.SendReasonerEvidence(ctx, req)   // step 3
```

## Smoke Test

```go
cognitiveagentclient.RunTestClient("http://localhost:8000", "all")       // all endpoints
cognitiveagentclient.RunTestClient("http://localhost:8000", "otel")      // single endpoint
cognitiveagentclient.RunTestClient("http://localhost:8000", "general")
cognitiveagentclient.RunTestClient("http://localhost:8000", "reasoner")
```

---

## Files

| File                      | Description                                    |
| ------------------------- | ---------------------------------------------- |
| `cognitiveagentclient.go` | Client, request/response types, and API methods |
| `testclient.go`           | Smoke-test helper with sample data              |
| `README.md`               | This document                                   |

## TODO

- [ ] Define `GeneralCognitionResponse` fields once API schema is finalized.
- [ ] Define `ReasonerCognitionResponse` fields once API schema is finalized.
- [ ] Update `ReasonerRequest.AdditionalContext` type once API clarifies the schema.
- [ ] Add audit CRUD operations for cognitive agent API calls.
