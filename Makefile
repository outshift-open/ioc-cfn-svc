SHELL:=/bin/bash
PROJECT_NAME=ioc-cfn-svc
GO_FILES=$(shell go list ./... | grep -v /vendor/)

GO_BUILD_ENV ?= CGO_ENABLED=0
BUILD_VERSION ?= latest
CONTAINER_IMAGE ?= ghcr.io/cisco-eti/$(PROJECT_NAME)
CONTAINER_TAG ?= $(BUILD_VERSION)

GIT_COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_TIME ?= $(shell git log -1 --format=%cI 2>/dev/null || echo "unknown")
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

.SILENT:

all: fmt lint vet docs test build

.PHONY: build
build:
	$(GO_BUILD_ENV) go mod verify
	$(GO_BUILD_ENV) go build -ldflags "-X main.buildVersion=${BUILD_VERSION} -X main.gitCommitSHA=${GIT_COMMIT_SHA} -X main.gitCommitTime=${GIT_COMMIT_TIME} -X main.gitBranch=${GIT_BRANCH}" -o ./$(PROJECT_NAME).bin .

vendor:
	go mod vendor

vet:
	go vet $(GO_FILES)

fmt:
	go fmt $(GO_FILES)

.PHONY: test
test:
	go test --cover -count=1 $(GO_FILES)

sonar: test
	sonar-scanner -Dsonar.projectVersion="$(version)"

integration-test:
	go test -tags=integration $(GO_FILES)

.PHONY: clean
clean:
	rm -rf coverage coverage.out coverage.html staticanalysis.txt
	rm -f $(PROJECT_NAME).bin
	rm -rf vendor
	#go clean -modcache

.PHONY: cover
cover:
	go test -coverprofile=coverage.out $(GO_FILES); go tool cover -html=coverage.html

.PHONY: lint
lint:
	swag fmt
	go fmt ./...
	golangci-lint run -v

.PHONY: docs
docs:
	swag init --parseDependency --parseInternal --dir .

.PHONY: run
run: ## Run in HTTP mode (default)
	PORT="9010" \
		DB_HOST=$${DB_HOST:-localhost} \
		DB_PORT=$${DB_PORT:-5432} \
		DB_NAME=$${DB_NAME:-cfn-svc} \
		DB_USER=$${DB_USER:-cfn-svc} \
		DB_PASSWORD=$${DB_PASSWORD} \
		./$(PROJECT_NAME).bin

.PHONY: run-mcp
run-mcp: ## Run in MCP mode
	MCP_ENABLED=true MCP_PORT=9010 ./$(PROJECT_NAME).bin

.PHONY: dev
dev: ## Run with go run (picks up .env file, injects git info)
	$(GO_BUILD_ENV) go run -ldflags "-X main.buildVersion=${BUILD_VERSION} -X main.gitCommitSHA=${GIT_COMMIT_SHA} -X main.gitCommitTime=${GIT_COMMIT_TIME} -X main.gitBranch=${GIT_BRANCH}" .

####################################################
##############     docker helpers     ##############
####################################################

.PHONY: docker
docker:
	GIT_COMMIT_SHA=$(GIT_COMMIT_SHA) GIT_COMMIT_TIME=$(GIT_COMMIT_TIME) GIT_BRANCH=$(GIT_BRANCH) BUILD_VERSION=$(BUILD_VERSION) ./build/build-docker.sh

.PHONY: test-in-docker
test-in-docker:
	GIT_COMMIT_SHA=$(GIT_COMMIT_SHA) GIT_COMMIT_TIME=$(GIT_COMMIT_TIME) GIT_BRANCH=$(GIT_BRANCH) BUILD_VERSION=$(BUILD_VERSION) ./build/build-docker.sh --unit-test --code-coverage --static-analysis

####################################################
############## docker compose helpers ##############
####################################################

.PHONY: dc-up
dc-up: ## Run in HTTP mode (default)
	GIT_COMMIT_SHA=$(GIT_COMMIT_SHA) GIT_COMMIT_TIME=$(GIT_COMMIT_TIME) GIT_BRANCH=$(GIT_BRANCH) docker compose --file build/docker-compose.yaml up

.PHONY: dc-up-mcp
dc-up-mcp: ## Run in MCP mode
	GIT_COMMIT_SHA=$(GIT_COMMIT_SHA) GIT_COMMIT_TIME=$(GIT_COMMIT_TIME) GIT_BRANCH=$(GIT_BRANCH) MCP_ENABLED=true docker compose --file build/docker-compose.yaml up

.PHONY: dc-up-build
dc-up-build: ## Build and run
	GIT_COMMIT_SHA=$(GIT_COMMIT_SHA) GIT_COMMIT_TIME=$(GIT_COMMIT_TIME) GIT_BRANCH=$(GIT_BRANCH) docker compose --file build/docker-compose.yaml up --build

.PHONY: dc-stop
dc-stop: ## Stop containers
	docker compose --file build/docker-compose.yaml stop

.PHONY: dc-down
dc-down: ## Stop and remove containers
	docker compose --file build/docker-compose.yaml down --volumes --rmi local

.PHONY: exec-db
exec-db:
	docker compose exec cfn-svc-db psql -U cfn-svc -d cfn-svc
