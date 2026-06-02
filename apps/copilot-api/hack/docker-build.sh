#!/bin/bash
set -euo pipefail

IMAGE_NAME="ghcr.io/navikt/copilot-api"
VERSION="${VERSION:-$(date +%Y.%m.%d)-$(date +%H.%M)-$(git rev-parse --short HEAD)}"
TAG="${IMAGE_NAME}:${VERSION}"

echo "Building Docker image: ${TAG}"
docker build -t "${TAG}" -t "${IMAGE_NAME}:latest" .
echo "✓ Built ${TAG}"
