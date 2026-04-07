#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Usage: $0 <version>}"
IMAGE="onefivefour/echolist-backend"

echo "Loggin into docker"
docker login

echo "Building ${IMAGE}:${VERSION} ..."
docker build -t "${IMAGE}:${VERSION}" -t "${IMAGE}:latest" .

echo "Pushing ${IMAGE}:${VERSION} ..."
docker push "${IMAGE}:${VERSION}"
docker push "${IMAGE}:latest"

echo "Done – pushed ${IMAGE}:${VERSION} and ${IMAGE}:latest"
