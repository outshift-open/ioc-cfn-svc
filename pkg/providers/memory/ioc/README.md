# Internet of Cognition (IOC) Memory Provider

This directory contains the implementation of the IOC Memory Provider, which provides a client interface for interacting with the IOC knowledge memory service.
It provides reference implementation of a test client for using the IOC Memory provider.

## Directory Structure

```
pkg/providers/memory/ioc/
├── README.md                          # documentation
├── schema.go                          # Go schema definitions (converted from Python Pydantic schema as defined by IOC knowledge memory service)
├── client.go                          # IOC memory provider implementation 
├── ioc_memory_provider_testclient/    # Test client for IOC memory provider
│   └── testclient.go
└── knowledge_memory_svc_testclient/   # Test client for proxying requests to knowledge memory service
    └── testclient.go
```

## Core Files

### `schema.go`
Contains Go struct definitions equivalent to the Python Pydantic schema defined by the IOC Knowledge Memory Service, including:

- **Request/Response Types**:
  - `KnowledgeGraphStoreRequest` / `KnowledgeGraphStoreResponse`
  - `KnowledgeGraphQueryRequest` / `KnowledgeGraphQueryResponse`
  - `KnowledgeGraphDeleteRequest` / `KnowledgeGraphDeleteResponse`

- **Data Types**:
  - `Concept` - Knowledge graph concepts with embeddings
  - `Relation` - Relationships between concepts
  - `KnowledgeGraphQueryCriteria` - Query parameters


### `client.go`
HTTP client implementation that uses the schema types:

- **Client Methods**:
  - `UpsertKnowledgeGraph()` - Store knowledge graph data
  - `QueryKnowledgeGraphPath()` - Find paths between concepts
  - `QueryKnowledgeGraphNeighbor()` - Find neighbor concepts and relations of a concept
  - `QueryKnowledgeGraphConcept()` - Query specific concept
  - `DeleteKnowledgeGraph()` - Delete knowledge graph data

- **Features**:
  - Structured logging using the project's logger
  - Request validation before HTTP calls
  - JSON pretty-printing for responses
  - Error handling with context

## Test Clients

### Configuration
Ensure you have a `.env` file with the following environment variable:
KNOWLEDGE_MEMORY_SVC_URL 
(Refer to ioc-cfn-svc/.env.sample as an example)

### IOC Memory Provider Test Client (`ioc_memory_provider_testclient/`)

**Purpose**: Demonstrates usage of a test client for the IOC memory provider client (`client.go`) with structured types defined in `schema.go`.

**Features**:
- Uses schema-defined request/response types
- Constructs requests using Go structs
- Structured logging
- JSON pretty-printing for responses

**Usage**:
```bash
cd pkg/providers/memory/ioc/ioc_memory_provider_testclient
go run testclient.go
```

### Knowledge Memory Service Test Client (`knowledge_memory_svc_testclient/`)

**Purpose**: Demonstrates usage of a test client as a proxy to the Knowledge Memory Service using raw JSON requests.

**Features**:
- Raw JSON request bodies
- Direct HTTP calls without schema validation
- Structured logging
- JSON pretty-printing for responses

**Usage**:
```bash
cd pkg/providers/memory/ioc/knowledge_memory_svc_testclient
go run testclient.go
```

## API Endpoints

Both test clients interact with these Knowledge Memory Service endpoints:

- `POST /api/knowledge/graphs` - Upsert knowledge graph data
- `POST /api/knowledge/graphs/query` - Query knowledge graph
- `DELETE /api/knowledge/graphs` - Delete knowledge graph data

## Query Types

The system supports three query types:

1. **Path Query** (`"path"`):
   - Finds paths between exactly 2 concepts 
   - Supports optional depth parameter (number of hops)
   - Supports optional boolean direction parameter (true or false)
   - Returns concepts and relations in the path

2. **Neighbor Query** (`"neighbor"`):
   - Finds neighbors of exactly 1 concept
   - Returns directly connected concepts and relations (non directional)

3. **Concept Query** (`"concept"`):
   - Retrieves details for exactly 1 concept
   - Returns concept information and metadata

4. **Concepts Filter Query** (`"concepts"`):
   - Returns concepts information and metadata

5. **Relations Filter Query** (`"relations"`):
   - Returns relations information and metadata