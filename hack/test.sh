#!/usr/bin/env bash
set -e
for app in $APPS; do
  echo "📦 $app:" && (cd "apps/$app" && mise run test) && echo ""
done
echo "🧭 nav-pilot:" && mise run nav-pilot:test && echo ""
echo "✅ All tests passed"
