#!/bin/sh
# Fail when public test data (testdata/, fixtures, *golden* files) contains a
# UUID or a host under .nav.no, .intern.nav.no or .nais.io that is not listed in
# .github/testdata-allowlist.txt. The repo is public: dev-seed client IDs once
# leaked into goldens this way (#3124).
#
#   scripts/testdata-scan.sh           scan tracked files
#   scripts/testdata-scan.sh --seed    print every current hit, for the allowlist
#
# POSIX grep -E only (no -P), so it runs the same on macOS and Linux.
set -eu
cd "$(git rev-parse --show-toplevel)"

allowlist=.github/testdata-allowlist.txt
paths='(^|/)(testdata|fixtures?|__fixtures__)/|golden'
values='[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}|[A-Za-z0-9.-]+\.(nav\.no|nais\.io)'

# path:line:value for every hit; -I skips binary files.
hits=$(git ls-files | grep -E "$paths" | tr '\n' '\0' |
  xargs -0 grep -IonE "$values" /dev/null || true)

if [ "${1:-}" = "--seed" ]; then
  printf '%s\n' "$hits" | awk -F: 'NF>=3 {print $NF}' | sort -u
  exit 0
fi

new=$(printf '%s\n' "$hits" |
  awk -F: -v list="$allowlist" '
    BEGIN { while ((getline l < list) > 0) if (l !~ /^#/ && l != "") ok[l] = 1 }
    NF >= 3 && !($NF in ok)')

if [ -n "$new" ]; then
  printf '%s\n' "$new" | sed 's/^/::error::test data value not in allowlist: /'
  echo "Replace real IDs and hostnames with fake ones (00000000-0000-4000-8000-000000000001, example.com)." >&2
  echo "If a value is public and intended, add it to $allowlist." >&2
  exit 1
fi
echo "testdata-scan: no unlisted UUIDs or Nav hostnames in test data"
