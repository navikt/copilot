#!/usr/bin/env bash
# Run checks for all apps side by side. Tracks failures per section so one
# broken app doesn't block the rest.
# shellcheck source=hack/parallel.sh
source "$(dirname "$0")/parallel.sh"

for app in $APPS; do
  section "$app" mise -C "apps/$app" run check
done
section retired mise run retired:check
section docs mise run docs:check
section skills mise run skills:lint -- -q
section hooks mise run hooks:test
section pricing mise run pricing:test ::: pricing:check
section models mise run models:test ::: models:check
section benchmark mise run benchmark:check
section nav-pilot mise run nav-pilot:check
wait_sections

# A timing test: it runs alone, after everything else has finished, as in CI.
section nav-pilot-budget mise run nav-pilot:budget
wait_sections

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "❌ Checks failed for: ${failed[*]}"
  exit 1
fi
echo "✅ All checks passed"
