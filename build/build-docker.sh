#!/bin/bash -e
# Build ioc-cfn-svc docker image

IMAGE_NAME=${1:-ghcr.io/cisco-eti/ioc-cfn-svc:latest}

GIT_COMMIT_SHA=${GIT_COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
GIT_COMMIT_TIME=${GIT_COMMIT_TIME:-$(git log -1 --format=%cI 2>/dev/null || echo "unknown")}
GIT_BRANCH=${GIT_BRANCH:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")}

echo "Building docker image: ${IMAGE_NAME}"
echo "Git: sha=${GIT_COMMIT_SHA} time=${GIT_COMMIT_TIME} branch=${GIT_BRANCH}"

docker build \
    --platform=linux/amd64 \
    --build-arg GIT_COMMIT_SHA="${GIT_COMMIT_SHA}" \
    --build-arg GIT_COMMIT_TIME="${GIT_COMMIT_TIME}" \
    --build-arg GIT_BRANCH="${GIT_BRANCH}" \
    -t ${IMAGE_NAME} \
    -f build/Dockerfile \
    .

echo "Done! Run with: docker run -p 9002:9002 ${IMAGE_NAME}"
