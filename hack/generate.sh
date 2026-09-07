#!/usr/bin/env bash
# Generate files for all apps. Tracks failures per app so one broken app
# doesn't block the rest.
failed=()

for app in $APPS_WITH_GENERATE; do
  echo "📦 $app:"
  if (cd "apps/$app" && mise run generate); then
    echo ""
  else
    failed+=("$app")
    echo ""
  fi
done

echo "🗑  retired:"
if mise run retired:generate; then
  echo ""
else
  failed+=("retired")
  echo ""
fi

echo "📄 docs:"
if mise run docs:generate; then
  echo ""
else
  failed+=("docs")
  echo ""
fi

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "⚠️  Generate failed for: ${failed[*]}"
  exit 1
fi
echo "✅ All files generated"
