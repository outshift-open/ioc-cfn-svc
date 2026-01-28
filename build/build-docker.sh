#!/bin/bash -e
# Build cfn-svc docker image

IMAGE_NAME=${1:-cfn-svc:latest}

echo "Building docker image: ${IMAGE_NAME}"

docker build \
    --platform=linux/amd64 \
    -t ${IMAGE_NAME} \
    -f build/Dockerfile \
    .

echo "Done! Run with: docker run -p 9010:9010 ${IMAGE_NAME}"
