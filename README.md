# IOC CFN Service

Go microservice with HTTP server and mock database.

**Docker Image:** `ghcr.io/cisco-eti/ioc-cfn-svc:latest`

## Quick Start

### Option 1: Docker (Recommended)

```bash
# Run with pre-built image from GHCR (no local build)
docker compose --file build/docker-compose.yaml up

# Or build and run locally
docker compose --file build/docker-compose.yaml up --build

# Or run directly without compose
docker run -p 9010:9010 ghcr.io/cisco-eti/ioc-cfn-svc:latest
```

### Option 2: Go directly

```bash
go run .
```

### Option 3: Build binary

```bash
make build
./ioc-cfn-svc.bin
```

App runs on **http://localhost:9010**

## API Endpoints

```bash
# Health check
curl http://localhost:9010/healthz

# Readiness check
curl http://localhost:9010/ready

# CFN dummy API
curl http://localhost:9010/api/v1/cfn/dummy

# Create foo
curl -X POST http://localhost:9010/api/v1/foo \
  -H "Content-Type: application/json" \
  -d '{"uuid":"123e4567-e89b-12d3-a456-426614174000","name":"test","email":"test@example.com"}'

# Get foo
curl http://localhost:9010/api/v1/foo/123e4567-e89b-12d3-a456-426614174000
```

## Configuration

Environment variables (uppercase):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 9010 | App server port |
| `APP_NAME` | ioc-cfn-svc | Service name |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |

## Commands

```bash
make build          # Build binary
make test           # Run tests
make docker         # Build docker image
make docs           # Generate swagger docs
make clean          # Remove build artifacts
```

## Project Structure

```
main.go             # Entry point
pkg/
  app/              # Routes, handlers
  audit/            # Audit logging
  client/           # Database clients
  config/           # Configuration
  mapper/           # Data mappers
  metric/           # Prometheus metrics
  model/            # Data models
  task/             # Task management
  tools/            # Utilities (logger, http)
build/              # Dockerfile, docker-compose, scripts
deploy/             # Helm charts
docs/               # Swagger docs
```

## CI/CD

- **PR**: Builds `ghcr.io/cisco-eti/ioc-cfn-svc:latest` (no push)
- **Merge to main**: Builds and pushes to GHCR/ECR
