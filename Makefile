SHELL:=/bin/bash
PROJECT_NAME=helloworld
GO_FILES=$(shell go list ./... | grep -v /vendor/)

GO_BUILD_ENV ?= CGO_ENABLED=0
BUILD_VERSION ?= latest
CONTAINER_IMAGE ?= $(PROJECT_NAME)
CONTAINER_TAG ?= $(BUILD_VERSION)

.SILENT:

all: fmt lint vet docs test build

.PHONY: build
build:
	$(GO_BUILD_ENV) go mod verify
	$(GO_BUILD_ENV) go build -ldflags "-X main.buildVersion=${BUILD_VERSION}" -o ./$(PROJECT_NAME).bin ./cmd/helloworld/.

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
	swag init --parseDependency --parseInternal --dir ./cmd/helloworld

.PHONY: run
run:
	PORT="9010" \
		DB_HOST="localhost" \
		DB_PORT="5432" \
		DB_NAME="helloworld" \
		DB_USER="helloworld" \
		DB_PASSWORD="helloworld" \
		./$(PROJECT_NAME).bin

####################################################
##############     docker helpers     ##############
####################################################

.PHONY: docker
docker:
	BUILD_VERSION=$(BUILD_VERSION) ./build/build-docker.sh

.PHONY: test-in-docker
test-in-docker:
	BUILD_VERSION=$(BUILD_VERSION) ./build/build-docker.sh --unit-test --code-coverage --static-analysis

####################################################
############## docker compose helpers ##############
####################################################

.PHONY: dc-up
dc-up:
	docker compose --file build/docker-compose.yaml up

.PHONY: dc-stop
dc-stop:
	docker compose --file build/docker-compose.yaml stop

.PHONY: dc-down
dc-down:
	docker compose --file build/docker-compose.yaml down --volumes --rmi local

.PHONY: exec-db
exec-db:
	docker compose exec helloworld-db psql -U helloworld -d helloworld
