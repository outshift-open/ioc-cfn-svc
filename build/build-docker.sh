#!/bin/bash -e
# Syntax build-docker.sh [-i|--image imagename]

PROJECT=sre-go-helloworld
DOCKER_IMAGE=${PROJECT}:latest
BASE_DOCKER_IMAGE=${PROJECT}:base
H_OUT=coverage.html
S_OUT=staticanalysis.txt

# TODO: is this necessary now that the project has been moved out of
# wwwin-github.cisco.com to github.com?
get_artifactory_credentials() {
  if [[ "${ARTIFACTORY_USER}" != "" ]] && [[ "${ARTIFACTORY_PASSWORD}" != "" ]]; then
    return
  fi

  if [[ "${NYOTA_CREDENTIALS_FILE}" = "" ]]; then
    NYOTA_CREDENTIALS_FILE=~/.nyota/credentials
    echo "Using DEFAULT Artifactory credentials file: $NYOTA_CREDENTIALS_FILE"
  else
    echo "Using Artifactory credentials file: $NYOTA_CREDENTIALS_FILE"
  fi

  [[ "$(uname)" = Darwin ]] && NYOTA_CREDENTIALS_FILE_MOD=$(stat -f "%p" "${NYOTA_CREDENTIALS_FILE}" | cut -c4-) || NYOTA_CREDENTIALS_FILE_MOD=$(stat -c "%a" "${NYOTA_CREDENTIALS_FILE}")
  if [[ ${NYOTA_CREDENTIALS_FILE_MOD} != "400" ]]; then
    echo "File ${NYOTA_CREDENTIALS_FILE} must have 400 mod permission"
    exit 1
  fi

  if [[ "$NYOTA_CREDENTIALS_SECTION" == "" ]]; then
    NYOTA_CREDENTIALS_SECTION=default
  fi

  while IFS=' = ' read key value; do
    if [[ ${key} == \[*] ]]; then
      section=${key}
    elif [[ ${value} ]] && [[ ${section} == "[${NYOTA_CREDENTIALS_SECTION}]" ]]; then
      if [[ ${key} == 'artifactory_user' ]]; then
        ARTIFACTORY_USER=${value}
      elif [[ ${key} == 'artifactory_password' ]]; then
        ARTIFACTORY_PASSWORD=${value}
      fi
    fi
  done <${NYOTA_CREDENTIALS_FILE}
}

code_coverage() {
    # extract the H_OUT file from the docker image created
    id=$(docker create --platform=linux/amd64 ${BASE_DOCKER_IMAGE})
    docker cp ${id}:/app/${H_OUT} .
    docker cp ${id}:/app/coverage.out .
    docker rm -v ${id}
    if [[ ! -d "pipeline/lib" ]] ; then
        echo "Your coverage HTML report is in $H_OUT"
    fi
}

static_analysis() {
    # extract the S_OUT file from the docker image created
    id=$(docker create --platform=linux/amd64 ${BASE_DOCKER_IMAGE})
    docker cp ${id}:/app/${S_OUT} .
    docker rm -v ${id}
    if [[ ! -d "pipeline/lib" ]] ; then
        echo "Your static analysis report is in $S_OUT"
    fi
}

while [[ $# -gt 0 ]]
do
    key="${1}"

    case ${key} in
    -i|--image)
        DOCKER_IMAGE="${2}"
        shift;shift
        ;;
    -h|--help)
        less README.md
        exit 0
        ;;
    -u|--unit-test)
        UNIT_TEST=ut
        shift
        ;;
    -c|--code-coverage)
        CODE_COVERAGE=cc
        shift
        ;;
    -s|--static-analysis)
        STATIC_ANALYSIS=sa
        shift
        ;;
    *) # unknown
        echo Unknown Parameter $1
        exit 4
    esac
done

#get_artifactory_credentials
echo BUILDING DOCKER ${BASE_DOCKER_IMAGE}

export GO111MODULE=on
export GOPROXY="https://proxy.golang.org, direct"

docker build \
    --platform=linux/amd64 \
    -t ${BASE_DOCKER_IMAGE} \
    -f build/Dockerfile \
    --build-arg HTML_OUT=${H_OUT} \
    --build-arg CODE_COVERAGE=${CODE_COVERAGE} \
    --build-arg STATIC_ANALYSIS=${STATIC_ANALYSIS} \
    --build-arg SA_OUT=${S_OUT} \
    --build-arg BUILD_VERSION=${BUILD_VERSION} \
    --build-arg GO_BUILD_ENV="CGO_ENABLED=0 GOOS=linux GOARCH=amd64" \
    .

if [[ "${UNIT_TEST}" = "ut" ]] ; then
    echo "Running unit tests in docker"
    docker run --platform=linux/amd64 --rm ${BASE_DOCKER_IMAGE} make test
fi

if [[ "${CODE_COVERAGE}" = "cc" ]] ; then
    echo "Generating code coverage"
    code_coverage
fi

if [[ "${STATIC_ANALYSIS}" = "sa" ]] ; then
    echo "Generating static analysis"
    static_analysis
fi

echo BUILDING DOCKER ${DOCKER_IMAGE}

docker build \
    --platform=linux/amd64 \
    --no-cache \
    -t ${DOCKER_IMAGE} \
    -f build/Dockerfile \
    .
