#!/usr/bin/env bash
set -e
for app in $APPS; do
  echo "📦 $app:" && (cd "apps/$app" && mise run check) && echo ""
done
echo "📄 docs:" && mise run docs:check && echo ""
echo "🔧 skills:" && mise run skills:lint -- -q && echo ""
echo "🧭 nav-pilot:" && mise run nav-pilot:check && echo ""
echo "✅ All checks passed"
