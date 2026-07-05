#!/usr/bin/env bash
# hack/dev.sh — Start copilot-api and my-copilot concurrently for local development.
#
# Usage:
#   mise dev                     # from repo root
#   bash hack/dev.sh             # directly
#
# Ports:
#   copilot-api  →  http://localhost:8080
#   my-copilot   →  http://localhost:3000
#
# Requires: mise, fnox (for secret injection in my-copilot and copilot-api)
# Hot reload: air (Go) for copilot-api, Next.js HMR for my-copilot
set -euo pipefail

BOLD='\033[1m'
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RESET='\033[0m'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

prefix_output() {
    local label="$1"
    local color="$2"
    while IFS= read -r line; do
        printf "${color}[${label}]${RESET} %s\n" "$line"
    done
}

cleanup() {
    if [[ "${CLEANUP_DONE:-0}" -eq 1 ]]; then
        return
    fi
    CLEANUP_DONE=1
    echo -e "\n${YELLOW}Stopping dev servers...${RESET}"
    kill_process_tree "${API_PID:-}" TERM
    kill_process_tree "${WEB_PID:-}" TERM
    sleep 1
    kill_process_tree "${API_PID:-}" KILL
    kill_process_tree "${WEB_PID:-}" KILL

    local pids=()
    [[ -n "${API_PID:-}" ]] && pids+=("${API_PID}")
    [[ -n "${WEB_PID:-}" ]] && pids+=("${WEB_PID}")
    if [[ "${#pids[@]}" -gt 0 ]]; then
        wait "${pids[@]}" 2>/dev/null || true
    fi
}
trap cleanup SIGINT SIGTERM EXIT

kill_process_tree() {
    local root_pid="$1"
    local signal="$2"

    [[ -z "${root_pid}" ]] && return 0
    if ! kill -0 "${root_pid}" 2>/dev/null; then
        return 0
    fi

    local child
    local children=()
    while IFS= read -r child; do
        [[ -n "${child}" ]] && children+=("${child}")
    done < <(ps -axo pid=,ppid= | awk -v p="${root_pid}" '$2==p {print $1}')

    for child in "${children[@]}"; do
        kill_process_tree "${child}" "${signal}"
    done

    kill "-${signal}" "${root_pid}" 2>/dev/null || true
}

echo -e "${BOLD}🚀 Starting local development environment${RESET}"
echo -e "   ${CYAN}copilot-api${RESET}  →  http://localhost:8080"
echo -e "   ${GREEN}my-copilot${RESET}   →  http://localhost:3000"
echo ""

cd "$ROOT_DIR"

# Export secrets from keychain into this shell's environment.
# Child processes (air, pnpm) inherit these vars naturally.
#
# `fnox export` can fail or silently under-populate the environment (e.g.
# Keychain locked, or a stale/incomplete secret) — previously this was
# swallowed (`2>/dev/null || true`), causing confusing 503s with no
# indication of the real cause. We now hard-fail with a clear message:
#   1. fnox export itself must succeed (exit 0).
#   2. Every secret key *declared* in that app's fnox.toml must resolve to
#      a non-empty value after export. Keys are read from fnox.toml itself
#      (not hardcoded here) so this stays correct as secrets are added or
#      removed per app.
required_secret_keys() {
    # Prints one secret key name per line from the [secrets] section of a
    # fnox.toml. Apps that declare zero secrets (e.g. my-copilot) print
    # nothing, so the check below is a no-op for them.
    local fnox_toml="$1"
    [[ -f "$fnox_toml" ]] || return 0
    awk '
        /^\[secrets\]/ { in_secrets = 1; next }
        /^\[/          { in_secrets = 0 }
        in_secrets && /^[A-Za-z_][A-Za-z0-9_]*[ \t]*=/ {
            sub(/[ \t]*=.*/, "")
            print
        }
    ' "$fnox_toml"
}

export_secrets() {
    local dir="$1" out
    if ! out="$(cd "$dir" && fnox export -f shell 2>&1)"; then
        echo -e "${YELLOW}✗ fnox export failed for ${dir}:${RESET}"
        echo "$out" | while IFS= read -r line; do echo "  $line"; done
        echo -e "${YELLOW}Fix the Keychain/fnox setup above, then re-run 'mise dev'.${RESET}"
        exit 1
    fi
    eval "$out"

    local missing=()
    local key
    while IFS= read -r key; do
        [[ -z "$key" ]] && continue
        if [[ -z "${!key:-}" ]]; then
            missing+=("$key")
        fi
    done < <(required_secret_keys "$dir/fnox.toml")

    if [[ "${#missing[@]}" -gt 0 ]]; then
        echo -e "${YELLOW}✗ ${dir}: fnox export succeeded but these declared secrets are empty:${RESET}"
        for key in "${missing[@]}"; do
            echo "  - $key"
        done
        echo -e "${YELLOW}Set them with: (cd ${dir} && fnox set <KEY> < path/to/value)${RESET}"
        exit 1
    fi
}
export_secrets apps/copilot-api
export_secrets apps/my-copilot

# Single source of truth for the local dev user. Inherited by BOTH child processes:
# - my-copilot: mock logged-in user (email + derived name)
# - copilot-api: mock token context; drives SAML username lookup (e.g. budget endpoint
#   resolves this email -> GitHub username). Kept here (not in mise [env]) so it never
#   pollutes `mise check`/CI test runs, which expect the default dev@nav.no.
export DEV_USER_EMAIL="${DEV_USER_EMAIL:-hans.kristian.flaatten@nav.no}"

# Video defaults for local development:
# - Prefer the injected dev bucket variables if they exist.
# - Buckets are expected to be publicly readable for video asset delivery.
if [[ -n "${VIDEO_BUCKET_PUBLIC_DEV:-}" || -n "${VIDEO_BUCKET_PUBLIC:-}" ]]; then
    export VIDEO_BUCKET_PUBLIC="${VIDEO_BUCKET_PUBLIC:-$VIDEO_BUCKET_PUBLIC_DEV}"
    if [[ -n "${VIDEO_BUCKET_PUBLIC:-}" ]]; then
        export VIDEO_PUBLIC_BASE_URL="${VIDEO_PUBLIC_BASE_URL:-https://storage.googleapis.com/${VIDEO_BUCKET_PUBLIC}}"
        export VIDEO_MANIFEST_URL="${VIDEO_MANIFEST_URL:-gs://${VIDEO_BUCKET_PUBLIC}/video_manifest.json}"
    fi
fi
export VIDEO_MANIFEST_PATH="${VIDEO_MANIFEST_PATH:-video_manifest.local-fallback.json}"
export VIDEO_FEED_CACHE_SECONDS="${VIDEO_FEED_CACHE_SECONDS:-60}"

(cd apps/copilot-api && LOG_LEVEL=DEBUG GCP_TEAM_PROJECT_ID=copilot-dev-e17a LOGGED_ENDPOINTS="/api/v1/,/health,/ready" mise exec -- air 2>&1 | prefix_output "api" "$CYAN") &
API_PID=$!

(cd apps/my-copilot && COPILOT_API_URL=http://localhost:8080 mise exec -- pnpm dev 2>&1 | prefix_output "web" "$GREEN") &
WEB_PID=$!

wait "$API_PID" "$WEB_PID"
