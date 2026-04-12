#!/usr/bin/env bash
set -e
for app in $APPS; do
  echo "📦 $app:" && (cd "apps/$app" && mise run build) && echo ""
done
echo "🧭 nav-pilot:" && mise run nav-pilot:build && echo ""
echo "✅ All apps built successfully"
