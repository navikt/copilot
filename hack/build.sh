#!/usr/bin/env bash
# shellcheck source=hack/parallel.sh
source "$(dirname "$0")/parallel.sh"

for app in $APPS; do
  section "$app" mise -C "apps/$app" run build
done
section nav-pilot mise run nav-pilot:build
wait_sections

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "❌ Build failed for: ${failed[*]}"
  exit 1
fi
echo "✅ All apps built successfully"
