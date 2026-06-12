# Public API Contracts

Versioned OpenAPI specifications for SDK generation and API documentation.

**Current version:** `public-api-v1.1.yaml`

## Quick Start

### 1. Pull Docker Image

```bash
docker pull openapitools/openapi-generator-cli
```

### 2. Generate SDK

**Python:**
```bash
docker run --rm \
  -v "${PWD}:/local" \
  openapitools/openapi-generator-cli generate \
  -i /local/docs/public-api/public-api-v1.0.yaml \
  -g python \
  -o /local/sdk/python/v1.0 \
  --package-name cfn_service_client
```

**TypeScript:**
```bash
docker run --rm \
  -v "${PWD}:/local" \
  openapitools/openapi-generator-cli generate \
  -i /local/docs/public-api/public-api-v1.0.yaml \
  -g typescript-axios \
  -o /local/sdk/typescript/v1.0 \
  --additional-properties=npmName=cfn-service-client,npmVersion=1.0.0
```

**Go:**
```bash
docker run --rm \
  -v "${PWD}:/local" \
  openapitools/openapi-generator-cli generate \
  -i /local/docs/public-api/public-api-v1.0.yaml \
  -g go \
  -o /local/sdk/go/v1.0 \
  --additional-properties=packageName=cfnclient
```

### 3. Use the SDK

**Python example:**
```python
import cfn_service_client
from cfn_service_client.models import QueryRequest, Header

config = cfn_service_client.Configuration(host="http://localhost:9002")

with cfn_service_client.ApiClient(config) as api_client:
    api = cfn_service_client.SharedMemoriesApi(api_client)
    
    response = api.fetch_shared_memories(
        workspace_id="ws-001",
        mas_id="mas-001",
        query_request=QueryRequest(
            header=Header(agent_id="agent-1"),
            intent="Find authentication concepts"
        )
    )
    print(response.message)
```

See [example_python_usage.py](example_python_usage.py) for complete examples.

## Naming Conventions

Generated SDKs follow language-specific conventions:

**Python** (PEP 8):
- Package: `cfn_service_client`
- Classes: `CreateOrUpdateRequest`, `SharedMemoriesApi`
- Methods: `fetch_shared_memories()`, `create_or_update_shared_memories()`
- Fields: `request_id`, `workspace_id`, `agent_id`

**TypeScript/Go**: Follow respective language conventions automatically.

## Publishing a New SDK Version

### 1. Update the Spec

**Option A: Ask AI**
```
"Update public API spec to v1.1 based on current handlers in pkg/app/"
```

**Option B: Manual**
```bash
cp public-api-v1.0.yaml public-api-v1.1.yaml
# Edit the file to match current implementation
```

### 2. Review Changes

```bash
diff public-api-v1.0.yaml public-api-v1.1.yaml
```

### 3. Generate SDKs

Run the same Docker commands with updated version:
```bash
docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
  -i /local/docs/public-api/public-api-v1.1.yaml \
  -g python \
  -o /local/sdk/python/v1.1 \
  --package-name cfn_service_client
```

### 4. Tag the Release

```bash
git add docs/public-api/public-api-v1.1.yaml
git commit -m "Release public API v1.1"
git tag api-v1.1.0
git push origin api-v1.1.0
```

## Versioning

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR (v2.0)**: Breaking changes (remove endpoint, rename field, change types)
- **MINOR (v1.1)**: Backward-compatible additions (new endpoint, optional field)
- **PATCH (v1.0.1)**: Documentation updates only

## File Naming

```
public-api-v{MAJOR}.{MINOR}.yaml
```

Examples:
- `public-api-v1.0.yaml` - Initial release
- `public-api-v1.1.yaml` - Backward-compatible changes
- `public-api-v2.0.yaml` - Breaking changes

## Important Notes
- Internal endpoints (`/api/internal/*`) are not included

**Before publishing:**
- ✅ Test the spec matches actual API behavior
- ✅ Verify all public endpoints are documented
- ✅ Test SDK generation works

