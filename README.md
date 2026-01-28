# CFN Service

Go microservice with HTTP server and mock database.

## Quick Start

### Option 1: Docker (Recommended)

```bash
# Build and run
docker compose --file build/docker-compose.yaml up --build
```

### Option 2: Go directly

```bash
go run .
```

### Option 3: Build binary

```bash
make build
./cfn-svc.bin
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
| `APP_NAME` | cfn-svc | Service name |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |

## Commands

```bash
make build          # Build binary
make test           # Run tests
make docs           # Generate swagger docs
make clean          # Remove build artifacts
```

## Project Structure

```
main.go             # Entry point
pkg/
  app/              # Routes, handlers
  config/           # Configuration
  client/           # Database clients
  model/            # Data models
build/              # Docker files
deploy/             # Helm charts
```

## CI/CD

- **PR**: Builds docker image (no push)
- **Merge to main**: Builds and pushes to ECR/GHCR
