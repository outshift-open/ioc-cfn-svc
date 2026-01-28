# CFN Service

Go microservice template with HTTP server, database access, and CI/CD setup.

## Quick Start - Run Locally

### Option 1: Without Database (Simplest)

```bash
# Build and run
make build
./cfn-svc.bin
```

The app runs on **http://localhost:9010** with a mock database.

### Option 2: With PostgreSQL Database

```bash
# 1. Start PostgreSQL via Docker Compose
make dc-up

# 2. In another terminal, build and run the app
make build
make run
```

### Option 3: Run with Go directly

```bash
# Without database
go run .

# With database (start postgres first with `make dc-up`)
DB_HOST=localhost DB_PORT=5432 DB_NAME=cfn-svc DB_USER=cfn-svc DB_PASSWORD=cfn-svc go run .
```

## Verify It's Working

```bash
# Health check
curl http://localhost:9010/healthz

# Create a foo
curl -X POST http://localhost:9010/api/v1/foo \
  -H "Content-Type: application/json" \
  -d '{"uuid":"123e4567-e89b-12d3-a456-426614174000","name":"test","email":"test@example.com"}'

# Get a foo
curl http://localhost:9010/api/v1/foo/123e4567-e89b-12d3-a456-426614174000
```

## Configuration

All config via environment variables or flags:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 9010 | App server port |
| `DB_HOST` | - | PostgreSQL host (empty = mock DB) |
| `DB_PORT` | - | PostgreSQL port |
| `DB_NAME` | - | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |

## Common Commands

```bash
make build          # Build binary
make run            # Run with default DB config
make test           # Run tests
make lint           # Run linter
make dc-up          # Start PostgreSQL
make dc-down        # Stop and cleanup PostgreSQL
make clean          # Remove build artifacts
```

## Project Structure

```
main.go             # Entry point
pkg/
  app/              # Application logic, routes, handlers
  config/           # Configuration management
  client/           # Database and external service clients
  model/            # Data models
  tools/            # Utilities (logger, http helpers)
build/              # Docker and compose files
deploy/             # Helm charts and k8s configs
```

## Using as a Template

1. Create new repo from this template
2. Run `./runme.sh` to rename the service
3. Request CI/CD pipeline from SRE team

## Resources

- [Troubleshooting](docs/troubleshooting.md)
- [Go Style Guide](https://google.github.io/styleguide/go/best-practices)
- [Effective Go](https://go.dev/doc/effective_go)
