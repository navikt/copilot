#!/usr/bin/env bash
#
# Verify that .github/skills/ and skills/ are identical.
#
# .github/skills/ is the canonical source (used by nav-pilot consumers).
# skills/ is the distribution copy (used by gh skill install discovery).
#
# Usage:
#   ./scripts/sync-skills-dirs.sh          # check only — exit 1 on drift
#   ./scripts/sync-skills-dirs.sh --fix    # mirror canonical → distribution, then check
#   ./scripts/sync-skills-dirs.sh -q       # quiet mode — only show drift / errors
#
set -euo pipefail

CANONICAL=".github/skills"
DISTRIBUTION="skills"

QUIET=false
FIX=false

for arg in "$@"; do
  case "$arg" in
    --fix) FIX=true ;;
    -q)    QUIET=true ;;
    *)     echo "Unknown argument: $arg"; exit 2 ;;
  esac
done

info() { $QUIET || echo "$1"; }

if [[ ! -d "$CANONICAL" ]]; then
  echo "❌ Canonical directory $CANONICAL does not exist"
  exit 1
fi

if $FIX; then
  info "🔄 Mirroring $CANONICAL → $DISTRIBUTION"
  rm -rf "$DISTRIBUTION"
  mkdir -p "$DISTRIBUTION"
  cp -R "$CANONICAL"/. "$DISTRIBUTION"/
  info "✅ Mirror complete"
fi

if [[ ! -d "$DISTRIBUTION" ]]; then
  echo "❌ Distribution directory $DISTRIBUTION does not exist"
  echo "   Run with --fix to create it from $CANONICAL"
  exit 1
fi

drift=$(diff -rq "$CANONICAL" "$DISTRIBUTION" 2>&1 || true)

if [[ -n "$drift" ]]; then
  echo "❌ Skills directories are out of sync:"
  echo ""
  echo "$drift"
  echo ""
  echo "Run: ./scripts/sync-skills-dirs.sh --fix"
  exit 1
fi

info "✅ Skills directories are in sync"
