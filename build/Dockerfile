# todo: use SRE base image
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG BUILDPLATFORM
ARG TARGETARCH

WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary using Go's native cross-compilation (no QEMU needed)
RUN echo "Build platform: $BUILDPLATFORM -> Target: $TARGETARCH" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o cfn-svc .

# Final minimal image
FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/cfn-svc .

EXPOSE 9010

ENTRYPOINT ["./cfn-svc"]
