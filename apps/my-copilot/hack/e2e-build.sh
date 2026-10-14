#!/usr/bin/env bash
# Bygger appen to ganger, til .e2e/a og .e2e/b. To bygg gir to build-id-er og
# to sett med server action-id-er, slik to deployer gjør i prod. e2e/survey.spec.ts
# åpner en side på A og sender skjemaet etter at B har tatt over porten (#1804).
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .e2e
for v in a b; do
  # .next/cache/.rscinfo holds the key that salts the action ids. A kept cache
  # gives both builds the same ids; the Docker build in CI starts without it.
  rm -rf .next
  pnpm build
  rm -rf ".e2e/$v"
  cp -R .next/standalone ".e2e/$v"
  cp -R .next/static ".e2e/$v/.next/static"
  cp -R public ".e2e/$v/public"
done
[ "$(cat .e2e/a/.next/BUILD_ID)" != "$(cat .e2e/b/.next/BUILD_ID)" ] || { echo "e2e: like build-id-er" >&2; exit 1; }
