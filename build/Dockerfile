# todo: use SRE base image
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG BUILDPLATFORM
ARG TARGETARCH
ARG GIT_COMMIT_SHA=unknown
ARG GIT_COMMIT_TIME=unknown
ARG GIT_BRANCH=unknown

ENV LOG_ENCODING=json
ENV LOG_LEVEL=info
ENV SERVICE_NAME=ioc-cfn-svc
ENV APPLICATION_VERSION=0.1.0

WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary using Go's native cross-compilation (no QEMU needed)
RUN echo "Build platform: $BUILDPLATFORM -> Target: $TARGETARCH" && \
    echo "Git: sha=$GIT_COMMIT_SHA time=$GIT_COMMIT_TIME branch=$GIT_BRANCH" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
      -ldflags "-X main.gitCommitSHA=$GIT_COMMIT_SHA -X main.gitCommitTime=$GIT_COMMIT_TIME -X main.gitBranch=$GIT_BRANCH" \
      -o cfn-svc .

# Final minimal image
FROM alpine:3.21

RUN apk add --no-cache curl

WORKDIR /app

COPY --from=builder /app/cfn-svc .
COPY --from=builder /app/docs/swagger.json ./docs/swagger.json
COPY --from=builder /app/docs/swagger-internal.json ./docs/swagger-internal.json

EXPOSE 9002

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:9002/api/internal/diagnostics/health || exit 1

ENTRYPOINT ["./cfn-svc"]
