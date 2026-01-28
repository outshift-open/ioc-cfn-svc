FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o cfn-svc .

# Final minimal image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/cfn-svc .

EXPOSE 9010

ENTRYPOINT ["./cfn-svc"]
