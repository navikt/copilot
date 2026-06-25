#!/usr/bin/env bash
#
# Compute the next skills semver from conventional commits.
#
# Reads the latest v*.*.* tag, finds commits since that tag touching
# .github/skills/, and determines the bump level from commit messages.
#
# Usage:
#   ./scripts/next-skills-version.sh          # outputs e.g. "0.2.0"
#   ./scripts/next-skills-version.sh --check  # exits 1 if no new commits
#
set -euo pipefail

CHECK=false
for arg in "$@"; do
  case "$arg" in
    --check) CHECK=true ;;
    *)       echo "Unknown argument: $arg"; exit 2 ;;
  esac
done

# Find latest skills version tag reachable from HEAD (ignore stray tags on other branches)
LATEST=$(git tag --merged HEAD -l "v*.*.*" --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)

if [[ -z "$LATEST" ]]; then
  RANGE="HEAD"
  CURRENT="0.0.0"
else
  RANGE="${LATEST}..HEAD"
  CURRENT="${LATEST#v}"
fi

# Get commit hashes touching canonical skills dir
mapfile -t HASHES < <(git log "$RANGE" --format="%H" -- skills/)

if [[ ${#HASHES[@]} -eq 0 ]]; then
  if $CHECK; then
    echo "No skills commits since ${LATEST:-beginning}" >&2
    exit 1
  fi
  IFS='.' read -r major minor patch <<< "$CURRENT"
  echo "${major}.${minor}.$((patch + 1))"
  exit 0
fi

MAJOR=false
MINOR=false

for hash in "${HASHES[@]}"; do
  # First line is the conventional commit header
  SUBJECT=$(git log -1 --format="%s" "$hash")
  BODY=$(git log -1 --format="%b" "$hash")

  # Breaking change via ! in subject: feat!:, fix(scope)!:, etc.
  if [[ "$SUBJECT" =~ ^[a-z]+'('.*')'!: ]] || [[ "$SUBJECT" =~ ^[a-z]+!: ]]; then
    MAJOR=true
    continue
  fi

  # Breaking change in commit body footer
  while IFS= read -r line; do
    if [[ "$line" =~ ^BREAKING[[:space:]]CHANGE: ]] || [[ "$line" =~ ^BREAKING-CHANGE: ]]; then
      MAJOR=true
      break
    fi
  done <<< "$BODY"

  # Feature (minor bump) — only from subject line
  if [[ "$SUBJECT" =~ ^feat'('.*')': ]] || [[ "$SUBJECT" =~ ^feat: ]]; then
    MINOR=true
  fi
done

IFS='.' read -r major minor patch <<< "$CURRENT"

if $MAJOR; then
  echo "$((major + 1)).0.0"
elif $MINOR; then
  echo "${major}.$((minor + 1)).0"
else
  echo "${major}.${minor}.$((patch + 1))"
fi
