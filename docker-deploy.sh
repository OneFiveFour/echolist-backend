#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Usage: $0 <version>}"
IMAGE="onefivefour/echolist-backend"

echo "Loggin into docker"
docker login

echo "Generating protobuf code (proto/gen is not checked in) ..."
(cd proto && buf generate)

# The production server is linux/amd64. Build for it explicitly so the image is
# correct even when building on Apple Silicon (override with PLATFORM=...).
PLATFORM="${PLATFORM:-linux/amd64}"

echo "Building and pushing ${IMAGE}:${VERSION} and :latest for ${PLATFORM} ..."
docker buildx build --platform "${PLATFORM}" \
  -t "${IMAGE}:${VERSION}" -t "${IMAGE}:latest" \
  --push .

echo "Done – pushed ${IMAGE}:${VERSION} and ${IMAGE}:latest"
