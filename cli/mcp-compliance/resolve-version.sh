#!/usr/bin/env bash
# Prints the release tag to install. Usage: resolve-version.sh <version input> <github.action_ref>
# An empty input means the Action's own release, so pinning the Action ref pins the binary.
set -euo pipefail
version=${1:-}
ref=${2:-}
if [ -z "$version" ]; then
  if [[ "$ref" =~ ^cli/mcp-compliance/v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    version=${ref#cli/mcp-compliance/}
  else
    echo "::notice::Action ref '${ref}' is not a cli/mcp-compliance release tag; using the latest release" >&2
    version=latest
  fi
fi
echo "$version"
