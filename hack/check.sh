#!/usr/bin/env bash
# Run checks for all apps. Tracks failures per section so one broken app
# doesn't block the rest.
failed=()

for app in $APPS; do
  echo "📦 $app:"
  if (cd "apps/$app" && mise run check); then
    echo ""
  else
    failed+=("$app")
    echo ""
  fi
done

echo "📄 docs:"
if mise run docs:check; then
  echo ""
else
  failed+=("docs")
  echo ""
fi

echo "🔧 skills:"
if mise run skills:lint -- -q; then
  echo ""
else
  failed+=("skills")
  echo ""
fi

echo "🔗 skills sync:"
if mise run skills:sync-check -- -q; then
  echo ""
else
  failed+=("skills-sync")
  echo ""
fi

echo "📦 collections:"
if mise run collections:lint -- -q; then
  echo ""
else
  failed+=("collections")
  echo ""
fi

echo "🧭 nav-pilot:"
if mise run nav-pilot:check; then
  echo ""
else
  failed+=("nav-pilot")
  echo ""
fi

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "❌ Checks failed for: ${failed[*]}"
  exit 1
fi
echo "✅ All checks passed"
