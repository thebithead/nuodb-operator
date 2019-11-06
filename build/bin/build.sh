#!/usr/bin/env bash
# Requires GoLang Delve debugger installed in $GOPATH/bin/dlv

set -o errexit
set -o nounset
set -o pipefail

if ! which go > /dev/null; then
   echo "golang needs to be installed"
   exit 1
fi

BIN_DIR="$(pwd)/tmp/_output/bin"
mkdir -p ${BIN_DIR}
PROJECT_NAME="nuodb-operator"
REPO_PATH="~/gopath/src/nuodb/nuodb-golang-operator"
#BUILD_PATH="${REPO_PATH}/cmd/manager"
BUILD_PATH="nuodb/nuodb-operator/cmd/manager"

# from `operator-sdk build --verbose quay.io/nuodb/nuodb-golang-operator-dev`
#go build -o "/home/tgates/gopath/src/nuodb/nuodb-golang-operator/build/_output/bin/nuodb-golang-operator" -gcflags "all=-trimpath=/home/tgates/gopath/src/nuodb" -asmflags "all=-trimpath=/home/tgates/gopath/src/nuodb" "nuodb/nuodb-operator/cmd/manager"
if [ $# -gt 0 ] && [ "$1" = "DEBUG" ] ; then
  echo "building "${PROJECT_NAME}" In DEBUG Mode..."
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -gcflags "all=-N -l -trimpath=/home/tgates/gopath/src/nuodb" -asmflags "all=-trimpath=/home/tgates/gopath/src/nuodb" -o ${BIN_DIR}/${PROJECT_NAME}-dev-debug $BUILD_PATH
  cp ${GOPATH}/bin/dlv ${BIN_DIR}
else
  echo "building "${PROJECT_NAME}"..."
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -gcflags "all=-trimpath=/home/tgates/gopath/src/nuodb" -asmflags "all=-trimpath=/home/tgates/gopath/src/nuodb" -o ${BIN_DIR}/${PROJECT_NAME}-dev $BUILD_PATH
fi
