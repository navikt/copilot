#!/usr/bin/env bash
# Generates to a temp file: the manifest is //go:embed'ed, and mise runs this in
# parallel with lint/vet/test, which must never see a partial write.
set -euo pipefail

MANIFEST="internal/discovery/copilot-manifest.json"
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

go run ./cmd/generate-manifest -output "$TMP"

if diff -u "$MANIFEST" "$TMP"; then
  echo "✅ Manifest is up to date"
else
  echo "❌ Manifest is out of date. Run 'mise generate' to update it."
  exit 1
fi
