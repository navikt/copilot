#!/usr/bin/env bash
failed=()

for app in $APPS; do
  echo "📦 $app:"
  if (cd "apps/$app" && mise run update); then
    echo ""
  else
    failed+=("$app")
    echo ""
  fi
done

echo "📦 cli/nav-pilot:"
if (cd cli/nav-pilot && go get -u ./... && go mod tidy); then
  echo ""
else
  failed+=("cli/nav-pilot")
  echo ""
fi

echo "📦 scripts/generate-docs:"
if (cd scripts/generate-docs && go get -u ./... && go mod tidy); then
  echo ""
else
  failed+=("generate-docs")
  echo ""
fi

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "❌ Update failed for: ${failed[*]}"
  exit 1
fi
echo "✅ All dependencies updated"
