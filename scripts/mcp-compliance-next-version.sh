#!/usr/bin/env bash
# Prints the next mcp-compliance version (vX.Y.Z) for commit $1, from the
# Conventional Commit subjects since the last cli/mcp-compliance/v* tag that
# touch cli/mcp-compliance. Below v1 a breaking change or feat bumps the minor,
# from v1 a breaking change bumps the major and feat the minor; anything else
# bumps the patch. No tag yet gives v0.1.0.
set -euo pipefail
sha=${1:?usage: $0 <commit>}
prefix=cli/mcp-compliance/
last=$(git tag --merged "$sha" -l "${prefix}v*" | sed "s|^${prefix}||" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 || true)
if [ -z "$last" ]; then echo v0.1.0; exit 0; fi
IFS=. read -r major minor patch <<<"${last#v}"
log=$(git log --format='%s%n%b' "${prefix}${last}..${sha}" -- cli/mcp-compliance)
if [ -z "$log" ]; then echo "no mcp-compliance changes since $last" >&2; exit 1; fi
if grep -qE '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:' <<<"$log"; then
  if [ "$major" = 0 ]; then minor=$((minor + 1)); else
    # The module path has no /v2 suffix, so v2+ would break `go install ...@latest`.
    echo "breaking change from v${major}.x needs v$((major + 1)), but the module path has no /v$((major + 1)) suffix; change the module path first" >&2
    exit 1
  fi
  patch=0
elif grep -qE '^feat(\([^)]*\))?:' <<<"$log"; then
  minor=$((minor + 1)); patch=0
else
  patch=$((patch + 1))
fi
echo "v${major}.${minor}.${patch}"
