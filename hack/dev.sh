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
    echo -e "\n${YELLOW}Stopping dev servers...${RESET}"
    kill "$API_PID" "$WEB_PID" 2>/dev/null || true
    wait "$API_PID" "$WEB_PID" 2>/dev/null || true
}
trap cleanup SIGINT SIGTERM EXIT

echo -e "${BOLD}🚀 Starting local development environment${RESET}"
echo -e "   ${CYAN}copilot-api${RESET}  →  http://localhost:8080"
echo -e "   ${GREEN}my-copilot${RESET}   →  http://localhost:3000"
echo ""

cd "$ROOT_DIR"

(cd apps/copilot-api && mise run dev 2>&1 | prefix_output "api" "$CYAN") &
API_PID=$!

(cd apps/my-copilot && mise run dev 2>&1 | prefix_output "web" "$GREEN") &
WEB_PID=$!

wait "$API_PID" "$WEB_PID"
