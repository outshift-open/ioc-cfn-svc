FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG GIT_COMMIT_SHA=unknown
ARG GIT_COMMIT_TIME=unknown
ARG GIT_BRANCH=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags="-X main.GitCommitSHA=${GIT_COMMIT_SHA} -X main.GitCommitTime=${GIT_COMMIT_TIME} -X main.GitBranch=${GIT_BRANCH}" \
    -o ioc-cfn-svc .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /app/ioc-cfn-svc .

EXPOSE 9002

CMD ["./ioc-cfn-svc"]
