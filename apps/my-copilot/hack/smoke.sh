#!/usr/bin/env bash
# Røyktest: start den bygde standalone-serveren og hent hver rute i src/app.
#
# Hvorfor: `next build --experimental-build-mode compile` hopper over
# prerendering, og `tsc` ser ikke render-feil. En side som kaster under render
# svarer likevel 200 — Next sender skjelettet i Suspense-fallbacken og merker
# grensen med `data-dgst`, digesten til feilen serveren kastet, og klienten
# bytter skjelettet ut med error.tsx («Noe gikk galt»). Statuskoden alene
# fanger altså ingenting; vi må se etter digesten i kroppen. Next bruker samme
# mekanisme til vanlig kontrollflyt — `redirect()` og `notFound()` gir en
# `NEXT_*`-digest — så det er bare digestene uten det prefikset som er feil.
set -euo pipefail

cd "$(dirname "$0")/.."

[ -f .next/standalone/server.js ] || { echo "smoke: mangler .next/standalone — kjør build først" >&2; exit 1; }

rm -rf .next/standalone/.next/static .next/standalone/public
cp -r .next/static .next/standalone/.next/static
cp -r public .next/standalone/public

PORT="${SMOKE_PORT:-3999}"
NODE_ENV=production PORT="$PORT" HOSTNAME=127.0.0.1 node .next/standalone/server.js >/tmp/smoke-server.log 2>&1 &
server=$!
trap 'kill "$server" 2>/dev/null || true' EXIT

for _ in $(seq 1 60); do
  curl -sf -o /dev/null "http://127.0.0.1:$PORT/" && break
  sleep 1
done

failed=0
while read -r page; do
  route=${page#src/app}
  route=${route%/page.tsx}
  route=$(printf '%s' "$route" | sed 's|/([^/)]*)||g')
  [ -z "$route" ] && route=/
  case "$route" in *"["*) continue;; esac

  body=$(mktemp)
  status=$(curl -s -o "$body" -w '%{http_code}' "http://127.0.0.1:$PORT$route")
  if [ "$status" -ge 400 ]; then
    echo "FAIL $route: HTTP $status"
    failed=1
  elif [ "$status" -lt 300 ] && grep -oE 'data-dgst="[^"]*"' "$body" | grep -qv 'NEXT_'; then
    echo "FAIL $route: siden kastet under render (React-feilgrense i HTML-en)"
    failed=1
  else
    echo "ok   $route ($status)"
  fi
  rm -f "$body"
done < <(find src/app -name page.tsx | sort)

if [ "$failed" -ne 0 ]; then
  echo "--- serverlogg ---"
  tail -40 /tmp/smoke-server.log
  exit 1
fi
