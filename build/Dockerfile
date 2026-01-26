FROM artifactory.devhub-cloud.cisco.com/sto-cg-docker/go:v1.25.6 AS base-build

ARG DEBIAN_FRONTEND=noninteractive
RUN rm -rf /var/cache/apt/archives /var/lib/apt/lists/*

ARG CODE_COVERAGE
ARG COVER_OUT=coverage.out
ARG HTML_OUT=coverage.html
ARG STATIC_ANALYSIS
ARG SA_OUT=staticanalysis.txt
ARG SRC_DIR=/app
ARG BUILD_VERSION
ARG GO_BUILD_ENV

ENV GO111MODULE=on
ENV GOPROXY="https://proxy.golang.org, direct"

WORKDIR ${SRC_DIR}
COPY go.mod go.sum Makefile ${SRC_DIR}/
RUN go mod download
COPY . ${SRC_DIR}/

# Run gofmt checks in case the developer missed installing the hook
RUN git config --global --add safe.directory ${SRC_DIR}
RUN sh ${SRC_DIR}/.github/githooks/gofmt_checks

RUN if [ "${CODE_COVERAGE}" = "cc" ] ; \
    then go test -v ./... -coverprofile=${COVER_OUT} && \
        go tool cover -html=${COVER_OUT} -o ${HTML_OUT} ; \
    else go test -v ./... ; fi

# Find out what type of system this Docker contains.  Pull down the correct
# static analysis tool.  Unzip it.
RUN if [ "${STATIC_ANALYSIS}" = "sa" ] ; then \
      latest=$(curl -s https://api.github.com/repos/dominikh/go-tools/releases/latest) && \
      tagname=$(echo "$latest" | grep "tag_name" | cut -d':' -f2 | cut -d'"' -f2) && \
      [ "$(uname)" = Darwin ] && system=darwin || system=linux && \
      echo "tagname == $tagname system == $system"; \
    fi


# Everything looks good so far, build the code
RUN BUILD_VERSION="${BUILD_VERSION}" GO_BUILD_ENV="${GO_BUILD_ENV}" make build

# We copy the binary to the final image
FROM artifactory.devhub-cloud.cisco.com/sto-cg-docker/chainguard-base:v20230214-2026.01.15 AS run

COPY --from=base-build \
    /app/helloworld.bin \
    /usr/bin/helloworld.bin

EXPOSE 9010

ENTRYPOINT ["/usr/bin/helloworld.bin"]
