#!/usr/bin/env bash
set -e
VERSION=${VERSION:-$(git rev-parse --short HEAD)}
docker build -f Dockerfile -t ghcr.io/navikt/mcp-onboarding:$VERSION ../..
docker tag ghcr.io/navikt/mcp-onboarding:$VERSION ghcr.io/navikt/mcp-onboarding:latest
