#!/usr/bin/env bash

#This script will build and push image for further testing.

echo "Building operator image and pushing"

PROJECT_NAME="nuodb-operator"

CGO_ENABLED=0 
GOOS=linux 
GOARCH=amd64 
go build \
	-o $TRAVIS_BUILD_DIR/build/_output/bin/${PROJECT_NAME} $TRAVIS_BUILD_DIR/cmd/manager/main.go

cd $TRAVIS_BUILD_DIR/

docker version
echo "Docker login..."
docker login -u $BOT_U -p $BOT_P $DOCKER_SERVER

echo "Build NuoDB Operator..."
echo "Build image tag $NUODB_OP_IMAGE"
operator-sdk build $NUODB_OP_IMAGE
docker push $NUODB_OP_IMAGE
