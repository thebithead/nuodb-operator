#!/usr/bin/env bash

if ! which docker > /dev/null; then
   echo "docker needs to be installed"
   exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. quay.io/<repo>/<your>-operator"}

if [ $# -gt 0 ] && [ "$1" = "DEBUG" ] ; then
  echo "building container ${IMAGE} in DEBUG Mode..."
  docker build -t "${IMAGE}" -f build/Dockerfile-dev-debug .
else
  echo "building container ${IMAGE}..."
  docker build -t "${IMAGE}" -f build/Dockerfile-dev .
fi