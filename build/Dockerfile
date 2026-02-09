# todo: use SRE base image
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG BUILDPLATFORM
ARG TARGETARCH
ARG GIT_COMMIT_SHA=unknown
ARG GIT_COMMIT_TIME=unknown
ARG GIT_BRANCH=unknown

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

WORKDIR /app

COPY --from=builder /app/cfn-svc .

EXPOSE 9010

ENTRYPOINT ["./cfn-svc"]
